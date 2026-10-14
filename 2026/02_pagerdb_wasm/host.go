// Package pagerwasm runs fixed-memory integer Wasm over PagerDB. One goroutine owns
// the Host, its Programs and the database. Close the Host before the database.
package pagerwasm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/containerman17/pagerdb"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// These are prototype consensus constants, not calibrated production prices.
const (
	InstructionGas        = 1
	HostGas               = 10
	DirtyLineGas          = 100
	StackLimit            = 65536
	TrapStatus     uint32 = math.MaxUint32
	lineSize              = 64
	bitmapBytes           = 1 << 20
)

type regionMemory []byte

func (m regionMemory) Reallocate(n uint64) []byte {
	if n > uint64(len(m)) {
		return nil
	}
	return m[:n]
}
func (m regionMemory) Free() {} // PagerDB owns the mapping.

type artifact struct {
	module wazero.CompiledModule
	limit  uint32
	data   [][2]int
}
type Host struct {
	db     *pagerdb.DB
	rt     wazero.Runtime
	cache  map[[32]byte]*artifact
	active *Program
}
type Program struct {
	host    *Host
	Region  pagerdb.Region
	memory  []byte
	limit   uint32
	module  api.Module
	gas     api.MutableGlobal
	globals []api.MutableGlobal
	call    api.Function
	args    Call
	result  Result
}

// Call supplies deterministic block and transaction values. Gas must fit in int64.
type Call struct {
	BlockTime, BlockNumber uint64
	Caller                 [20]byte
	Value                  [32]byte
	CallType               uint32
	Input                  []byte
	Gas                    uint64
}
type Result struct {
	Status     uint32
	Output     []byte
	Logs       [][]byte
	GasLeft    uint64
	DirtyLines []uint32
}

func New(ctx context.Context, db *pagerdb.DB) (*Host, error) {
	h := &Host{db: db, rt: wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().WithMemoryLimitPages(8192+bitmapBytes/65536)), cache: make(map[[32]byte]*artifact)}
	b := h.rt.NewHostModuleBuilder("env")
	add := func(name string, params, results []api.ValueType, fn func(*Program, []uint64)) {
		b.NewFunctionBuilder().WithGoFunction(api.GoFunc(func(_ context.Context, s []uint64) {
			p := h.active
			if p == nil {
				panic("host import outside call")
			}
			p.charge(HostGas)
			fn(p, s)
		}), params, results).Export(name)
	}
	i32, i64 := []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}
	add("block_time", nil, i64, func(p *Program, s []uint64) { s[0] = p.args.BlockTime })
	add("block_number", nil, i64, func(p *Program, s []uint64) { s[0] = p.args.BlockNumber })
	add("gas_left", nil, i64, func(p *Program, s []uint64) { s[0] = p.gas.Get() })
	add("call_type", nil, i32, func(p *Program, s []uint64) { s[0] = uint64(p.args.CallType) })
	add("input_len", nil, i32, func(p *Program, s []uint64) { s[0] = uint64(len(p.args.Input)) })
	add("caller", i32, nil, func(p *Program, s []uint64) { p.write(uint32(s[0]), p.args.Caller[:]) })
	add("value", i32, nil, func(p *Program, s []uint64) { p.write(uint32(s[0]), p.args.Value[:]) })
	add("input", i32, nil, func(p *Program, s []uint64) { p.write(uint32(s[0]), p.args.Input) })
	add("output", []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil, func(p *Program, s []uint64) { p.result.Output = bytes.Clone(p.read(uint32(s[0]), uint32(s[1]))) })
	add("emit_log", []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil, func(p *Program, s []uint64) {
		p.result.Logs = append(p.result.Logs, bytes.Clone(p.read(uint32(s[0]), uint32(s[1]))))
	})
	add("touch", i32, nil, func(p *Program, s []uint64) { p.touch(uint32(s[0])) })
	if _, err := b.Instantiate(ctx); err != nil {
		h.rt.Close(ctx)
		return nil, err
	}
	return h, nil
}
func (h *Host) Close(ctx context.Context) error { return h.rt.Close(ctx) }

func (h *Host) compile(ctx context.Context, code []byte) (*artifact, error) {
	key := sha256.Sum256(code)
	if a := h.cache[key]; a != nil {
		return a, nil
	}
	dir, err := os.MkdirTemp("", "pagerwasm-deploy-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	src, dst := filepath.Join(dir, "in.wasm"), filepath.Join(dir, "out.wasm")
	if err = os.WriteFile(src, code, 0600); err != nil {
		return nil, err
	}
	_, file, _, _ := runtime.Caller(0)
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		home, e := os.UserHomeDir()
		if e != nil {
			return nil, e
		}
		cargo = filepath.Join(home, ".cargo/bin/cargo")
	}
	// shortcut: Rust subprocess at deploy, port to Go or cgo when the host ships
	cmd := exec.CommandContext(ctx, cargo, "run", "--quiet", "--locked", "--release", "--manifest-path", filepath.Join(filepath.Dir(file), "instrument/Cargo.toml"), "--bin", "rust-instrument", "--", src, dst, strconv.Itoa(InstructionGas), strconv.Itoa(StackLimit))
	if out, e := cmd.CombinedOutput(); e != nil {
		return nil, fmt.Errorf("instrumentation rejected module: %w: %s", e, out)
	}
	out, err := os.ReadFile(dst)
	if err != nil {
		return nil, err
	}
	meta, err := os.ReadFile(dst + ".meta")
	if err != nil {
		return nil, err
	}
	a := &artifact{}
	reader := bytes.NewReader(meta)
	if _, err = fmt.Fscan(reader, &a.limit); err != nil {
		return nil, err
	}
	for {
		var r [2]int
		n, e := fmt.Fscan(reader, &r[0], &r[1])
		if e == io.EOF && n == 0 {
			break
		}
		if e != nil {
			return nil, e
		}
		a.data = append(a.data, r)
	}
	a.module, err = h.rt.CompileModule(ctx, out)
	if err != nil {
		return nil, err
	}
	f := a.module.ExportedFunctions()["call"]
	if f == nil || len(f.ParamTypes()) != 0 || len(f.ResultTypes()) != 1 || f.ResultTypes()[0] != api.ValueTypeI32 {
		a.module.Close(ctx)
		return nil, fmt.Errorf("call must have signature () -> i32")
	}
	h.cache[key] = a
	return a, nil
}

// Deploy creates a new region. The caller commits deployment before execution.
// The module cache lives for this Host's lifetime and is keyed by original code hash.
func (h *Host) Deploy(ctx context.Context, code []byte) (*Program, error) {
	a, err := h.compile(ctx, code)
	if err != nil {
		return nil, err
	}
	mark := h.db.Mark()
	r, err := h.db.Alloc(int(a.limit) + bitmapBytes)
	if err != nil {
		return nil, err
	}
	mem, err := h.db.Map(r)
	if err != nil {
		h.db.Undo(mark)
		return nil, err
	}
	for _, d := range a.data {
		if d[1] > 0 {
			h.db.Touch(r, d[0], d[1])
		}
	}
	ctx = experimental.WithMemoryAllocator(ctx, experimental.MemoryAllocatorFunc(func(_, _ uint64) experimental.LinearMemory { return regionMemory(mem[:int(a.limit)+bitmapBytes]) }))
	m, err := h.rt.InstantiateModule(ctx, a.module, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		h.db.Undo(mark)
		return nil, err
	}
	p := &Program{host: h, Region: r, memory: mem, limit: a.limit, module: m, gas: m.ExportedGlobal("__gas").(api.MutableGlobal), call: m.ExportedFunction("call")}
	for i := 0; ; i++ {
		g := m.ExportedGlobal(fmt.Sprintf("__global_%d", i))
		if g == nil {
			break
		}
		p.globals = append(p.globals, g.(api.MutableGlobal))
	}
	return p, nil
}
func (p *Program) charge(n uint64) {
	left := p.gas.Get()
	if left > math.MaxInt64 || left < n {
		p.gas.Set(0)
		panic("out of gas")
	}
	p.gas.Set(left - n)
}
func (p *Program) read(off, n uint32) []byte {
	end := uint64(off) + uint64(n)
	if end > uint64(p.limit) {
		panic("ABI memory bounds")
	}
	return p.memory[off:end]
}
func (p *Program) touch(line uint32) {
	if line >= p.limit/lineSize {
		panic("reserved bitmap store")
	}
	off, bit := p.limit+line/8, byte(1<<(line%8))
	if p.memory[off]&bit != 0 {
		return
	}
	p.host.db.Touch(p.Region, int(line)*lineSize, lineSize)
	p.memory[off] |= bit
}
func (p *Program) write(off uint32, b []byte) {
	dst := p.read(off, uint32(len(b)))
	if len(b) > 0 {
		end := uint32((uint64(off)+uint64(len(b))-1)/lineSize) + 1
		for line := off / lineSize; line < end; {
			if p.memory[p.limit+line/8]&byte(1<<(line%8)) != 0 {
				line++
				continue
			}
			start := line
			for line < end && p.memory[p.limit+line/8]&byte(1<<(line%8)) == 0 {
				line++
			}
			p.host.db.Touch(p.Region, int(start)*lineSize, int(line-start)*lineSize)
			for i := start; i < line; i++ {
				p.memory[p.limit+i/8] |= byte(1 << (i % 8))
			}
		}
	}
	copy(dst, b)
}

// Run reverts bytes and globals on nonzero status, traps, or exhausted gas.
// It does not commit. Trap text is never included in Result.
func (p *Program) Run(ctx context.Context, args Call) (res Result) {
	if p.host.active != nil {
		panic("nested execution unsupported")
	}
	if args.Gas > math.MaxInt64 || uint64(len(args.Input)) > uint64(p.limit) {
		return Result{Status: TrapStatus}
	}
	mark := p.host.db.Mark()
	saved := make([]uint64, len(p.globals))
	for i, g := range p.globals {
		saved[i] = g.Get()
	}
	p.args = args
	p.result = Result{}
	p.gas.Set(args.Gas)
	p.host.active = p
	defer func() {
		p.host.active = nil
		// A crash mid-call can leave bitmap bytes on disk; recovery detects the root mismatch and the node resyncs, as with PagerDB power loss.
		bitmap := p.memory[p.limit : p.limit+bitmapBytes]
		for off := 0; off < len(bitmap); off += 8 {
			word := binary.LittleEndian.Uint64(bitmap[off:])
			if word == 0 {
				continue
			}
			binary.LittleEndian.PutUint64(bitmap[off:], 0)
			for word != 0 {
				p.result.DirtyLines = append(p.result.DirtyLines, uint32(off*8+bits.TrailingZeros64(word)))
				word &= word - 1
			}
		}
		left := p.gas.Get()
		cost := uint64(len(p.result.DirtyLines)) * DirtyLineGas
		if left > math.MaxInt64 || left < cost {
			p.result.Status = TrapStatus
			p.result.Output = nil
			left = 0
		} else {
			left -= cost
		}
		p.result.GasLeft = left
		if p.result.Status != 0 {
			p.host.db.Undo(mark)
			for i, g := range p.globals {
				g.Set(saved[i])
			}
			p.result.Logs = nil
		}
		res = p.result
		p.args = Call{}
		p.result = Result{}
	}()
	out, err := p.call.Call(ctx)
	if err != nil {
		p.result.Status = TrapStatus
		p.result.Output = nil
	} else {
		p.result.Status = uint32(out[0])
		// Guest mutable globals are scratch. Persistent state must live in the region.
		for i, g := range p.globals[:len(p.globals)-2] {
			if p.result.Status == 0 && g.Get() != saved[i] {
				p.result.Status = TrapStatus
				p.result.Output = nil
				break
			}
		}
	}
	return
}
