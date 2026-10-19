package pagerwasm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/containerman17/pagerdb"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	synthetic "github.com/containerman17/slop/2026/02_pagerdb_wasm/internal/fixture"
)

func fnv(data []byte) uint64 {
	h := uint64(14695981039346656037)
	for _, c := range data {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
func fixture(t *testing.T) (*os.File, int, int, []uint64) {
	t.Helper()
	path := os.Getenv("PAGERWASM_FIXTURE")
	if path == "" {
		blocks := 20
		if v := os.Getenv("PAGERWASM_BLOCKS"); v != "" {
			var e error
			blocks, e = strconv.Atoi(v)
			if e != nil || blocks < 1 {
				t.Fatal("invalid PAGERWASM_BLOCKS")
			}
		}
		path = filepath.Join(t.TempDir(), "fixture.bin")
		out, e := os.Create(path)
		if e != nil {
			t.Fatal(e)
		}
		e = synthetic.Write(out, blocks)
		closeErr := out.Close()
		if e != nil {
			t.Fatal(e)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skip("matcher fixture absent; set PAGERWASM_FIXTURE")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	var hdr [16]byte
	if _, err = io.ReadFull(f, hdr[:]); err != nil {
		t.Fatal(err)
	}
	per, blocks := int(binary.LittleEndian.Uint32(hdr[8:])), int(binary.LittleEndian.Uint32(hdr[12:]))
	if per != 50000 || blocks < 1 {
		t.Fatalf("unexpected fixture dimensions %d x %d", blocks, per)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(home, ".cargo/bin/cargo"), "run", "--quiet", "--locked", "--release", "--manifest-path", "testdata/matcher/Cargo.toml", "-p", "matcher-tools", "--", "hashes", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	expected, err := cmd.Output()
	if err != nil {
		t.Fatalf("native oracle: %v: %s", err, stderr.Bytes())
	}
	reader := bufio.NewReader(bytes.NewReader(expected))
	hashes := make([]uint64, blocks)
	for i := range hashes {
		var index int
		if _, err := fmt.Fscanf(reader, "%d %x\n", &index, &hashes[i]); err != nil || index != i {
			t.Fatalf("native hash line %d: %v", i, err)
		}
	}
	return f, per, blocks, hashes
}
func TestMatcher(t *testing.T) {
	f, per, blocks, golden := fixture(t)
	code, err := os.ReadFile("testdata/matcher.wasm")
	if err != nil {
		t.Fatal(err)
	}
	roots := make([][32]byte, blocks)
	for run := 0; run < 2; run++ {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			db, h := testHost(t)
			p, err := h.Deploy(context.Background(), code)
			if err != nil {
				t.Fatal(err)
			}
			commit(t, db)
			timed := &barrierTimedCall{Function: p.call}
			p.call = timed
			if _, err = f.Seek(16, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			block := make([]byte, per*40)
			var execution, commits time.Duration
			var pending uint64
			accept := func(i int, version uint64) {
				root, err := db.RootAt(version)
				if err != nil {
					t.Fatal(err)
				}
				if run == 0 {
					roots[i] = root
				} else if root != roots[i] {
					t.Fatalf("block %d root differs", i)
				}
			}
			var lines, leaves int64
			var lastTouches int
			durations := make([]time.Duration, 0, blocks)
			var before []byte
			for i := 0; i < blocks; i++ {
				if _, err = io.ReadFull(f, block); err != nil {
					t.Fatal(err)
				}
				if i == blocks-1 && os.Getenv("PAGERWASM_DIAGNOSTICS") == "1" {
					before = bytes.Clone(p.memory)
				}
				start := time.Now()
				res := p.Run(context.Background(), Call{Input: block, BlockNumber: uint64(i), Gas: 1 << 50})
				execution += time.Since(start)
				if res.Status != 0 {
					t.Fatalf("block %d status %d gas left %d lines %d", i, res.Status, res.GasLeft, len(res.DirtyLines))
				}
				if hash := fnv(res.Output); hash != golden[i] {
					t.Fatalf("block %d hash %016x want %016x", i, hash, golden[i])
				}
				if res.BytesWritten != uint64(len(res.DirtyLines))*lineSize {
					t.Fatal("byte count differs from bitmap")
				}

				lines += int64(len(res.DirtyLines))
				lastTouches = len(res.DirtyLines)
				previous := ^uint32(0)
				var countedLeaves uint64
				for _, line := range res.DirtyLines {
					leaf := line * lineSize / pagerdb.LeafSize
					if leaf != previous {
						leaves++
						countedLeaves++
						previous = leaf
					}
				}
				if countedLeaves != res.LeavesTouched {
					t.Fatal("leaf count differs from bitmap")
				}
				start = time.Now()
				version, err := db.Commit()
				if err != nil {
					t.Fatal(err)
				}
				d := time.Since(start)
				commits += d
				durations = append(durations, d)
				if i > 0 {
					accept(i-1, pending)
				}
				pending = version
				if (i+1)%100 == 0 {
					t.Logf("run %d: %d/%d native hashes and roots checked", run, i+1, blocks)
				}
			}
			accept(blocks-1, pending)
			if err := db.Flush(); err != nil {
				t.Fatal(err)
			}
			if err := db.Verify(); err != nil {
				t.Fatal(err)
			}
			if before != nil {
				changed := 0
				for j, b := range before {
					if b != p.memory[j] {
						changed++
					}
				}
				t.Logf("last block final changed bytes=%d, uncoalesced Touch log bytes upper bound=%d, logged old bytes=%d", changed, lastTouches*(16+lineSize), lastTouches*lineSize)
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			t.Logf("uncoalesced log bytes/block upper bound=%.0f (ABI coalescing reduces headers), dirty %d-byte leaves/block=%.1f, Commit p99 ms=%.3f", 92+float64(lines)*(16+lineSize)/float64(blocks), pagerdb.LeafSize, float64(leaves)/float64(blocks), float64(durations[(99*blocks+99)/100-1])/float64(time.Millisecond))
			t.Logf("bytes_written/block=%.0f, leaves_touched/block=%.1f", float64(lines)*lineSize/float64(blocks), float64(leaves)/float64(blocks))
			t.Logf("guest ms/block=%.3f, end ms/block=%.3f", timed.guest.Seconds()*1000/float64(blocks), (execution-timed.guest).Seconds()*1000/float64(blocks))
			t.Logf("correctness-run diagnostics (not a benchmark): %.0f msgs/s execution, %.0f msgs/s including snapshot Commit (excludes final drain), %.1f dirty tracking units/block, %.3f Commit ms/block", float64(per*blocks)/execution.Seconds(), float64(per*blocks)/(execution+commits).Seconds(), float64(lines)/float64(blocks), commits.Seconds()*1000/float64(blocks))
		})
	}
}
