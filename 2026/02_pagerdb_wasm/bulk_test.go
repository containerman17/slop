package pagerwasm

import (
	"bytes"
	"context"
	"encoding/binary"
	"reflect"
	"testing"
)

func bulkInput(fill, dst, src, n, status, trap uint32) []byte {
	b := make([]byte, 24)
	for i, v := range []uint32{fill, dst, src, n, status, trap} {
		binary.LittleEndian.PutUint32(b[i*4:], v)
	}
	return b
}

func TestBulkBarrier(t *testing.T) {
	for _, tc := range []struct {
		name              string
		fill, dst, src, n uint32
	}{
		{"copy-cross-line", 0, 318, 256, 64}, {"overlap-forward", 0, 264, 256, 48},
		{"overlap-backward", 0, 256, 264, 48}, {"fill-cross-line", 1, 510, 0x1234, 10},
		{"zero-copy-at-end", 0, 65536, 65536, 0}, {"zero-fill-at-end", 1, 65536, 99, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, h := testHost(t)
			p := deployCase(t, h, "bulk")
			commit(t, db)
			before := bytes.Clone(p.memory)
			want := bytes.Clone(before)
			input := bulkInput(tc.fill, tc.dst, tc.src, tc.n, 0, 0)
			copy(want, input)
			if tc.fill == 0 {
				copy(want[tc.dst:tc.dst+tc.n], want[tc.src:tc.src+tc.n])
			} else {
				for i := tc.dst; i < tc.dst+tc.n; i++ {
					want[i] = byte(tc.src)
				}
			}
			result := p.Run(context.Background(), Call{Input: input, Gas: 1 << 30})
			if result.Status != 0 || !bytes.Equal(want, p.memory) {
				t.Fatalf("status %d or wrong copy/fill bytes", result.Status)
			}
			var changed []uint32
			for i := 0; i < len(before); i += lineSize {
				if !bytes.Equal(before[i:i+lineSize], want[i:i+lineSize]) {
					changed = append(changed, uint32(i/lineSize))
				}
			}
			if !reflect.DeepEqual(result.DirtyLines, changed) {
				t.Fatalf("dirty %v changed %v", result.DirtyLines, changed)
			}
			commit(t, db)
			if err := db.Verify(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBulkRevertAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		fill, dst, src, n, status, trap uint32
		gas                             uint64
		want                            uint32
	}{
		{"copy-status", 0, 510, 256, 64, 7, 0, 1 << 30, 7},
		{"fill-status", 1, 510, 99, 64, 7, 0, 1 << 30, 7},
		{"copy-trap", 0, 510, 256, 64, 0, 1, 1 << 30, TrapStatus},
		{"fill-trap", 1, 510, 99, 64, 0, 1, 1 << 30, TrapStatus},
		{"copy-length-gas", 0, 4096, 256, 32000, 0, 0, 1000, TrapStatus},
		{"fill-length-gas", 1, 4096, 99, 32000, 0, 0, 1000, TrapStatus},
		{"copy-source-bounds", 0, 510, 65535, 2, 0, 0, 1 << 30, TrapStatus},
		{"copy-destination-bounds", 0, 65535, 256, 2, 0, 0, 1 << 30, TrapStatus},
		{"fill-destination-bounds", 1, 65535, 99, 2, 0, 0, 1 << 30, TrapStatus},
		{"copy-source-overflow", 0, 510, ^uint32(0), 2, 0, 0, 1 << 40, TrapStatus},
		{"copy-destination-overflow", 0, ^uint32(0), 256, 2, 0, 0, 1 << 40, TrapStatus},
		{"fill-length-overflow", 1, 510, 99, ^uint32(0), 0, 0, 1 << 40, TrapStatus},
		{"fill-destination-overflow", 1, ^uint32(0), 99, 2, 0, 0, 1 << 40, TrapStatus},
		{"copy-length-overflow", 0, 510, 256, ^uint32(0), 0, 0, 1 << 40, TrapStatus},
		{"zero-copy-bad-source", 0, 65536, 65537, 0, 0, 0, 1 << 30, TrapStatus},
		{"zero-copy-bad-destination", 0, 65537, 65536, 0, 0, 0, 1 << 30, TrapStatus},
		{"zero-fill-bad-destination", 1, 65537, 99, 0, 0, 0, 1 << 30, TrapStatus},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, h := testHost(t)
			p := deployCase(t, h, "bulk")
			root := commit(t, db)
			before := bytes.Clone(p.memory)
			result := p.Run(context.Background(), Call{Input: bulkInput(tc.fill, tc.dst, tc.src, tc.n, tc.status, tc.trap), Gas: tc.gas})
			if result.Status != tc.want || !bytes.Equal(before, p.memory) || commit(t, db) != root {
				t.Fatalf("status %d or revert changed state", result.Status)
			}
			if tc.status == 0 && tc.trap == 0 && !reflect.DeepEqual(result.DirtyLines, []uint32{0}) {
				t.Fatalf("invalid/OOG bulk partially journaled destination: %v", result.DirtyLines)
			}
			if next := p.Run(context.Background(), Call{Input: bulkInput(1, 510, 99, 10, 0, 0), Gas: 1 << 30}); next.Status != 0 {
				t.Fatal("call after revert failed")
			}
		})
	}
}

func TestBulkLengthGas(t *testing.T) {
	for _, fill := range []uint32{0, 1} {
		t.Run(string(rune('0'+fill)), func(t *testing.T) {
			db, h := testHost(t)
			p := deployCase(t, h, "bulk")
			commit(t, db)
			run := func(n uint32, gas uint64) Result {
				return p.Run(context.Background(), Call{Input: bulkInput(fill, 512, 256, n, 0, 0), Gas: gas})
			}
			zero, a, b := run(0, 100000), run(1, 100000), run(2, 100000)
			if zero.Status != 0 || zero.GasLeft-a.GasLeft != BulkByteGas+lineSize*ByteGas {
				t.Fatal("zero-length gas")
			}
			if a.Status != 0 || b.Status != 0 || a.GasLeft-b.GasLeft != BulkByteGas {
				t.Fatalf("length gas %d %d", a.GasLeft, b.GasLeft)
			}
			used := 100000 - b.GasLeft
			if r := run(2, used); r.Status != 0 || r.GasLeft != 0 {
				t.Fatal("exact gas failed")
			}
			root := commit(t, db)
			before := bytes.Clone(p.memory)
			r := run(2, used-1)
			if r.Status != TrapStatus || r.GasLeft != 0 || !bytes.Equal(before, p.memory) || commit(t, db) != root {
				t.Fatal("final state gas did not revert bulk")
			}
		})
	}
}
