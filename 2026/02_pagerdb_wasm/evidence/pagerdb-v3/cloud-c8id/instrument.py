"""Add diagnostic counters to box-local archives. No storage behavior changes."""
from pathlib import Path
import sys
core, host = map(Path, sys.argv[1:])
def edit(path, old, new):
    s=path.read_text()
    assert s.count(old)==1, (path, old, s.count(old))
    path.write_text(s.replace(old,new))
edit(core/'pagerdb.go', '"sync"', '"sync"\n "sync/atomic"\n "time"')
edit(core/'pagerdb.go', 'type DB struct {', 'type DB struct {\n diag [8]atomic.Int64')
edit(core/'pagerdb.go', '\t\tdb.tree.update(b.ids, b.data, db.pool)', '\t\tstart := time.Now()\n\t\tdb.tree.update(b.ids, b.data, db.pool)\n db.diag[0].Add(time.Since(start).Nanoseconds())')
with (core/'pagerdb.go').open('a') as f:
    f.write('\n// DiagnosticCounters returns cumulative phase nanoseconds and bytes.\nfunc (db *DB) DiagnosticCounters() (out [8]int64) { for i := range out { out[i] = db.diag[i].Load() }; return }\n')
edit(core/'disk.go', '"sync"', '"sync"\n "time"')
edit(core/'disk.go', 'func (d *diskWriter) checkpoint(v uint64, root [32]byte) error {', 'func (d *diskWriter) checkpoint(v uint64, root [32]byte) error {\n start := time.Now(); defer func() { d.db.diag[4].Add(time.Since(start).Nanoseconds()); d.db.diag[5].Add(1) }()')
edit(core/'disk.go', 'func (d *diskWriter) append(b *block) error {', 'func (d *diskWriter) append(b *block) error {\n start := time.Now(); defer func() { d.db.diag[1].Add(time.Since(start).Nanoseconds()) }()')
edit(core/'disk.go', '\tn, err := d.journal.Write(record)', '\twriteStart := time.Now()\n n, err := d.journal.Write(record)\n d.db.diag[2].Add(time.Since(writeStart).Nanoseconds())\n d.db.diag[3].Add(int64(n))')
edit(core/'disk.go', '\t\t\t\tfor i, id := range b.ids {', '\t\t\t\timageStart := time.Now()\n\t\t\t\tfor i, id := range b.ids {')
edit(core/'disk.go', '\t\t\t\tif b.checkpoint {', '\t\t\t\td.db.diag[6].Add(time.Since(imageStart).Nanoseconds())\n\t\t\t\tif b.checkpoint {')
edit(host/'matcher_test.go', '"time"', '"time"\n "runtime/pprof"\n "golang.org/x/sys/unix"')
edit(host/'matcher_test.go', '\t\t\tfor i := 0; i < blocks; i++ {', '''
            if err := db.Flush(); err != nil { t.Fatal(err) }
            countersBefore := db.DiagnosticCounters()
            var usageBefore, usageAfter unix.Rusage
            if err := unix.Getrusage(unix.RUSAGE_SELF, &usageBefore); err != nil { t.Fatal(err) }
            var profile *os.File
            if path := os.Getenv("V3_CPU_PROFILE"); path != "" && run == 0 {
                profile, err = os.Create(path); if err != nil { t.Fatal(err) }
                if err := pprof.StartCPUProfile(profile); err != nil { t.Fatal(err) }
            }
            wallStart := time.Now()
            for i := 0; i < blocks; i++ {''')
edit(host/'matcher_test.go', '\t\t\tif err := db.Verify(); err != nil {', '''
            wall := time.Since(wallStart)
            if profile != nil { pprof.StopCPUProfile(); if err := profile.Close(); err != nil { t.Fatal(err) } }
            if err := unix.Getrusage(unix.RUSAGE_SELF, &usageAfter); err != nil { t.Fatal(err) }
            counters := db.DiagnosticCounters()
            for i := range counters { counters[i] -= countersBefore[i] }
            scale := 1e6*float64(blocks)
            t.Logf("V3_METRICS run=%d blocks=%d execution_ms=%.6f commit_return_ms=%.6f wall_s=%.6f wall_msgs_s=%.0f wall_blocks_s=%.6f hash_ms=%.6f journal_append_ms=%.6f journal_write_ms=%.6f journal_MB_s=%.3f journal_bytes_block=%.3f checkpoint_ms=%.6f checkpoint_count=%d image_copy_ms=%.6f minor_faults_block=%.6f final_root=%x", run, blocks, float64(execution.Nanoseconds())/scale, float64(commits.Nanoseconds())/scale, wall.Seconds(), float64(per*blocks)/wall.Seconds(), float64(blocks)/wall.Seconds(), float64(counters[0])/scale, float64(counters[1])/scale, float64(counters[2])/scale, float64(counters[3])*1000/float64(counters[2]), float64(counters[3])/float64(blocks), float64(counters[4])/1e6, counters[5], float64(counters[6])/scale, float64(usageAfter.Minflt-usageBefore.Minflt)/float64(blocks), roots[blocks-1])
            if err := db.Verify(); err != nil {''')
edit(core/'evm_bench_test.go', 'func BenchmarkEVM(b *testing.B) {', 'var diagnosticEVMRoots = map[string][][32]byte{}\n\nfunc BenchmarkEVM(b *testing.B) {')
edit(core/'evm_bench_test.go', '\t\t\tvar dirty int', '\t\t\tvar dirty int\n var pending uint64\n roots := make([][32]byte, b.N)')
edit(core/'evm_bench_test.go', '\t\t\t\t_, e := db.Commit()', '\t\t\t\tversion, e := db.Commit()')
edit(core/'evm_bench_test.go', '\t\t\t\ttimes[block] = time.Since(commit).Seconds() * 1000', '\t\t\t\ttimes[block] = time.Since(commit).Seconds() * 1000\n if block > 0 { roots[block-1], e = db.RootAt(pending); must(b,e) }; pending = version')
edit(core/'evm_bench_test.go', '\t\t\telapsed := time.Since(start)', '\t\t\tvar rootErr error\n roots[b.N-1], rootErr = db.RootAt(pending); must(b,rootErr)\n elapsed := time.Since(start)')
edit(core/'evm_bench_test.go', '\t\t\tb.StopTimer()', '''
            b.StopTimer()
            must(b, db.Verify())
            key := fmt.Sprintf("%s/%d", name, b.N)
            if previous, ok := diagnosticEVMRoots[key]; ok {
                if !slices.Equal(previous, roots) { b.Fatal("EVM repeated roots differ") }
                b.Logf("EVM_ROOTS_CHECKED distribution=%s blocks=%d final_root=%x", name, b.N, roots[b.N-1])
            } else { diagnosticEVMRoots[key] = roots }
''')
