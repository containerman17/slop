//go:build linux

package pagerwasm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"golang.org/x/sys/unix"
)

// This opt-in diagnostic is not a recoverable database. It measures a store bound.
// The production instrumenter, guest, ABI gas and 64-byte first-write barrier stay intact.
type idealGuest struct {
	p    *Program
	undo []byte
}

func (g *idealGuest) touch(line uint32) {
	p := g.p
	if line >= p.limit/64 {
		panic("reserved bitmap store")
	}
	off, bit := p.limit+line/8, byte(1<<(line%8))
	if p.memory[off]&bit != 0 {
		return
	}
	if len(g.undo)+68 > cap(g.undo) {
		panic("preallocated undo exhausted")
	}
	g.undo = binary.LittleEndian.AppendUint32(g.undo, line)
	g.undo = append(g.undo, p.memory[int(line)*64:int(line+1)*64]...)
	p.memory[off] |= bit
}
func (g *idealGuest) touchRange(off, n uint32) {
	g.p.read(off, n)
	if n > 0 {
		for line := off / 64; line <= uint32((uint64(off)+uint64(n)-1)/64); line++ {
			g.touch(line)
		}
	}
}
func (g *idealGuest) write(off uint32, b []byte) {
	g.touchRange(off, uint32(len(b)))
	copy(g.p.read(off, uint32(len(b))), b)
}
func idealDeploy(t *testing.T, huge bool) (*idealGuest, func()) {
	t.Helper()
	ctx := context.Background()
	h := &Host{rt: wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().WithMemoryLimitPages(8208)), cache: make(map[[32]byte]*artifact)}
	g := &idealGuest{}
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
	i32, i64 := []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}
	add("block_time", nil, i64, func(p *Program, s []uint64) { s[0] = p.args.BlockTime })
	add("block_number", nil, i64, func(p *Program, s []uint64) { s[0] = p.args.BlockNumber })
	add("gas_left", nil, i64, func(p *Program, s []uint64) { s[0] = p.gas.Get() })
	add("call_type", nil, i32, func(p *Program, s []uint64) { s[0] = uint64(p.args.CallType) })
	add("input_len", nil, i32, func(p *Program, s []uint64) { s[0] = uint64(len(p.args.Input)) })
	add("caller", i32, nil, func(p *Program, s []uint64) { g.write(uint32(s[0]), p.args.Caller[:]) })
	add("value", i32, nil, func(p *Program, s []uint64) { g.write(uint32(s[0]), p.args.Value[:]) })
	add("input", i32, nil, func(p *Program, s []uint64) { g.write(uint32(s[0]), p.args.Input) })
	add("output", []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil, func(p *Program, s []uint64) { p.result.Output = bytes.Clone(p.read(uint32(s[0]), uint32(s[1]))) })
	add("emit_log", []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil, func(p *Program, s []uint64) {
		p.result.Logs = append(p.result.Logs, bytes.Clone(p.read(uint32(s[0]), uint32(s[1]))))
	})
	add("touch", i32, nil, func(_ *Program, s []uint64) { g.touch(uint32(s[0])) })
	add("touch_range", []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil, func(_ *Program, s []uint64) { g.touchRange(uint32(s[0]), uint32(s[1])) })
	if _, err := b.Instantiate(ctx); err != nil {
		t.Fatal(err)
	}
	code, err := os.ReadFile("testdata/matcher.wasm")
	if err != nil {
		t.Fatal(err)
	}
	a, err := h.compile(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	mem, err := unix.Mmap(-1, 0, int(a.limit)+bitmapBytes, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
	if err != nil {
		t.Fatal(err)
	}
	advice := unix.MADV_NOHUGEPAGE
	if huge {
		advice = unix.MADV_HUGEPAGE
	}
	if err = unix.Madvise(mem, advice); err != nil {
		t.Fatal(err)
	}
	ctx = experimental.WithMemoryAllocator(ctx, experimental.MemoryAllocatorFunc(func(_, _ uint64) experimental.LinearMemory { return regionMemory(mem) }))
	m, err := h.rt.InstantiateModule(ctx, a.module, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		t.Fatal(err)
	}
	p := &Program{host: h, memory: mem, limit: a.limit, module: m, gas: m.ExportedGlobal("__gas").(api.MutableGlobal), call: m.ExportedFunction("call")}
	g.p = p
	for i := 0; ; i++ {
		v := m.ExportedGlobal(fmt.Sprintf("__global_%d", i))
		if v == nil {
			break
		}
		p.globals = append(p.globals, v.(api.MutableGlobal))
	}
	g.undo = make([]byte, 0, int(p.limit)/64*68)
	return g, func() {
		h.Close(context.Background())
		if err := unix.Munmap(mem); err != nil {
			t.Error(err)
		}
	}
}
func (g *idealGuest) run(input []byte, block int) Result {
	p := g.p
	saved := make([]uint64, len(p.globals))
	for i, v := range p.globals {
		saved[i] = v.Get()
	}
	p.args = Call{Input: input, BlockNumber: uint64(block), Gas: 1 << 50}
	p.result = Result{}
	p.gas.Set(p.args.Gas)
	p.host.active = p
	out, err := p.call.Call(context.Background())
	p.host.active = nil
	if err != nil {
		p.result.Status = TrapStatus
	} else {
		p.result.Status = uint32(out[0])
	}
	for i, v := range p.globals[:len(p.globals)-2] {
		if v.Get() != saved[i] {
			p.result.Status = TrapStatus
		}
	}
	bitmap := p.memory[p.limit:]
	last := ^uint32(0)
	for off := 0; off < len(bitmap); off += 8 {
		word := binary.LittleEndian.Uint64(bitmap[off:])
		if word == 0 {
			continue
		}
		binary.LittleEndian.PutUint64(bitmap[off:], 0)
		for word != 0 {
			line := uint32(off*8 + bits.TrailingZeros64(word))
			p.result.DirtyLines = append(p.result.DirtyLines, line)
			if leaf := line / 16; leaf != last {
				p.result.LeavesTouched++
				last = leaf
			}
			word &= word - 1
		}
	}
	p.result.BytesWritten = uint64(len(p.result.DirtyLines)) * 64
	left := p.gas.Get()
	cost := p.result.BytesWritten*ByteGas + p.result.LeavesTouched*LeafGas
	if left > math.MaxInt64 || left < cost {
		p.result.Status = TrapStatus
		left = 0
	} else {
		left -= cost
	}
	p.result.GasLeft = left
	if p.result.Status != 0 {
		for off := len(g.undo) - 68; off >= 0; off -= 68 {
			line := binary.LittleEndian.Uint32(g.undo[off:])
			copy(p.memory[int(line)*64:int(line+1)*64], g.undo[off+4:off+68])
		}
		for i, v := range p.globals {
			v.Set(saved[i])
		}
		p.result.Output = nil
		p.result.Logs = nil
	}
	res := p.result
	p.result = Result{}
	p.args = Call{}
	return res
}

// Persistent workers partition each level. Different workers never update the same node.
type idealWork struct {
	n  int
	fn func(int)
}
type idealPool struct {
	jobs []chan idealWork
	done chan struct{}
}

func newIdealPool() *idealPool {
	p := &idealPool{done: make(chan struct{}, 16)}
	for worker := 0; worker < 16; worker++ {
		ch := make(chan idealWork)
		p.jobs = append(p.jobs, ch)
		go func(w int) {
			for job := range ch {
				for i := job.n * w / 16; i < job.n*(w+1)/16; i++ {
					job.fn(i)
				}
				p.done <- struct{}{}
			}
		}(worker)
	}
	return p
}
func (p *idealPool) each(n int, fn func(int)) {
	if n < 256 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	for _, ch := range p.jobs {
		ch <- idealWork{n, fn}
	}
	for range p.jobs {
		<-p.done
	}
}
func (p *idealPool) close() {
	for _, ch := range p.jobs {
		close(ch)
	}
}

type idealTree struct {
	nodes   [][32]byte
	n, leaf int
	pool    *idealPool
}

func newIdealTree(mem []byte, leaf int, pool *idealPool) *idealTree {
	n := len(mem) / leaf
	if n&(n-1) != 0 {
		panic("tree size")
	}
	tr := &idealTree{make([][32]byte, 2*n), n, leaf, pool}
	pool.each(n, func(i int) { tr.nodes[n+i] = sha256.Sum256(mem[i*leaf : (i+1)*leaf]) })
	for level := n / 2; level > 0; level /= 2 {
		base := level
		pool.each(level, func(i int) { tr.parent(base + i) })
	}
	return tr
}
func (tr *idealTree) parent(i int) {
	var pair [64]byte
	copy(pair[:32], tr.nodes[2*i][:])
	copy(pair[32:], tr.nodes[2*i+1][:])
	tr.nodes[i] = sha256.Sum256(pair[:])
}
func (tr *idealTree) update(ids []int, data []byte, snapshot bool) {
	tr.pool.each(len(ids), func(i int) {
		off := ids[i] * tr.leaf
		if snapshot {
			off = i * tr.leaf
		}
		tr.nodes[tr.n+ids[i]] = sha256.Sum256(data[off : off+tr.leaf])
	})
	parents := make([]int, len(ids))
	for i, id := range ids {
		parents[i] = tr.n + id
	}
	for len(parents) > 1 || parents[0] > 1 {
		next := parents[:0]
		last := -1
		for _, id := range parents {
			v := id / 2
			if v != last {
				next = append(next, v)
				last = v
			}
		}
		parents = next
		tr.pool.each(len(parents), func(i int) { tr.parent(parents[i]) })
	}
}

type idealCommitResult struct {
	root                    [32]byte
	hash, write, sync, pack time.Duration
	bytes, calls            int
	err                     error
}

func idealCommit(tr *idealTree, ids []int, data []byte, snapshot bool, fd int, durable bool, log *idealLog) idealCommitResult {
	var r idealCommitResult
	start := time.Now()
	tr.update(ids, data, snapshot)
	r.root = tr.nodes[1]
	r.hash = time.Since(start)
	if log != nil {
		return log.commit(ids, data, snapshot, r, durable)
	}
	start = time.Now()
	for i := 0; i < len(ids); {
		end := i + 1
		for end < len(ids) && ids[end] == ids[end-1]+1 {
			end++
		}
		off := ids[i] * tr.leaf
		src := off
		if snapshot {
			src = i * tr.leaf
		}
		b := data[src : src+(end-i)*tr.leaf]
		for len(b) > 0 {
			n, err := unix.Pwrite(fd, b, int64(off))
			r.calls++
			r.bytes += n
			if err != nil {
				r.err = err
				return r
			}
			if n == 0 {
				r.err = io.ErrShortWrite
				return r
			}
			off += n
			b = b[n:]
		}
		i = end
	}
	r.write = time.Since(start)
	if durable {
		start = time.Now()
		r.err = unix.Fdatasync(fd)
		r.sync = time.Since(start)
	}
	return r
}
func idealFaults() int64 {
	var r unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &r); err != nil {
		panic(err)
	}
	return r.Minflt
}

func TestIdealStore(t *testing.T) {
	if os.Getenv("PAGERWASM_IDEAL") != "1" {
		t.Skip("opt-in anonymous store diagnostic")
	}
	if runtime.GOMAXPROCS(0) != 16 {
		t.Fatal("requires GOMAXPROCS=16")
	}
	huge := os.Getenv("IDEAL_HUGE") == "1"
	overlap := os.Getenv("IDEAL_OVERLAP") == "1"
	durable := os.Getenv("IDEAL_SYNC") == "1"
	layout := os.Getenv("IDEAL_LAYOUT")
	packed := layout == "log"
	rootOnly := layout == "root"
	if layout != "" && !packed && !rootOnly {
		t.Fatal("unknown IDEAL_LAYOUT")
	}
	parallelSnapshot := os.Getenv("IDEAL_PARALLEL_SNAPSHOT") == "1"
	if parallelSnapshot && !overlap {
		t.Fatal("parallel snapshot requires overlap")
	}
	leaf := 1024
	if v := os.Getenv("IDEAL_LEAF"); v != "" {
		var err error
		leaf, err = strconv.Atoi(v)
		if err != nil || (leaf != 512 && leaf != 1024 && leaf != 4096) {
			t.Fatal("leaf must be 512, 1024 or 4096")
		}
	}
	f, per, blocks, golden := fixture(t)
	if v := os.Getenv("IDEAL_BLOCKS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > blocks {
			t.Fatal("invalid block limit")
		}
		blocks = n
	}
	var reference [][32]byte
	for run := 0; run < 2; run++ {
		t.Run(strconv.Itoa(run), func(t *testing.T) {
			g, cleanup := idealDeploy(t, huge)
			defer cleanup()
			p := g.p
			pool := newIdealPool()
			defer pool.close()
			tr := newIdealTree(p.memory[:p.limit], leaf, pool)
			disk, err := os.CreateTemp(os.Getenv("TMPDIR"), "ideal-state-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(disk.Name())
			defer disk.Close()
			fd := int(disk.Fd())
			fileSize := int64(p.limit)
			if packed {
				fileSize += int64(blocks) * (48 + int64(p.limit)/int64(leaf)*4 + int64(p.limit))
			}
			if rootOnly {
				fileSize = int64(blocks) * 32
			}
			if err = unix.Fallocate(fd, 0, 0, fileSize); err != nil {
				t.Fatal(err)
			}
			if !rootOnly {
				if _, err = disk.WriteAt(p.memory[:p.limit], 0); err != nil {
					t.Fatal(err)
				}
			}
			if err = unix.Fdatasync(fd); err != nil {
				t.Fatal(err)
			}
			// Two buffers bound overlap to one Commit and one executing block. These buffers are allocated before timing.
			var snapshots [2][]byte
			var undos [2][]byte
			if overlap {
				for i := range snapshots {
					snapshots[i] = make([]byte, int(p.limit))
					if i == 0 {
						undos[i] = g.undo
					} else {
						undos[i] = make([]byte, 0, cap(g.undo))
					}
				}
				g.undo = undos[0]
			}
			var log *idealLog
			if packed || rootOnly {
				log = &idealLog{fd: fd, leaf: leaf, offset: int64(p.limit), header: make([]byte, 48+int(p.limit)/leaf*4)}
				if rootOnly {
					log.rootOnly = true
					log.offset = 0
				}
				if !overlap && !rootOnly {
					log.buffer = make([]byte, int(p.limit))
				}
			}
			input := make([]byte, per*40)
			if _, err = f.Seek(16, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			roots := make([][32]byte, blocks)
			type job struct {
				ids  []int
				data []byte
			}
			jobs := make(chan job)
			done := make(chan idealCommitResult, 1)
			var wg sync.WaitGroup
			if overlap {
				wg.Go(func() {
					for j := range jobs {
						done <- idealCommit(tr, j.ids, j.data, true, fd, durable, log)
					}
				})
				defer func() { close(jobs); wg.Wait() }()
			}
			csvPath := os.Getenv("IDEAL_CSV")
			var csv *os.File
			if csvPath != "" {
				csv, err = os.Create(fmt.Sprintf("%s-%d.csv", csvPath, run))
				if err != nil {
					t.Fatal(err)
				}
				defer csv.Close()
				fmt.Fprintln(csv, "block,execution_ms,snapshot_ms,wait_ms,hash_ms,write_ms,fdatasync_ms,pack_ms,dirty_lines,dirty_leaves,minor_faults,root")
			}
			type row struct {
				exec, snapshot, wait time.Duration
				lines, leaves        int
				faults               int64
			}
			rows := make([]row, blocks)
			results := make([]idealCommitResult, blocks)
			var execution, snapshotTime, waitTime, hashTime, writeTime, syncTime, packTime time.Duration
			var lines, leaves, writeBytes, writeCalls int64
			accept := func(i int, r idealCommitResult) {
				if r.err != nil {
					t.Fatal(r.err)
				}
				roots[i] = r.root
				if run == 1 && r.root != reference[i] {
					t.Fatalf("root mismatch block %d", i)
				}
				hashTime += r.hash
				writeTime += r.write
				syncTime += r.sync
				packTime += r.pack
				writeBytes += int64(r.bytes)
				writeCalls += int64(r.calls)
				results[i] = r
			}
			runtime.GC()
			faultStart := idealFaults()
			wallStart := time.Now()
			for block := 0; block < blocks; block++ {
				before := idealFaults()
				if _, err = io.ReadFull(f, input); err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				res := g.run(input, block)
				d := time.Since(start)
				execution += d
				rows[block].exec = d
				if res.Status != 0 || fnv(res.Output) != golden[block] {
					t.Fatalf("block %d output mismatch status=%d", block, res.Status)
				}
				if len(g.undo) != len(res.DirtyLines)*68 {
					t.Fatal("undo count differs from dirty lines")
				}
				ids := make([]int, 0, len(res.DirtyLines))
				last := -1
				for _, line := range res.DirtyLines {
					id := int(line) * 64 / leaf
					if id != last {
						ids = append(ids, id)
						last = id
					}
				}
				rows[block].lines = len(res.DirtyLines)
				rows[block].leaves = len(ids)
				lines += int64(len(res.DirtyLines))
				leaves += int64(len(ids))
				if overlap {
					start = time.Now()
					if block > 0 {
						accept(block-1, <-done)
						undos[(block-1)%2] = undos[(block-1)%2][:0]
					}
					rows[block].wait = time.Since(start)
					waitTime += rows[block].wait
					start = time.Now()
					buf := snapshots[block%2][:len(ids)*leaf]
					if parallelSnapshot {
						pool.each(len(ids), func(i int) { id := ids[i]; copy(buf[i*leaf:(i+1)*leaf], p.memory[id*leaf:(id+1)*leaf]) })
					} else {
						for i, id := range ids {
							copy(buf[i*leaf:(i+1)*leaf], p.memory[id*leaf:(id+1)*leaf])
						}
					}

					rows[block].snapshot = time.Since(start)
					snapshotTime += rows[block].snapshot
					undos[block%2] = g.undo
					jobs <- job{ids, buf}
					g.undo = undos[(block+1)%2][:0]
				} else {
					accept(block, idealCommit(tr, ids, p.memory[:p.limit], false, fd, durable, log))
					g.undo = g.undo[:0]
				}
				rows[block].faults = idealFaults() - before
				if (block+1)%100 == 0 {
					t.Logf("run=%d blocks=%d native hashes checked", run, block+1)
				}
			}
			if overlap {
				start := time.Now()
				accept(blocks-1, <-done)
				waitTime += time.Since(start)
				undos[(blocks-1)%2] = undos[(blocks-1)%2][:0]
			}
			wall := time.Since(wallStart)
			faults := idealFaults() - faultStart
			if csv != nil {
				for i, r := range results {
					v := rows[i]
					fmt.Fprintf(csv, "%d,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%d,%d,%d,%x\n", i, float64(v.exec)/1e6, float64(v.snapshot)/1e6, float64(v.wait)/1e6, float64(r.hash)/1e6, float64(r.write)/1e6, float64(r.sync)/1e6, float64(r.pack)/1e6, v.lines, v.leaves, v.faults, r.root)
				}
			}
			t.Logf("anonymous mapping: %s", idealMappingInfo(p.memory))

			// Rebuild the entire state tree independently of dirty tracking and verify every file byte.
			full := newIdealTree(p.memory[:p.limit], leaf, pool)
			if full.nodes[1] != tr.nodes[1] {
				t.Fatal("full state root mismatch")
			}
			if packed || rootOnly {
				log.verify(t, disk, p.memory[:p.limit], roots, pool)
			} else {
				check := make([]byte, 1<<20)
				for off := 0; off < int(p.limit); off += len(check) {
					n := min(len(check), int(p.limit)-off)
					if _, err = disk.ReadAt(check[:n], int64(off)); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(check[:n], p.memory[off:off+n]) {
						t.Fatalf("file mismatch at %d", off)
					}
				}
			}
			if run == 0 {
				reference = roots
			}
			ms := func(d time.Duration) float64 { return float64(d) / 1e6 / float64(blocks) }
			t.Logf("IDEAL run=%d blocks=%d huge=%t overlap=%t fdatasync=%t leaf=%d packed_log=%t root_only=%t parallel_snapshot=%t execution_ms=%.6f snapshot_ms=%.6f wait_ms=%.6f hash_ms=%.6f write_ms=%.6f fdatasync_ms=%.6f pack_ms=%.6f commit_ms=%.6f timed_combined_msgs_s=%.0f wall_msgs_s=%.0f wall_s=%.6f minor_faults_block=%.3f dirty_lines_block=%.3f dirty_leaves_block=%.3f write_bytes_block=%.3f pwrite_calls_block=%.3f write_MB_s=%.3f final_root=%x", run, blocks, huge, overlap, durable, leaf, packed, rootOnly, parallelSnapshot, ms(execution), ms(snapshotTime), ms(waitTime), ms(hashTime), ms(writeTime), ms(syncTime), ms(packTime), ms(hashTime+writeTime+syncTime+packTime), float64(per*blocks)/(execution+hashTime+writeTime+syncTime+packTime).Seconds(), float64(per*blocks)/wall.Seconds(), wall.Seconds(), float64(faults)/float64(blocks), float64(lines)/float64(blocks), float64(leaves)/float64(blocks), float64(writeBytes)/float64(blocks), float64(writeCalls)/float64(blocks), float64(writeBytes)/writeTime.Seconds()/1e6, tr.nodes[1])
			t.Log("verified native outputs, undo/dirty counts, repeated root sequence, rebuilt full-state root, and complete file readback (repeat comparison applies to run 1)")
		})
	}
}

// Report only mapping properties, never process names or host addresses.
func idealMappingInfo(mem []byte) string {
	f, err := os.Open("/proc/self/smaps")
	if err != nil {
		return err.Error()
	}
	defer f.Close()
	address, err := strconv.ParseUint(fmt.Sprintf("%p", &mem[0])[2:], 16, 64)
	if err != nil {
		return err.Error()
	}
	scan := bufio.NewScanner(f)
	selected := false
	var values []string
	for scan.Scan() {
		line := scan.Text()
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.Contains(fields[0], "-") {
			ends := strings.Split(fields[0], "-")
			lo, e1 := strconv.ParseUint(ends[0], 16, 64)
			hi, e2 := strconv.ParseUint(ends[1], 16, 64)
			if e1 == nil && e2 == nil {
				selected = address >= lo && address < hi
			}
		}
		if selected {
			for _, key := range []string{"Size:", "Rss:", "Anonymous:", "AnonHugePages:", "THPeligible:", "VmFlags:"} {
				if strings.HasPrefix(line, key) {
					values = append(values, line)
				}
			}
		}
	}
	return strings.Join(values, "; ")
}

func TestIdealTreeDiagnostic(t *testing.T) {
	pool := newIdealPool()
	defer pool.close()
	for _, leaf := range []int{512, 1024, 4096} {
		t.Run(strconv.Itoa(leaf), func(t *testing.T) {
			mem := make([]byte, 2*1024*1024)
			tr := newIdealTree(mem, leaf, pool)
			for round := 0; round < 10; round++ {
				var ids []int
				for i := 0; i < len(mem)/leaf; i++ {
					if (i*17+round*31)%13 == 0 {
						ids = append(ids, i)
						for j := 0; j < leaf; j++ {
							mem[i*leaf+j] = byte(i + j + round)
						}
					}
				}
				snap := make([]byte, len(ids)*leaf)
				for i, id := range ids {
					copy(snap[i*leaf:], mem[id*leaf:(id+1)*leaf])
				}
				tr.update(ids, snap, true)
				full := newIdealTree(mem, leaf, pool)
				if full.nodes[1] != tr.nodes[1] {
					t.Fatalf("round %d root mismatch", round)
				}
			}
		})
	}
}

func TestIdealUndoDiagnostic(t *testing.T) {
	mem := make([]byte, 256+bitmapBytes)
	for i := 0; i < 256; i++ {
		mem[i] = byte(i)
	}
	before := bytes.Clone(mem[:256])
	g := &idealGuest{p: &Program{memory: mem, limit: 256}, undo: make([]byte, 0, 4*68)}
	g.write(30, bytes.Repeat([]byte{99}, 120))
	g.write(50, bytes.Repeat([]byte{88}, 70))
	if len(g.undo) != 3*68 {
		t.Fatalf("first-write records %d", len(g.undo)/68)
	}
	for i := 0; i < len(g.undo); i += 68 {
		line := int(binary.LittleEndian.Uint32(g.undo[i:]))
		if !bytes.Equal(g.undo[i+4:i+68], before[line*64:(line+1)*64]) {
			t.Fatal("undo does not contain original bytes")
		}
		copy(mem[line*64:(line+1)*64], g.undo[i+4:i+68])
	}
	if !bytes.Equal(mem[:256], before) {
		t.Fatal("undo restoration failed")
	}
}
