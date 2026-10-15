package pagerevm

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/containerman17/pagerdb"
)

// Table stores fixed-size values after raw 32-byte keys. An all-zero value
// means absent. The region starts with a 16-byte header: count and value size.
// All integers, including the hash prefix, use little endian.
type Table struct {
	DB        *pagerdb.DB
	Region    pagerdb.Region
	ValueSize int
}

func NewTable(db *pagerdb.DB, size int) (*Table, error) {
	if size < 1 {
		return nil, fmt.Errorf("invalid value size")
	}
	r, e := db.Alloc(16 + 16*(32+size))
	if e != nil {
		return nil, e
	}
	t := &Table{db, r, size}
	var h [16]byte
	binary.LittleEndian.PutUint64(h[8:], uint64(size))
	db.Write(r, 0, h[:])
	return t, nil
}
func (t *Table) data() []byte {
	b, e := t.DB.Map(t.Region)
	if e != nil {
		panic(e)
	}
	return b
}
func (t *Table) Len() int      { return int(binary.LittleEndian.Uint64(t.data())) }
func (t *Table) Capacity() int { return (len(t.data()) - 16) / (32 + t.ValueSize) }
func home(key []byte, mask int) int {
	h := sha256.Sum256(key)
	return int(binary.LittleEndian.Uint64(h[:8]) & uint64(mask))
}
func zero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}
func (t *Table) entry(b []byte, i int) []byte { s := 32 + t.ValueSize; return b[16+i*s : 16+(i+1)*s] }
func (t *Table) find(k [32]byte) (int, bool, int) {
	b := t.data()
	mask := t.Capacity() - 1
	i := home(k[:], mask)
	for d := 0; d <= mask; d++ {
		e := t.entry(b, i)
		if zero(e[32:]) {
			return i, false, d
		}
		if bytes.Equal(e[:32], k[:]) {
			return i, true, d
		}
		i = (i + 1) & mask
	}
	panic("full table")
}
func (t *Table) Get(k [32]byte) []byte {
	i, ok, _ := t.find(k)
	if !ok {
		return make([]byte, t.ValueSize)
	}
	return bytes.Clone(t.entry(t.data(), i)[32:])
}
func (t *Table) count(n int) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(n))
	t.DB.Write(t.Region, 0, b[:])
}
func (t *Table) write(i int, k [32]byte, v []byte) {
	b := make([]byte, 32+t.ValueSize)
	copy(b, k[:])
	copy(b[32:], v)
	t.DB.Write(t.Region, 16+i*len(b), b)
}
func (t *Table) Set(k [32]byte, v []byte) error {
	if len(v) != t.ValueSize {
		return fmt.Errorf("value length %d, want %d", len(v), t.ValueSize)
	}
	i, found, d := t.find(k)
	if zero(v) {
		if found {
			t.remove(i)
		}
		return nil
	}
	if found {
		t.write(i, k, v)
		return nil
	}
	for (t.Len()+1)*4 > t.Capacity()*3 || d > 64 {
		if e := t.grow(); e != nil {
			return e
		}
		i, _, d = t.find(k)
	}
	t.write(i, k, v)
	t.count(t.Len() + 1)
	return nil
}
func (t *Table) remove(hole int) {
	b := t.data()
	mask := t.Capacity() - 1
	for i := (hole + 1) & mask; !zero(t.entry(b, i)[32:]); i = (i + 1) & mask {
		e := t.entry(b, i)
		h := home(e[:32], mask)
		if (i-h)&mask >= (i-hole)&mask {
			t.DB.Write(t.Region, 16+hole*(32+t.ValueSize), bytes.Clone(e))
			hole = i
		}
	}
	t.DB.Write(t.Region, 16+hole*(32+t.ValueSize), make([]byte, 32+t.ValueSize))
	t.count(t.Len() - 1)
}
func (t *Table) grow() error {
	old := bytes.Clone(t.data())
	n := t.Capacity() * 2
	// Rehash in old bucket order. Grow preserves the stable region handle.
	if e := t.DB.Grow(t.Region, 16+n*(32+t.ValueSize)); e != nil {
		return e
	}
	t.DB.Write(t.Region, 16, make([]byte, n*(32+t.ValueSize)))
	t.count(0)
	for off := 16; off < len(old); off += 32 + t.ValueSize {
		e := old[off : off+32+t.ValueSize]
		if !zero(e[32:]) {
			var k [32]byte
			copy(k[:], e[:32])
			if err := t.Set(k, e[32:]); err != nil {
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
	for off := 16; off < len(b); off += 32 + t.ValueSize {
		e := b[off : off+32+t.ValueSize]
		if !zero(e[32:]) {
			if err := fn([32]byte(e[:32]), e[32:]); err != nil {
				return err
			}
		}
	}
	return nil
}
