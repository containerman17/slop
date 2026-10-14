package pagerwasm

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func TestInlineMeterMatchesHelper(t *testing.T) {
	ctx := context.Background()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		trapped bool
		gas     uint64
		results []uint64
	}
	var expected []outcome
	budgets := make([]uint64, 257)
	for i := range budgets {
		budgets[i] = uint64(i)
	}
	budgets = append(budgets, math.MaxInt64)
	for _, mode := range []string{"legacy", "inline"} {
		path := filepath.Join(t.TempDir(), "meter.wasm")
		cmd := exec.Command(filepath.Join(home, ".cargo/bin/cargo"), "run", "--quiet", "--locked", "--release", "--manifest-path", "instrument/Cargo.toml", "--bin", "meter-only", "--", "testdata/cases/meter.wasm", path, "1", "65536", mode, strconv.Itoa(BulkByteGas))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		code, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rt := wazero.NewRuntime(ctx)
		defer rt.Close(ctx)
		compiled, err := rt.CompileModule(ctx, code)
		if err != nil {
			t.Fatal(err)
		}
		var got []outcome
		for _, budget := range budgets {
			m, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
			if err != nil {
				t.Fatal(err)
			}
			g := m.ExportedGlobal("__gas").(api.MutableGlobal)
			g.Set(budget)
			values, err := m.ExportedFunction("call").Call(ctx)
			got = append(got, outcome{err != nil, g.Get(), values})
			m.Close(ctx)
		}
		if mode == "legacy" {
			expected = got
		} else if !reflect.DeepEqual(got, expected) {
			t.Fatal("inline gas, traps or results differ from helper")
		}
	}
}
