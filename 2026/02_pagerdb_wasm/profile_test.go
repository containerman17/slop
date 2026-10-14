package pagerwasm

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"testing"
	"time"
)

// Opt-in CPU attribution, not a production API. Phase labels propagate to
// PagerDB parallel workers, but not to its independently created sealer.
func TestCommitProfile(t *testing.T) {
	path := os.Getenv("PAGERWASM_PROFILE")
	if path == "" {
		t.Skip("opt-in CPU profile")
	}
	f, per, blocks, hashes := fixture(t)
	startBlock := 0
	if v := os.Getenv("PAGERWASM_PROFILE_START"); v != "" {
		var err error
		startBlock, err = strconv.Atoi(v)
		if err != nil || startBlock < 0 {
			t.Fatal("invalid profile start")
		}
	}
	if blocks < startBlock+20 {
		t.Fatal("profile needs 20 blocks")
	}
	db, h := testHost(t)
	code, err := os.ReadFile("testdata/matcher.wasm")
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.Deploy(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, db)
	profile, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer profile.Close()
	traceFile, err := os.Create(path + ".trace")
	if err != nil {
		t.Fatal(err)
	}
	defer traceFile.Close()
	defer pprof.StopCPUProfile()
	defer trace.Stop()
	block := make([]byte, per*40)
	var elapsed time.Duration
	var pages int
	var before []byte
	var last Result
	for i := 0; i < startBlock+20; i++ {
		if i == startBlock {
			if err := pprof.StartCPUProfile(profile); err != nil {
				t.Fatal(err)
			}
			if err := trace.Start(traceFile); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := io.ReadFull(f, block); err != nil {
			t.Fatal(err)
		}
		if i == startBlock+19 {
			before = bytes.Clone(p.memory)
		}
		var result Result
		pprof.Do(context.Background(), pprof.Labels("stage", "execute"), func(ctx context.Context) {
			result = p.Run(ctx, Call{Input: block, BlockNumber: uint64(i), Gas: 1 << 50})
		})
		if result.Status != 0 || fnv(result.Output) != hashes[i] {
			t.Fatalf("block %d failed", i)
		}
		start := time.Now()
		pprof.Do(context.Background(), pprof.Labels("stage", "commit"), func(context.Context) { commit(t, db) })
		d := time.Since(start)
		if i < startBlock {
			if (i+1)%100 == 0 {
				t.Logf("warmup %d/%d", i+1, startBlock)
			}
			continue
		}
		elapsed += d
		pages += len(result.DirtyPages)
		t.Logf("block %d dirty_pages=%d Commit_ms=%.3f", i, len(result.DirtyPages), float64(d)/float64(time.Millisecond))
		last = result
	}
	trace.Stop()
	pprof.StopCPUProfile()
	changed, changedPages, unchangedTouched := 0, 0, 0
	var buckets [9]int
	for _, page := range last.DirtyPages {
		off := int(page) * pageSize
		n := 0
		for j := 0; j < pageSize; j++ {
			if before[off+j] != p.memory[off+j] {
				n++
			}
		}
		changed += n
		if n > 0 {
			changedPages++
		} else {
			unchangedTouched++
		}
		buckets[off/(64<<20)]++
	}
	t.Logf("20-block M3 diagnostic under background load: mean Commit_ms=%.3f dirty_pages=%.1f", elapsed.Seconds()*1000/20, float64(pages)/20)
	t.Logf("last profiled block: touched_pages=%d changed_pages=%d unchanged_touched_pages=%d changed_bytes=%d page_bytes=%d dirty_pages_by_64MiB=%v", len(last.DirtyPages), changedPages, unchangedTouched, changed, len(last.DirtyPages)*pageSize, buckets)
}
