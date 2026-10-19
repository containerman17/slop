package pagerevm

import (
	"bytes"
	"encoding/binary"
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
	for n := uint64(0); n < 13; n++ {
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
	p.Write(db.accounts.Region, 16, make([]byte, 8))
	if _, err := NewDatabase(p); err == nil {
		t.Fatal("accepted old bucket layout")
	}
}
