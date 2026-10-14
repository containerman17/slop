package pagerwasm

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"golang.org/x/sys/unix"
)

// Opt-in diagnostic control. Take ~/.cache/pagerdb-bench.lock before running.
// This deliberately has no PagerDB writes; it isolates execution without Touch.
func TestDiagnostics(t *testing.T) {
	if os.Getenv("PAGERWASM_DIAGNOSTICS") != "1" {
		t.Skip("opt-in diagnostics")
	}
	f, per, blocks, golden := fixture(t)
	ctx := context.Background()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	metered := filepath.Join(t.TempDir(), "metered.wasm")
	cmd := exec.Command(filepath.Join(home, ".cargo/bin/cargo"), "run", "--quiet", "--locked", "--release", "--manifest-path", "instrument/Cargo.toml", "--bin", "meter-only", "--", "testdata/matcher.wasm", metered, strconv.Itoa(InstructionGas), strconv.Itoa(StackLimit))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, path := range []string{"testdata/matcher.wasm", metered} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			rt := wazero.NewRuntime(ctx)
			defer rt.Close(ctx)
			var input, output []byte
			var gas api.MutableGlobal
			hostCharge := func() {
				if gas != nil {
					gas.Set(gas.Get() - HostGas)
				}
			}
			_, err := rt.NewHostModuleBuilder("env").NewFunctionBuilder().WithFunc(func() uint32 { hostCharge(); return uint32(len(input)) }).Export("input_len").NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, off uint32) {
				hostCharge()
				if !m.Memory().Write(off, input) {
					panic("bounds")
				}
			}).Export("input").NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, off, n uint32) {
				hostCharge()
				b, ok := m.Memory().Read(off, n)
				if !ok {
					panic("bounds")
				}
				output = append(output[:0], b...)
			}).Export("output").Instantiate(ctx)
			if err != nil {
				t.Fatal(err)
			}
			code, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			memory, err := unix.Mmap(-1, 0, 512<<20, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Munmap(memory)
			allocCtx := experimental.WithMemoryAllocator(ctx, experimental.MemoryAllocatorFunc(func(_, _ uint64) experimental.LinearMemory { return regionMemory(memory) }))
			m, err := rt.InstantiateWithConfig(allocCtx, code, wazero.NewModuleConfig().WithStartFunctions())
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close(ctx)
			if g := m.ExportedGlobal("__gas"); g != nil {
				gas = g.(api.MutableGlobal)
			}
			call := m.ExportedFunction("call")
			input = make([]byte, per*40)
			if _, err = f.Seek(16, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			var elapsed time.Duration
			for i := 0; i < blocks; i++ {
				if _, err = io.ReadFull(f, input); err != nil {
					t.Fatal(err)
				}
				if gas != nil {
					gas.Set(1 << 50)
				}
				start := time.Now()
				r, err := call.Call(ctx)
				elapsed += time.Since(start)
				if err != nil || r[0] != 0 {
					t.Fatalf("block %d: %v %v", i, r, err)
				}
				if hash := fnv(output); hash != golden[i] {
					t.Fatalf("block %d hash %x", i, hash)
				}
			}
			t.Logf("single-process M3 diagnostics: %s %.0f msgs/s, %.3f execution ms/block", filepath.Base(path), float64(per*blocks)/elapsed.Seconds(), elapsed.Seconds()*1000/float64(blocks))
		})
	}
}
