package pagerwasm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/containerman17/pagerdb"
)

func testHost(t *testing.T) (*pagerdb.DB, *Host) {
	t.Helper()
	db, err := pagerdb.Open(t.TempDir(), pagerdb.Options{RetainVersions: 2})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(context.Background()); err != nil {
			t.Error(err)
		}
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, h
}
func deployCase(t *testing.T, h *Host, name string) *Program {
	t.Helper()
	code, err := os.ReadFile("testdata/cases/" + name + ".wasm")
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.Deploy(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func commit(t *testing.T, db *pagerdb.DB) [32]byte {
	t.Helper()
	r, _, err := db.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestBarrierAndBitmap(t *testing.T) {
	db, h := testHost(t)
	p := deployCase(t, h, "write")
	commit(t, db)
	before := bytes.Clone(p.memory)
	got := p.Run(context.Background(), Call{Input: []byte{0}, Gas: 1 << 30})
	if got.Status != 0 {
		t.Fatalf("status %d", got.Status)
	}
	var changed []uint32
	for off := 0; off < len(before); off += lineSize {
		if !bytes.Equal(before[off:off+lineSize], p.memory[off:off+lineSize]) {
			changed = append(changed, uint32(off/lineSize))
		}
	}
	if !reflect.DeepEqual(got.DirtyLines, changed) {
		t.Fatalf("barrier %v memcmp %v", got.DirtyLines, changed)
	}
	if len(got.Logs) != 1 || len(got.Output) != 8 {
		t.Fatalf("result %+v", got)
	}
	if !bytes.Equal(p.memory[p.limit:p.limit+bitmapBytes], make([]byte, bitmapBytes)) {
		t.Fatal("bitmap not cleared")
	}
	root := commit(t, db)
	if err := db.Verify(); err != nil {
		t.Fatal(err)
	}
	// Independent DB receives only the expected application stores, no bitmap writes.
	other, err := pagerdb.Open(t.TempDir(), pagerdb.Options{RetainVersions: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	r, err := other.Alloc(int(p.limit) + bitmapBytes)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, other)
	var val [8]byte
	binary.LittleEndian.PutUint64(val[:], 123456)
	other.Write(r, 4094, val[:])
	other.Write(r, 12300, []byte{99})
	other.Write(r, 16, []byte{1})
	binary.LittleEndian.PutUint64(val[:], 72623859790382856)
	other.Write(r, 62, val[:])
	other.Write(r, 1022, val[:])
	if want := commit(t, other); root != want {
		t.Fatalf("bitmap changed root: %x != %x", root, want)
	}
}
func TestRevert(t *testing.T) {
	for _, mode := range []byte{1, 2, 3} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			db, h := testHost(t)
			p := deployCase(t, h, "write")
			beforeRoot := commit(t, db)
			before := bytes.Clone(p.memory)
			got := p.Run(context.Background(), Call{Input: []byte{mode}, Gas: 100000})
			want := TrapStatus
			if mode == 1 {
				want = 1
			}
			if got.Status != want {
				t.Fatalf("status %d want %d", got.Status, want)
			}
			if len(got.DirtyLines) != 7 {
				t.Fatalf("did not reach writes: %v", got.DirtyLines)
			}
			if !bytes.Equal(before, p.memory) || db.Root() != beforeRoot {
				t.Fatal("revert changed bytes or root")
			}
			if after := commit(t, db); after != beforeRoot {
				t.Fatal("revert changed committed root")
			}
			if len(got.Logs) != 0 {
				t.Fatal("reverted logs escaped")
			}
			if next := p.Run(context.Background(), Call{Input: []byte{0}, Gas: 1 << 30}); next.Status != 0 {
				t.Fatalf("call after revert failed: %d", next.Status)
			}
		})
	}
}
func TestDeployRejects(t *testing.T) {
	_, h := testHost(t)
	for _, name := range []string{"clock", "float", "simd", "bulk", "touch"} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile("testdata/cases/" + name + ".wasm")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.Deploy(context.Background(), b); err == nil {
				t.Fatal("accepted forbidden module")
			}
		})
	}
}
func TestTrapBoundaries(t *testing.T) {
	for _, name := range []string{"reserved", "overflow", "stack"} {
		t.Run(name, func(t *testing.T) {
			db, h := testHost(t)
			p := deployCase(t, h, name)
			root := commit(t, db)
			before := bytes.Clone(p.memory)
			got := p.Run(context.Background(), Call{Gas: 1 << 40})
			if got.Status != TrapStatus {
				t.Fatalf("status %d", got.Status)
			}
			if !bytes.Equal(before, p.memory) || commit(t, db) != root {
				t.Fatal("trap changed state")
			}
		})
	}
}
func TestCache(t *testing.T) {
	_, h := testHost(t)
	a := deployCase(t, h, "write")
	b := deployCase(t, h, "write")
	if len(h.cache) != 1 || a.Region == b.Region {
		t.Fatal("cache or region isolation broken")
	}
}

func TestLineGasRevertsAtCallEnd(t *testing.T) {
	db, h := testHost(t)
	p := deployCase(t, h, "write")
	root := commit(t, db)
	mark := db.Mark()
	initial := bytes.Clone(p.memory)
	first := p.Run(context.Background(), Call{Input: []byte{0}, Gas: 100000})
	if first.Status != 0 {
		t.Fatal(first.Status)
	}
	used := 100000 - first.GasLeft
	db.Undo(mark)
	got := p.Run(context.Background(), Call{Input: []byte{0}, Gas: used - 1})
	if got.Status != TrapStatus || got.GasLeft != 0 || len(got.DirtyLines) != 7 {
		t.Fatalf("line gas result %+v", got)
	}
	if !bytes.Equal(initial, p.memory) || commit(t, db) != root {
		t.Fatal("end-of-call gas failure changed state")
	}
}

func TestGlobalsAreScratch(t *testing.T) {
	db, h := testHost(t)
	p := deployCase(t, h, "globals")
	root := commit(t, db)
	for _, status := range []byte{0, 1} {
		got := p.Run(context.Background(), Call{Input: []byte{status}, Gas: 100000})
		want := TrapStatus
		if status == 1 {
			want = 1
		}
		if got.Status != want {
			t.Fatalf("status %d want %d", got.Status, want)
		}
		if p.globals[0].Get() != 7 || commit(t, db) != root {
			t.Fatal("global or bytes not restored")
		}
	}
}

func TestABI(t *testing.T) {
	db, h := testHost(t)
	p := deployCase(t, h, "abi")
	commit(t, db)
	args := Call{BlockTime: 123, BlockNumber: 456, CallType: 2, Input: []byte{1, 2, 3, 4}, Gas: 100000}
	for i := range args.Caller {
		args.Caller[i] = byte(i + 1)
	}
	for i := range args.Value {
		args.Value[i] = byte(i + 21)
	}
	got := p.Run(context.Background(), args)
	if got.Status != 0 || len(got.Output) != 100 || len(got.Logs) != 1 {
		t.Fatalf("bad result %+v", got)
	}
	b := got.Output
	if !bytes.Equal(b[:20], args.Caller[:]) || !bytes.Equal(b[20:52], args.Value[:]) || !bytes.Equal(b[96:], args.Input) || !bytes.Equal(got.Logs[0], b[:52]) {
		t.Fatal("ABI byte values differ")
	}
	if binary.LittleEndian.Uint64(b[56:]) != 123 || binary.LittleEndian.Uint64(b[64:]) != 456 || binary.LittleEndian.Uint32(b[72:]) != 2 || binary.LittleEndian.Uint32(b[76:]) != 4 {
		t.Fatal("ABI numeric values differ")
	}
	gas := binary.LittleEndian.Uint64(b[80:])
	if gas >= args.Gas || gas <= got.GasLeft {
		t.Fatal("gas_left did not account for charges")
	}
}
