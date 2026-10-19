package pagerevm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"

	"github.com/containerman17/pagerdb"
)

func pages(t *testing.T) *pagerdb.DB {
	t.Helper()
	p, e := pagerdb.Open(t.TempDir(), pagerdb.Options{MaxLeaves: 1 << 16, Workers: 2, RetainVersions: 2})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := p.Close(); e != nil {
			t.Error(e)
		}
	})
	return p
}
func TestTableRandom(t *testing.T) {
	for _, size := range []int{32, 80} {
		p := pages(t)
		tab, e := NewTableCap(p, size, 17)
		if e != nil {
			t.Fatal(e)
		}
		rng := rand.New(rand.NewSource(12))
		ref := make(map[[32]byte][]byte)
		for n := 0; n < 20000; n++ {
			var k [32]byte
			binary.LittleEndian.PutUint64(k[:], uint64(rng.Intn(1000)))
			v := make([]byte, size)
			if rng.Intn(3) != 0 {
				rng.Read(v)
				ref[k] = bytes.Clone(v)
			} else {
				delete(ref, k)
			}
			if e = tab.Set(k, v); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(tab.Get(k), v) {
				t.Fatalf("value step %d", n)
			}
			if tab.Len() != len(ref) {
				t.Fatalf("count step %d", n)
			}
			if n%100 == 0 {
				for k, v := range ref {
					if !bytes.Equal(tab.Get(k), v) {
						t.Fatalf("lost entry step %d", n)
					}
				}
			}
		}
		for k := range ref {
			if e = tab.Set(k, make([]byte, size)); e != nil {
				t.Fatal(e)
			}
		}
		if tab.Len() != 0 {
			t.Fatal("not empty")
		}
	}
}
func TestProbeCapAndUndo(t *testing.T) {
	p := pages(t)
	tab, e := NewTableCap(p, 32, 129)
	if e != nil {
		t.Fatal(e)
	}
	mark := p.Mark()
	var keys [][32]byte
	for n := uint64(0); len(keys) < 66; n++ {
		var k [32]byte
		binary.LittleEndian.PutUint64(k[:], n)
		if home(&k, 129) == 128 {
			keys = append(keys, k)
		}
	}
	v := make([]byte, 32)
	v[0] = 1
	for _, k := range keys {
		if e = tab.Set(k, v); e != nil {
			t.Fatal(e)
		}
	}
	if tab.Capacity() <= 129 {
		t.Fatal("probe cap did not grow")
	}
	for _, k := range keys {
		_, ok, d := tab.find(tab.data(), &k)
		if !ok || d > 64 {
			t.Fatal("probe distance", d)
		}
	}
	for i, k := range keys {
		if i%2 == 0 {
			if e = tab.Set(k, make([]byte, 32)); e != nil {
				t.Fatal(e)
			}
		}
	}
	for i, k := range keys {
		if zero(tab.Get(k)) != (i%2 == 0) {
			t.Fatal("collision deletion lost entry")
		}
	}
	p.Undo(mark)
	if tab.Capacity() != 129 || tab.Len() != 0 {
		t.Fatal("undo failed")
	}
}

// Small big-endian keys (slot 0, 1, 2, ...) and random keys must spread without
// probe-cap growth, and lookups and value updates must not allocate.
func TestHomeSpreadAndAllocs(t *testing.T) {
	for _, random := range []bool{false, true} {
		p := pages(t)
		tab, e := NewTableCap(p, 32, 32769)
		if e != nil {
			t.Fatal(e)
		}
		rng := rand.New(rand.NewSource(3))
		v := make([]byte, 32)
		v[31] = 1
		const n = 16000 // about 49% of 32769: a uniform hash stays far below the probe cap
		keys := make([][32]byte, n)
		for i := range keys {
			if random {
				rng.Read(keys[i][:])
			} else {
				binary.BigEndian.PutUint64(keys[i][24:], uint64(i))
			}
			if e = tab.Set(keys[i], v); e != nil {
				t.Fatal(e)
			}
		}
		if tab.Capacity() != 32769 {
			t.Fatalf("random=%v: capacity %d, want %d", random, tab.Capacity(), 32769)
		}
		var dst [32]byte
		allocs := testing.AllocsPerRun(1000, func() {
			k := keys[rng.Intn(n)]
			tab.Read(k, dst[:])
			dst[0]++
			if e := tab.Set(k, dst[:]); e != nil {
				t.Fatal(e)
			}
		})
		if allocs != 0 {
			t.Fatalf("random=%v: %v allocations per Read+Set", random, allocs)
		}
	}
}

func TestTableWrapDelete(t *testing.T) {
	p := pages(t)
	tab, err := NewTableCap(p, 32, 17)
	if err != nil {
		t.Fatal(err)
	}
	var keys [][32]byte
	// The cluster crosses bucket zero and contains entries with different homes.
	for _, bucket := range []int{15, 16, 15, 0, 16, 1, 15} {
		for n := uint64(len(keys) * 10000); ; n++ {
			var k [32]byte
			binary.LittleEndian.PutUint64(k[:], n)
			if home(&k, 17) == bucket {
				keys = append(keys, k)
				break
			}
		}
	}
	var value [32]byte
	value[0] = 1
	for _, k := range keys {
		if err := tab.Set(k, value[:]); err != nil {
			t.Fatal(err)
		}
	}
	for i, k := range keys {
		if err := tab.Set(k, make([]byte, 32)); err != nil {
			t.Fatal(err)
		}
		for j, remaining := range keys {
			if zero(tab.Get(remaining)) != (j <= i) {
				t.Fatalf("delete %d lost key %d", i, j)
			}
		}
	}
}

func TestTableGrowth(t *testing.T) {
	p := pages(t)
	tab, err := NewTableCap(p, 32, 17)
	if err != nil {
		t.Fatal(err)
	}
	for n := uint64(0); n < uint64(17*maxLoad/100+1); n++ {
		var k, v [32]byte
		binary.LittleEndian.PutUint64(k[:], n)
		v[0] = 1
		if err := tab.Set(k, v[:]); err != nil {
			t.Fatal(err)
		}
	}
	if tab.Capacity() != 26 {
		t.Fatalf("capacity %d, want 26", tab.Capacity())
	}
}

func TestRejectOldTableLayout(t *testing.T) {
	p := pages(t)
	db, err := NewDatabase(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []uint64{0, 1} {
		var b [8]byte
		le.PutUint64(b[:], format)
		p.Write(db.accounts.Region, 16, b[:])
		if _, err := NewDatabase(p); err == nil {
			t.Fatalf("accepted old layout %d", format)
		}
	}
}

// Verify the lookup cutoff at every bucket on every resident key's path.
func checkTableInvariant(t *testing.T, tab *Table) {
	t.Helper()
	b := tab.data()
	s, capacity, count := 32+tab.ValueSize, tab.Capacity(), 0
	for i := 0; i < capacity; i++ {
		e := b[tableHeader+i*s : tableHeader+(i+1)*s]
		if zero(e[32:]) {
			continue
		}
		count++
		k := (*[32]byte)(e[:32])
		h := home(k, capacity)
		d := distance(h, i, capacity)
		if d > maxProbe {
			t.Fatalf("bucket %d: distance %d exceeds cap", i, d)
		}
		for j, probe := h, 0; probe <= d; j, probe = next(j, capacity), probe+1 {
			r := b[tableHeader+j*s : tableHeader+(j+1)*s]
			if zero(r[32:]) || distance(home((*[32]byte)(r[:32]), capacity), j, capacity) < probe {
				t.Fatalf("bucket %d unreachable at bucket %d, distance %d", i, j, probe)
			}
		}
		if got, ok, _ := tab.find(b, k); !ok || got != i {
			t.Fatalf("bucket %d unreachable by lookup", i)
		}
	}
	if count != tab.Len() {
		t.Fatalf("count %d, header %d", count, tab.Len())
	}
}

func TestRobinHoodDifferential(t *testing.T) {
	for _, size := range []int{32, 80, 160} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			p := pages(t)
			tab, err := NewTableCap(p, size, 17)
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewSource(721))
			ref := make(map[[32]byte][]byte)
			var keys [8192][32]byte
			for i := range keys {
				if i%2 == 0 {
					rng.Read(keys[i][:])
				} else {
					// Consecutive big-endian array slots with a common hashed prefix.
					copy(keys[i][:], []byte("array storage prefix"))
					binary.BigEndian.PutUint64(keys[i][24:], uint64(i/2))
				}
			}
			capacity, growths := tab.Capacity(), 0
			v := make([]byte, size)
			for op := 0; op < 1000000; op++ {
				k := keys[rng.Intn(len(keys))]
				if rng.Intn(4) == 0 {
					clear(v)
					delete(ref, k)
				} else {
					rng.Read(v)
					v[0] |= 1
					ref[k] = bytes.Clone(v)
				}
				if err := tab.Set(k, v); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(tab.Get(k), v) || tab.Len() != len(ref) {
					t.Fatalf("operation %d differs", op)
				}
				if tab.Capacity() != capacity {
					growths++
					capacity = tab.Capacity()
					checkTableInvariant(t, tab)
				}
				if op%8192 == 0 {
					checkTableInvariant(t, tab)
					for _, key := range keys {
						got := tab.Get(key)
						if want, ok := ref[key]; ok {
							if !bytes.Equal(got, want) {
								t.Fatalf("lost key at %d", op)
							}
						} else if !zero(got) {
							t.Fatalf("unexpected key at %d", op)
						}
					}
				}
			}
			checkTableInvariant(t, tab)
			for key, want := range ref {
				if !bytes.Equal(tab.Get(key), want) {
					t.Fatal("final value differs from map")
				}
			}
			if growths < 10 {
				t.Fatalf("only %d growths", growths)
			}
		})
	}
}

func TestHomeConsecutiveDistribution(t *testing.T) {
	for _, prefix := range []string{"", "array storage prefix"} {
		var counts [256]int
		var k [32]byte
		copy(k[:], prefix)
		const n = 1 << 20
		for i := 0; i < n; i++ {
			binary.BigEndian.PutUint64(k[24:], uint64(i))
			counts[Home(&k)>>56]++
		}
		chi := 0.0
		for _, c := range counts {
			delta := float64(c - n/256)
			chi += delta * delta / (n / 256)
		}
		t.Logf("prefix=%q chi-square=%.2f (255 degrees of freedom)", prefix, chi)
		if chi > 400 {
			t.Fatalf("consecutive keys cluster: chi-square %.2f", chi)
		}
	}
}

func TestRobinHoodDisplacementCap(t *testing.T) {
	p := pages(t)
	tab, err := NewTableCap(p, 32, 257)
	if err != nil {
		t.Fatal(err)
	}
	var keys [][32]byte
	for _, bucket := range []int{127, 128} {
		want := 65
		if bucket == 127 {
			want = 2
		}
		for n, found := uint64(0), 0; found < want; n++ {
			var k [32]byte
			binary.LittleEndian.PutUint64(k[:], n)
			if home(&k, 257) == bucket {
				keys = append(keys, k)
				found++
			}
		}
	}
	var v [32]byte
	v[0] = 1
	for i, k := range keys {
		if i == 1 {
			continue
		}
		if err := tab.Set(k, v[:]); err != nil {
			t.Fatal(err)
		}
	}
	if tab.Capacity() != 257 {
		t.Fatal("grew before displacement")
	}
	checkTableInvariant(t, tab)
	mark := p.Mark()
	if err := tab.Set(keys[1], v[:]); err != nil {
		t.Fatal(err)
	}
	if tab.Capacity() == 257 {
		t.Fatal("displaced resident exceeded cap without growth")
	}
	checkTableInvariant(t, tab)
	for _, k := range keys {
		if !bytes.Equal(tab.Get(k), v[:]) {
			t.Fatal("lost displaced key")
		}
	}
	p.Undo(mark)
	if tab.Capacity() != 257 || tab.Len() != 66 || !zero(tab.Get(keys[1])) {
		t.Fatal("undo of displacement growth failed")
	}
	checkTableInvariant(t, tab)
}

func TestRobinHoodHighLoad(t *testing.T) {
	p := pages(t)
	tab, err := NewTableCap(p, 32, 32769)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(86))
	keys := make([][32]byte, tab.Capacity()*maxLoad/100)
	var value [32]byte
	value[0] = 1
	for i := range keys {
		rng.Read(keys[i][:])
		if err := tab.Set(keys[i], value[:]); err != nil {
			t.Fatal(err)
		}
	}
	for op := 0; op < 100000; op++ {
		i := rng.Intn(len(keys))
		old := keys[i]
		if err := tab.Set(old, make([]byte, 32)); err != nil {
			t.Fatal(err)
		}
		if !zero(tab.Get(old)) {
			t.Fatal("deleted key remains")
		}
		rng.Read(keys[i][:])
		if err := tab.Set(keys[i], value[:]); err != nil {
			t.Fatal(err)
		}
		if op%4096 == 0 {
			checkTableInvariant(t, tab)
		}
	}
	if tab.Capacity() != 32769 || tab.Len() != len(keys) {
		t.Fatal("high load caused growth or count mismatch")
	}
	checkTableInvariant(t, tab)
	for _, key := range keys {
		if !bytes.Equal(tab.Get(key), value[:]) {
			t.Fatal("high-load churn lost key")
		}
	}
}
