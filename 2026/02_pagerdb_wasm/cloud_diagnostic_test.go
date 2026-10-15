package pagerwasm

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/containerman17/pagerdb"
	"golang.org/x/sys/unix"
)

// TestCloudDiagnostics measures successful calls only. It does not validate
// recovery or undo safety of source changes made in separate diagnostic copies.
func TestCloudDiagnostics(t *testing.T) {
	if os.Getenv("PAGERWASM_CLOUD") != "1" {
		t.Skip("opt-in cloud diagnostic")
	}
	f, per, blocks, golden := fixture(t)
	code, err := os.ReadFile("testdata/matcher.wasm")
	if err != nil {
		t.Fatal(err)
	}
	var out io.Writer = io.Discard
	if path := os.Getenv("PAGERWASM_CLOUD_CSV"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		out = file
	}
	fmt.Fprintln(out, "run,block,execution_ns,commit_ns,execution_minor_faults,execution_major_faults,dirty_lines,dirty_leaves,journal_bytes")
	roots := make([][32]byte, blocks)
	for run := 0; run < 2; run++ {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			opts := pagerdb.Options{RetainVersions: 2}
			if os.Getenv("PAGERWASM_CLOUD_NO_SEAL") == "1" {
				opts.EpochVersions = 1000000
				opts.EpochBytes = 1 << 40
			}
			db, err := pagerdb.Open(t.TempDir(), opts)
			if err != nil {
				t.Fatal(err)
			}
			h, err := New(context.Background(), db)
			if err != nil {
				db.Close()
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
			p, err := h.Deploy(context.Background(), code)
			if err != nil {
				t.Fatal(err)
			}
			commit(t, db)
			timed := &barrierTimedCall{Function: p.call}
			p.call = timed
			if _, err := f.Seek(16, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			// Read the owner's current log cursor outside timers. This is specific
			// to the pinned PagerDB version and fails loudly if its layout changes.
			tail := func() int64 {
				return reflect.ValueOf(db).Elem().FieldByName("cur").Elem().FieldByName("tail").Int()
			}
			epoch := func() uint64 {
				return reflect.ValueOf(db).Elem().FieldByName("cur").Elem().FieldByName("epoch").Uint()
			}
			startEpoch := epoch()
			block := make([]byte, per*40)
			var execution, commits time.Duration
			var minor, major, lines, leaves, journal int64
			durations := make([]time.Duration, 0, blocks)
			for i := 0; i < blocks; i++ {
				if _, err := io.ReadFull(f, block); err != nil {
					t.Fatal(err)
				}
				beforeTail := tail()
				var before, after unix.Rusage
				if err := unix.Getrusage(unix.RUSAGE_SELF, &before); err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				res := p.Run(context.Background(), Call{Input: block, BlockNumber: uint64(i), Gas: 1 << 50})
				execDuration := time.Since(start)
				if err := unix.Getrusage(unix.RUSAGE_SELF, &after); err != nil {
					t.Fatal(err)
				}
				// Run never rotates. Include the fixed 56-byte Commit record;
				// exclude epoch checkpoints and sealed-file compression.
				logBytes := tail() - beforeTail + 56
				if res.Status != 0 || fnv(res.Output) != golden[i] {
					t.Fatalf("block %d status=%d gas_left=%d hash=%016x want=%016x", i, res.Status, res.GasLeft, fnv(res.Output), golden[i])
				}
				if res.BytesWritten != uint64(len(res.DirtyLines))*lineSize {
					t.Fatal("byte count differs from bitmap")
				}
				var counted uint64
				previous := ^uint32(0)
				for _, line := range res.DirtyLines {
					for leaf := line * lineSize / pagerdb.LeafSize; leaf <= ((line+1)*lineSize-1)/pagerdb.LeafSize; leaf++ {
						if leaf != previous {
							counted++
							previous = leaf
						}
					}
				}
				if counted != res.LeavesTouched {
					t.Fatal("leaf count differs from bitmap")
				}
				start = time.Now()
				root := commit(t, db)
				commitDuration := time.Since(start)
				if run == 0 {
					roots[i] = root
				} else if root != roots[i] {
					t.Fatalf("block %d root differs", i)
				}
				execution += execDuration
				commits += commitDuration
				durations = append(durations, commitDuration)
				minor += after.Minflt - before.Minflt
				major += after.Majflt - before.Majflt
				lines += int64(len(res.DirtyLines))
				leaves += int64(counted)
				journal += logBytes
				fmt.Fprintf(out, "%d,%d,%d,%d,%d,%d,%d,%d,%d\n", run, i, execDuration, commitDuration, after.Minflt-before.Minflt, after.Majflt-before.Majflt, len(res.DirtyLines), counted, logBytes)
				if (i+1)%100 == 0 {
					t.Logf("run %d: %d/%d native hashes, roots and counts checked", run, i+1, blocks)
				}
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			t.Logf("execution_msgs_s=%.0f combined_msgs_s=%.0f execution_ms=%.3f commit_ms=%.3f commit_p99_ms=%.3f end_ms=%.3f", float64(per*blocks)/execution.Seconds(), float64(per*blocks)/(execution+commits).Seconds(), execution.Seconds()*1000/float64(blocks), commits.Seconds()*1000/float64(blocks), float64(durations[(99*blocks+99)/100-1])/float64(time.Millisecond), (execution-timed.guest).Seconds()*1000/float64(blocks))
			t.Logf("per_block: dirty_lines=%.1f dirty_leaves=%.1f journal_bytes=%.1f execution_minor_faults=%.1f execution_major_faults=%.1f", float64(lines)/float64(blocks), float64(leaves)/float64(blocks), float64(journal)/float64(blocks), float64(minor)/float64(blocks), float64(major)/float64(blocks))
			t.Logf("epoch_start=%d epoch_end=%d", startEpoch, epoch())
			if os.Getenv("PAGERWASM_CLOUD_NO_SEAL") == "1" && epoch() != startEpoch {
				t.Fatal("epoch rotated in no-seal diagnostic")
			}
		})
	}
}
