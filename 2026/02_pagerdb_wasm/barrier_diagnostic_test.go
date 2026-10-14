package pagerwasm

import (
	"bytes"
	"context"
	"io"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/containerman17/pagerdb"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Diagnostic controls deliberately bypass undo. Never use them for persisted state.
type barrierTimedCall struct {
	api.Function
	guest, scan time.Duration
	after       func()
}

func (f *barrierTimedCall) Call(ctx context.Context, args ...uint64) (out []uint64, err error) {
	start := time.Now()
	pprof.Do(ctx, pprof.Labels("phase", "guest"), func(ctx context.Context) { out, err = f.Function.Call(ctx, args...) })
	f.guest += time.Since(start)
	if f.after != nil {
		start = time.Now()
		pprof.Do(ctx, pprof.Labels("phase", "diagnostic-scan"), func(context.Context) { f.after() })
		f.scan += time.Since(start)
	}
	return
}
func barrierTouchWithoutDB(p *Program, line uint32) {
	if line >= p.limit/lineSize {
		panic("reserved bitmap store")
	}
	off, bit := p.limit+line/8, byte(1<<(line%8))
	if p.memory[off]&bit != 0 {
		return
	}
	p.result.DirtyLines = append(p.result.DirtyLines, line)
	p.memory[off] |= bit
}

func TestBarrierDiagnostics(t *testing.T) {
	if os.Getenv("PAGERWASM_BARRIER_DIAGNOSTICS") != "1" {
		t.Skip("opt-in barrier controls")
	}
	f, per, blocks, golden := fixture(t)
	ctx := context.Background()
	source, err := os.ReadFile("testdata/matcher.wasm")
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	rust := func(bin string, args ...string) {
		t.Helper()
		cmd := exec.Command(filepath.Join(home, ".cargo/bin/cargo"), append([]string{"run", "--quiet", "--locked", "--release", "--manifest-path", "instrument/Cargo.toml", "--bin", bin, "--"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	full := filepath.Join(dir, "full.wasm")
	rust("rust-instrument", "testdata/matcher.wasm", full, "1", "65536")
	metered := filepath.Join(dir, "metered.wasm")
	rust("meter-only", "testdata/matcher.wasm", metered, "1", "65536", "inline-stack")
	rust("barrier-control", metered, metered, "metered")
	paths := map[string]string{"metered": metered, "host": full, "full": full}
	for _, mode := range []string{"stub", "empty"} {
		paths[mode] = filepath.Join(dir, mode+".wasm")
		rust("barrier-control", full, paths[mode], mode)
	}
	modes := []string{"metered", "stub", "empty", "host", "full"}
	if mode := os.Getenv("PAGERWASM_BARRIER_MODE"); mode != "" {
		modes = strings.Split(mode, ",")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			db, err := pagerdb.Open(t.TempDir(), pagerdb.Options{RetainVersions: 2})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()
			h := &Host{db: db, rt: wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().WithMemoryLimitPages(8208)), cache: make(map[[32]byte]*artifact)}
			defer h.Close(ctx)
			var inputTime time.Duration
			var misses, inputTouches, stackMisses, outsideFrame uint64
			b := h.rt.NewHostModuleBuilder("env")
			add := func(name string, params, results []api.ValueType, fn func(*Program, []uint64)) {
				b.NewFunctionBuilder().WithGoFunction(api.GoFunc(func(_ context.Context, s []uint64) {
					p := h.active
					if p == nil {
						panic("outside call")
					}
					p.charge(HostGas)
					fn(p, s)
				}), params, results).Export(name)
			}
			i32 := []api.ValueType{api.ValueTypeI32}
			add("input_len", nil, i32, func(p *Program, s []uint64) { s[0] = uint64(len(p.args.Input)) })
			add("input", i32, nil, func(p *Program, s []uint64) {
				start := time.Now()
				off := uint32(s[0])
				data := p.args.Input
				if mode == "metered" {
					copy(p.read(off, uint32(len(data))), data)
				} else {
					before := len(p.result.DirtyLines)
					if mode == "full" {
						p.write(off, data)
					} else {
						dst := p.read(off, uint32(len(data)))
						if len(data) > 0 {
							for line := off / lineSize; line <= uint32((uint64(off)+uint64(len(data))-1)/lineSize); line++ {
								barrierTouchWithoutDB(p, line)
							}
						}
						copy(dst, data)
					}
					inputTouches += uint64(len(p.result.DirtyLines) - before)
				}
				inputTime += time.Since(start)
			})
			add("output", []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil, func(p *Program, s []uint64) { p.result.Output = bytes.Clone(p.read(uint32(s[0]), uint32(s[1]))) })
			add("touch", i32, nil, func(p *Program, s []uint64) {
				misses++
				switch mode {
				case "full":
					line := uint32(s[0])
					if line*lineSize < 1<<20 {
						stackMisses++
						if (line+1)*lineSize <= uint32(p.globals[0].Get()) {
							outsideFrame++
						}
					}
					p.touch(line)
				case "host":
					barrierTouchWithoutDB(p, uint32(s[0]))
				case "empty":
				default:
					panic("unexpected touch")
				}
			})
			if _, err := b.Instantiate(ctx); err != nil {
				t.Fatal(err)
			}
			// Reuse production deployment metadata and initialization with a diagnostic code body.
			a, err := h.compile(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			code, err := os.ReadFile(paths[mode])
			if err != nil {
				t.Fatal(err)
			}
			a.module.Close(ctx)
			a.module, err = h.rt.CompileModule(ctx, code)
			if err != nil {
				t.Fatal(err)
			}
			p, err := h.Deploy(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			commit(t, db)
			// This matcher has one original mutable global: the downward shadow-stack pointer.
			stackTop := uint32(p.globals[0].Get())
			if stackTop != 1<<20 {
				t.Fatalf("unexpected matcher stack top %d", stackTop)
			}
			// The allocator initialization in this pinned matcher starts at 1051136.
			// Data segments end before this 64-byte-aligned boundary.
			const heapBase = 1051136
			dataEnd := 0
			for _, d := range a.data {
				if end := d[0] + d[1]; end > dataEnd {
					dataEnd = end
				}
			}
			if dataEnd != 1051112 {
				t.Fatalf("unexpected matcher data end %d", dataEnd)
			}
			t.Logf("layout: shadow stack [0,%d), static [%d,%d), heap [%d,%d); data_end=%d", stackTop, stackTop, heapBase, heapBase, p.limit, dataEnd)
			timed := &barrierTimedCall{Function: p.call}
			if mode == "stub" || mode == "empty" {
				timed.after = func() {
					p.result.DirtyLines = p.result.DirtyLines[:0]
					for i, b := range p.memory[p.limit : p.limit+bitmapBytes] {
						for b != 0 {
							bit := bits.TrailingZeros8(b)
							p.result.DirtyLines = append(p.result.DirtyLines, uint32(i*8+bit))
							b &= b - 1
						}
					}
				}
			}
			p.call = timed
			profilePath := os.Getenv("PAGERWASM_BARRIER_PROFILE")
			if profilePath != "" {
				if len(modes) != 1 || mode != "full" {
					t.Fatal("profile only one full control")
				}
				profile, err := os.Create(profilePath)
				if err != nil {
					t.Fatal(err)
				}
				defer profile.Close()
				if err := pprof.StartCPUProfile(profile); err != nil {
					t.Fatal(err)
				}
				defer pprof.StopCPUProfile()
			}
			if _, err = f.Seek(16, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			input := make([]byte, per*40)
			var elapsed, commits time.Duration
			var dirty, stackLines, staticLines uint64
			minStack := stackTop
			var first, lastStack int
			for block := 0; block < blocks; block++ {
				if _, err = io.ReadFull(f, input); err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				var result Result
				pprof.Do(ctx, pprof.Labels("phase", "run"), func(ctx context.Context) {
					result = p.Run(ctx, Call{Input: input, Gas: 1 << 50, BlockNumber: uint64(block)})
				})
				elapsed += time.Since(start)
				if result.Status != 0 || fnv(result.Output) != golden[block] {
					t.Fatalf("block %d failed status=%d", block, result.Status)
				}
				nstack := 0
				for _, line := range result.DirtyLines {
					if line*lineSize >= stackTop && line*lineSize < heapBase {
						staticLines++
					}
					if line*lineSize < stackTop {
						nstack++
						if line*lineSize < minStack {
							minStack = line * lineSize
						}
					}
				}
				if block == 0 {
					first = nstack
				}
				lastStack = nstack
				stackLines += uint64(nstack)
				dirty += uint64(len(result.DirtyLines))
				if mode == "full" {
					start = time.Now()
					pprof.Do(ctx, pprof.Labels("phase", "commit"), func(context.Context) { commit(t, db) })
					commits += time.Since(start)
				}
				if (block+1)%100 == 0 {
					t.Logf("%d/%d output hashes checked", block+1, blocks)
				}
			}
			pprof.StopCPUProfile()
			ms := func(d time.Duration) float64 { return d.Seconds() * 1000 / float64(blocks) }
			t.Logf("single-process M3 diagnostic mode=%s blocks=%d guest_ms=%.3f input_ms=%.3f end_ms=%.3f scan_ms=%.3f Run_ms=%.3f msgs/s=%.0f Commit_ms=%.3f", mode, blocks, ms(timed.guest), ms(inputTime), ms(elapsed-timed.guest-timed.scan), ms(timed.scan), ms(elapsed-timed.scan), float64(per*blocks)/(elapsed-timed.scan).Seconds(), ms(commits))
			t.Logf("dirty_lines/block=%.3f stack_lines/block=%.3f nonstack_lines/block=%.3f guest_touch_calls/block=%.3f ABI_first_touches/block=%.3f stack_top=%d min_dirty_stack_address=%d first/last_stack_lines=%d/%d", float64(dirty)/float64(blocks), float64(stackLines)/float64(blocks), float64(dirty-stackLines)/float64(blocks), float64(misses)/float64(blocks), float64(inputTouches)/float64(blocks), stackTop, minStack, first, lastStack)
			t.Logf("static_lines/block=%.3f heap_lines/block=%.3f first_stack_misses_in_active_frame=%d outside_frame=%d", float64(staticLines)/float64(blocks), float64(dirty-stackLines-staticLines)/float64(blocks), stackMisses-outsideFrame, outsideFrame)
			if mode != "metered" && dirty == 0 {
				t.Fatalf("no dirty lines in %s", mode)
			}
		})
	}
}
