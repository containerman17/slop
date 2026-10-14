package pagerwasm

import (
	"context"
	"encoding/hex"
	"github.com/containerman17/pagerdb"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental"
	"testing"
)

func TestExternalMemory(t *testing.T) {
	db, err := pagerdb.Open(t.TempDir(), pagerdb.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := db.Alloc(65536)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.Map(r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := experimental.WithMemoryAllocator(context.Background(), experimental.MemoryAllocatorFunc(func(_, _ uint64) experimental.LinearMemory { return regionMemory(b) }))
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	// (memory 1 1) (export "memory" (memory 0)) (func (export "call") i32.const 0 i32.const 42 i32.store)
	code, _ := hex.DecodeString("0061736d0100000001040160000003020100050401010101071102066d656d6f727902000463616c6c00000a0b0109004100412a3602000b")
	m, err := rt.Instantiate(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	db.Touch(r, 0, 4096)
	if _, err = m.ExportedFunction("call").Call(ctx); err != nil {
		t.Fatal(err)
	}
	view, _ := m.Memory().Read(0, 65536)
	if &view[0] != &b[0] || b[0] != 42 {
		t.Fatal("linear memory does not alias PagerDB")
	}
}
