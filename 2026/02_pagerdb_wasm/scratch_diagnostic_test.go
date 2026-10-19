package pagerwasm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/containerman17/pagerdb"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// These layout constants describe the pinned diagnostic matcher, not a host ABI.
const scratchHeapBase = 1051136
const (
	scratchStack byte = 1 << iota
	scratchInput
	scratchOutput
	scratchBoundary
	scratchKeep
)

type scratchRange struct{ First, End uint32 }

func TestScratchDiagnostics(t *testing.T) {
	if os.Getenv("PAGERWASM_SCRATCH_DIAGNOSTICS") != "1" {
		t.Skip("opt-in full memory comparison")
	}
	code, err := os.ReadFile("testdata/matcher.wasm")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(code)) != "6c9ed683e0966462a6bff4d3ee61cb50162870f92bebd9a23076f90e5aa8d4a0" {
		t.Fatal("recheck matcher stack and heap layout")
	}
	f, per, blocks, golden := fixture(t)
	db, err := pagerdb.Open(t.TempDir(), pagerdb.Options{RetainVersions: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	var p *Program
	var flags []byte
	var touches []scratchRange
	var inputs, outputs []scratchRange
	var stackTop, minSP uint32
	markRange := func(off, n uint32, flag byte) {
		if n == 0 {
			return
		}
		end := off + n
		for line := off / lineSize; line <= (end-1)/lineSize; line++ {
			flags[line] |= flag
		}
		if off%lineSize != 0 {
			flags[off/lineSize] |= scratchBoundary
		}
		if end%lineSize != 0 {
			flags[(end-1)/lineSize] |= scratchBoundary
		}
	}
	// Mirror only the existing coalescing decisions. This observer never writes memory.
	observeTouch := func(off, n uint32) {
		if n == 0 {
			return
		}
		sp := uint32(p.globals[0].Get())
		minSP = min(minSP, sp)
		end := (off+n-1)/lineSize + 1
		for line := off / lineSize; line < end; {
			if p.memory[p.limit+line/8]&byte(1<<(line%8)) != 0 {
				line++
				continue
			}
			start := line
			for line < end && p.memory[p.limit+line/8]&byte(1<<(line%8)) == 0 {
				if line*lineSize < stackTop && (line+1)*lineSize > sp {
					flags[line] |= scratchStack
				}
				line++
			}
			touches = append(touches, scratchRange{start, line})
		}
	}
	ctx := experimental.WithFunctionListenerFactory(context.Background(), experimental.FunctionListenerFactoryFunc(func(def api.FunctionDefinition) experimental.FunctionListener {
		if def.ModuleName() != "env" {
			return nil
		}
		name := def.Name()
		switch name {
		case "input", "output", "touch", "touch_range":
		default:
			return nil
		}
		return experimental.FunctionListenerFunc(func(_ context.Context, _ api.Module, _ api.FunctionDefinition, args []uint64, _ experimental.StackIterator) {
			off := uint32(args[0])
			switch name {
			case "input":
				n := uint32(len(p.args.Input))
				inputs = append(inputs, scratchRange{off, off + n})
				markRange(off, n, scratchInput)
				observeTouch(off, n)
			case "output":
				n := uint32(args[1])
				outputs = append(outputs, scratchRange{off, off + n})
				markRange(off, n, scratchOutput)
			case "touch":
				observeTouch(off*lineSize, lineSize)
			case "touch_range":
				observeTouch(off, uint32(args[1]))
			}
		})
	}))
	h, err := New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close(ctx)
	p, err = h.Deploy(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	stackTop = uint32(p.globals[0].Get())
	if stackTop != 1<<20 {
		t.Fatal("unexpected stack top")
	}
	flags = make([]byte, p.limit/lineSize)
	before := make([]byte, len(p.memory))
	commit(t, db)
	series := map[string][]uint64{}
	categories := []string{"stack", "static", "input", "output", "input_output", "other_heap", "below_stack"}
	emit := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(string(b))
	}
	t.Logf("single-process M3 diagnostic: blocks=%d stack_top=%d heap_base=%d memory_bytes=%d; no throughput claim", blocks, stackTop, scratchHeapBase, len(before))
	for block := 0; block < blocks; block++ {
		data := make([]byte, per*40)
		if _, err := io.ReadFull(f, data); err != nil {
			t.Fatal(err)
		}
		clear(flags)
		touches = touches[:0]
		inputs = inputs[:0]
		outputs = outputs[:0]
		minSP = stackTop
		copy(before, p.memory)
		res := p.Run(ctx, Call{Input: data, BlockNumber: uint64(block), Gas: 1 << 50})
		if res.Status != 0 || fnv(res.Output) != golden[block] {
			t.Fatalf("block %d status/hash", block)
		}
		if len(inputs) != 1 || len(outputs) != 1 || len(touches) == 0 {
			t.Fatal("missing listener observations")
		}
		metrics := map[string]uint64{}
		for _, k := range []string{"dirty_lines", "changed_lines", "unchanged_lines", "changed_bytes", "changed_1_8", "changed_9_32", "changed_33_64", "all_leaves", "changed_leaves", "scratch_lines", "retained_lines", "retained_leaves", "io_boundary_lines", "touch_calls", "journal_records", "journal_bytes", "retained_records", "retained_journal_bytes", "retained_line_record_upper_bytes", "sp_min"} {
			metrics[k] = 0
		}
		for _, c := range categories {
			for _, suffix := range []string{"lines", "changed", "unchanged", "changed_bytes"} {
				metrics[c+"_"+suffix] = 0
			}
		}
		previousAll, previousChanged, previousKept := ^uint32(0), ^uint32(0), ^uint32(0)
		for _, line := range res.DirtyLines {
			start := int(line) * lineSize
			var changed uint64
			for j := start; j < start+lineSize; j++ {
				if before[j] != p.memory[j] {
					changed++
				}
			}
			flag := flags[line]
			category := "other_heap"
			switch {
			case flag&scratchInput != 0 && flag&scratchOutput != 0:
				category = "input_output"
			case flag&scratchInput != 0:
				category = "input"
			case flag&scratchOutput != 0:
				category = "output"
			case flag&scratchStack != 0:
				category = "stack"
			case line*lineSize < stackTop:
				category = "below_stack"
			case line*lineSize < scratchHeapBase:
				category = "static"
			}
			metrics[category+"_lines"]++
			metrics[category+"_changed_bytes"] += changed
			metrics["dirty_lines"]++
			metrics["changed_bytes"] += changed
			leaf := line / (pagerdb.LeafSize / lineSize)
			if leaf != previousAll {
				metrics["all_leaves"]++
				previousAll = leaf
			}
			if changed == 0 {
				metrics["unchanged_lines"]++
				metrics[category+"_unchanged"]++
			} else {
				metrics["changed_lines"]++
				metrics[category+"_changed"]++
				switch {
				case changed <= 8:
					metrics["changed_1_8"]++
				case changed <= 32:
					metrics["changed_9_32"]++
				default:
					metrics["changed_33_64"]++
				}
				if leaf != previousChanged {
					metrics["changed_leaves"]++
					previousChanged = leaf
				}
			}
			ioLine := flag&(scratchInput|scratchOutput) != 0
			if ioLine && flag&scratchBoundary != 0 {
				metrics["io_boundary_lines"]++
			}
			if changed == 0 || ioLine {
				metrics["scratch_lines"]++
			} else {
				flags[line] |= scratchKeep
				metrics["retained_lines"]++
				if leaf != previousKept {
					metrics["retained_leaves"]++
					previousKept = leaf
				}
			}
		}
		// A range Touch has 16 bytes of framing plus old bytes.
		// Keep original Touch boundaries in the scratch estimate.
		var observedLines uint64
		for _, span := range touches {
			metrics["touch_calls"]++
			observedLines += uint64(span.End - span.First)
			metrics["journal_records"]++
			for line := span.First; line < span.End; {
				if flags[line]&scratchKeep == 0 {
					line++
					continue
				}
				for line < span.End && flags[line]&scratchKeep != 0 {
					line++
				}
				metrics["retained_records"]++
			}
		}
		if observedLines != metrics["dirty_lines"] || metrics["below_stack_lines"] != 0 {
			t.Fatal("attribution or Touch coverage mismatch")
		}
		if metrics["changed_1_8"]+metrics["changed_9_32"]+metrics["changed_33_64"] != metrics["changed_lines"] {
			t.Fatal("histogram mismatch")
		}
		metrics["journal_bytes"] = 92 + 16*metrics["journal_records"] + lineSize*metrics["dirty_lines"]
		metrics["retained_journal_bytes"] = 92 + 16*metrics["retained_records"] + lineSize*metrics["retained_lines"]
		metrics["retained_line_record_upper_bytes"] = 92 + (16+lineSize)*metrics["retained_lines"]
		metrics["sp_min"] = uint64(minSP)
		for k, v := range metrics {
			series[k] = append(series[k], v)
		}
		emit(struct {
			Block         int
			Input, Output []scratchRange
			Metrics       map[string]uint64
		}{block, inputs, outputs, metrics})
		commit(t, db)
	}
	keys := make([]string, 0, len(series))
	for k := range series {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		values := series[k]
		var total uint64
		for _, v := range values {
			total += v
		}
		slices.Sort(values)
		t.Logf("SUMMARY %s total=%d mean=%.3f p99=%d", k, total, float64(total)/float64(blocks), values[(99*blocks+99)/100-1])
	}
}
