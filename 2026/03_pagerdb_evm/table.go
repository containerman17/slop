package pagerevm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"

	"github.com/containerman17/pagerdb"
)

// tableHeader holds count u64 and value size u64, padded to 64 bytes so that
// 64-byte slot entries and their values stay aligned to PagerDB's 32-byte redo
// chunks and never straddle a leaf.
const tableHeader = 64

// tableFormat identifies the fastrange layout; zero was the low-bit layout.
const tableFormat = 1

var le = binary.LittleEndian

// Table stores fixed-size values after raw 32-byte keys. An all-zero value
// means absent. All integers use little endian.
type Table struct {
	DB        *pagerdb.DB
	Region    pagerdb.Region
	ValueSize int
}

func NewTable(db *pagerdb.DB, size int) (*Table, error) { return NewTableCap(db, size, 16) }

// NewTableCap creates a table with any starting capacity of at least 16.
func NewTableCap(db *pagerdb.DB, size, capacity int) (*Table, error) {
	if size < 1 || capacity < 16 {
		return nil, fmt.Errorf("invalid value size or capacity")
	}
	r, e := db.Alloc(tableHeader + capacity*(32+size))
	if e != nil {
		return nil, e
	}
	var h [24]byte
	le.PutUint64(h[8:], uint64(size))
	le.PutUint64(h[16:], tableFormat)
	db.Write(r, 0, h[:])
	return &Table{db, r, size}, nil
}
func (t *Table) data() []byte {
	b, e := t.DB.Map(t.Region)
	if e != nil {
		panic(e)
	}
	return b
}
func (t *Table) Len() int      { return int(le.Uint64(t.data())) }
func (t *Table) Capacity() int { return (len(t.data()) - tableHeader) / (32 + t.ValueSize) }

// Home is the 64-bit bucket hash of a key; a table maps it with fastrange. One
// multiply-xorshift step per key word: Keccak-derived keys are already uniform,
// and the shifts spread small big-endian keys such as 0, 1, 2 over all buckets.
// It is not collision resistant; neither were bucket indexes cut from SHA-256.
func Home(k *[32]byte) uint64 {
	h := uint64(0x9E3779B97F4A7C15)
	for i := 0; i < 32; i += 8 {
		h = (h ^ le.Uint64(k[i:])) * 0xBF58476D1CE4E5B9
		h ^= h >> 31
	}
	h *= 0x94D049BB133111EB
	return h ^ h>>29
}
func home(k *[32]byte, capacity int) int {
	hi, _ := bits.Mul64(Home(k), uint64(capacity))
	return int(hi)
}

func next(i, capacity int) int {
	i++
	if i == capacity {
		i = 0
	}
	return i
}

func distance(from, to, capacity int) int {
	d := to - from
	if d < 0 {
		d += capacity
	}
	return d
}

func zero(b []byte) bool {
	var x uint64
	for ; len(b) >= 8; b = b[8:] {
		x |= le.Uint64(b)
	}
	for _, c := range b {
		x |= uint64(c)
	}
	return x == 0
}
func (t *Table) find(b []byte, k *[32]byte) (int, bool, int) {
	s := 32 + t.ValueSize
	capacity := (len(b) - tableHeader) / s
	k0, k1, k2, k3 := le.Uint64(k[0:]), le.Uint64(k[8:]), le.Uint64(k[16:]), le.Uint64(k[24:])
	i := home(k, capacity)
	for d := 0; d < capacity; d++ {
		e := b[tableHeader+i*s : tableHeader+(i+1)*s]
		if zero(e[32:]) {
			return i, false, d
		}
		if le.Uint64(e[0:]) == k0 && le.Uint64(e[8:]) == k1 && le.Uint64(e[16:]) == k2 && le.Uint64(e[24:]) == k3 {
			return i, true, d
		}
		i = next(i, capacity)
	}
	panic("full table")
}

// Read copies the value of k into dst, or zeros dst when k is absent.
func (t *Table) Read(k [32]byte, dst []byte) {
	b := t.data()
	if i, ok, _ := t.find(b, &k); ok {
		off := tableHeader + i*(32+t.ValueSize) + 32
		copy(dst, b[off:off+t.ValueSize])
		return
	}
	clear(dst[:t.ValueSize])
}
func (t *Table) Get(k [32]byte) []byte {
	v := make([]byte, t.ValueSize)
	t.Read(k, v)
	return v
}
func (t *Table) count(b []byte, n int) {
	t.DB.Touch(t.Region, 0, 8)
	le.PutUint64(b, uint64(n))
}
func (t *Table) Set(k [32]byte, v []byte) error {
	if len(v) != t.ValueSize {
		return fmt.Errorf("value length %d, want %d", len(v), t.ValueSize)
	}
	b := t.data()
	i, found, d := t.find(b, &k)
	s := 32 + t.ValueSize
	if zero(v) {
		if found {
			t.remove(b, i)
		}
		return nil
	}
	if found {
		// Only the value changes; an unchanged value dirties nothing.
		off := tableHeader + i*s + 32
		if !bytes.Equal(b[off:off+t.ValueSize], v) {
			t.DB.Touch(t.Region, off, t.ValueSize)
			copy(b[off:], v)
		}
		return nil
	}
	n := t.Len()
	for (n+1)*4 > t.Capacity()*3 || d > 64 {
		if e := t.grow(); e != nil {
			return e
		}
		b = t.data()
		i, _, d = t.find(b, &k)
	}
	off := tableHeader + i*s
	t.DB.Touch(t.Region, off, s)
	copy(b[off:], k[:])
	copy(b[off+32:], v)
	t.count(b, n+1)
	return nil
}
func (t *Table) remove(b []byte, hole int) {
	s := 32 + t.ValueSize
	capacity := (len(b) - tableHeader) / s
	entry := func(i int) []byte { return b[tableHeader+i*s : tableHeader+(i+1)*s] }
	for i := next(hole, capacity); !zero(entry(i)[32:]); i = next(i, capacity) {
		e := entry(i)
		h := home((*[32]byte)(e[:32]), capacity)
		if distance(h, i, capacity) >= distance(hole, i, capacity) {
			t.DB.Touch(t.Region, tableHeader+hole*s, s)
			copy(entry(hole), e)
			hole = i
		}
	}
	t.DB.Touch(t.Region, tableHeader+hole*s, s)
	clear(entry(hole))
	t.count(b, t.Len()-1)
}
func (t *Table) grow() error {
	old := bytes.Clone(t.data())
	n := t.Capacity() + (t.Capacity()+1)/2
	// Rehash in old bucket order. Grow preserves the stable region handle.
	if e := t.DB.Grow(t.Region, tableHeader+n*(32+t.ValueSize)); e != nil {
		return e
	}
	t.DB.Write(t.Region, tableHeader, make([]byte, n*(32+t.ValueSize)))
	t.count(t.data(), 0)
	for off := tableHeader; off < len(old); off += 32 + t.ValueSize {
		e := old[off : off+32+t.ValueSize]
		if !zero(e[32:]) {
			if err := t.Set([32]byte(e[:32]), e[32:]); err != nil {
				return err
			}
		}
	}
	return nil
}

// Each visits non-empty entries. The callback must not mutate the table or
// retain the value slice, which refers directly to the mapped region.
func (t *Table) Each(fn func([32]byte, []byte) error) error {
	b := t.data()
	for off := tableHeader; off < len(b); off += 32 + t.ValueSize {
		e := b[off : off+32+t.ValueSize]
		if !zero(e[32:]) {
			if err := fn([32]byte(e[:32]), e[32:]); err != nil {
				return err
			}
		}
	}
	return nil
}
