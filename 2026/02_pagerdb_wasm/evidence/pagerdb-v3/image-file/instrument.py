"""Add diagnostic counters to box-local archives of the image-file PagerDB and the
host. No storage behavior changes. Usage: instrument.py PAGERDB_DIR HOST_DIR"""
from pathlib import Path
import sys
core, host = map(Path, sys.argv[1:])
def edit(path, old, new):
    s = path.read_text()
    assert s.count(old) == 1, (path, old, s.count(old))
    path.write_text(s.replace(old, new))
edit(core/'pagerdb.go', '"sync/atomic"', '"sync/atomic"\n "time"')
edit(core/'pagerdb.go', 'type DB struct {', 'type DB struct {\n diag [8]atomic.Int64')
edit(core/'pagerdb.go', '\t\tdb.tree.update(b.ids, b.data, db.pool)', '\t\tstart := time.Now()\n\t\tdb.tree.update(b.ids, b.data, db.pool)\n db.diag[0].Add(time.Since(start).Nanoseconds())')
with (core/'pagerdb.go').open('a') as f:
    f.write('''
// DiagnosticCounters returns cumulative phase nanoseconds, bytes and counts.
func (db *DB) DiagnosticCounters() (out [8]int64) { for i := range out { out[i] = db.diag[i].Load() }; return }
// DiagnosticApply returns the published image version and unapplied journal bytes.
func (db *DB) DiagnosticApply() (uint64, int64) { return db.disk.applyState() }
''')
# 1 append ns, 2 journal File.Write ns, 3 journal bytes, 4 applier batch ns, 5 batches
edit(core/'disk.go', 'func (d *diskWriter) append(b *block) error {', 'func (d *diskWriter) append(b *block) error {\n start := time.Now(); defer func() { d.db.diag[1].Add(time.Since(start).Nanoseconds()) }()')
edit(core/'disk.go', '\tn, err := d.journal.Write(record)', '\twriteStart := time.Now()\n n, err := d.journal.Write(record)\n d.db.diag[2].Add(time.Since(writeStart).Nanoseconds())\n d.db.diag[3].Add(int64(n))')
edit(core/'disk.go', '\td.hook("apply")\n', '\td.hook("apply")\n batchStart := time.Now(); defer func() { d.db.diag[4].Add(time.Since(batchStart).Nanoseconds()); d.db.diag[5].Add(1) }()\n')
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
edit(host/'matcher_test.go', '"time"', '"time"\n "runtime/pprof"\n "strings"\n "golang.org/x/sys/unix"')
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
            procKiB := func(key string) int64 {
                b, _ := os.ReadFile("/proc/self/status")
                for _, line := range strings.Split(string(b), "\\n") { if f := strings.Fields(line); len(f) == 3 && f[0] == key { v, _ := strconv.ParseInt(f[1], 10, 64); return v } }
                return -1
            }
            diskBytes := func() int64 {
                b, _ := os.ReadFile("/proc/diskstats")
                for _, line := range strings.Split(string(b), "\\n") { if f := strings.Fields(line); len(f) > 9 && f[2] == os.Getenv("PAGERDB_DISK") { v, _ := strconv.ParseInt(f[9], 10, 64); return v * 512 } }
                return -1
            }
            peakAnon, stopPeak := procKiB("RssAnon:"), make(chan struct{})
            peakDone := make(chan struct{})
            go func() { defer close(peakDone); tick := time.NewTicker(10 * time.Millisecond); defer tick.Stop(); for { select { case <-stopPeak: return; case <-tick.C: peakAnon = max(peakAnon, procKiB("RssAnon:")) } } }()
            var maxLagVersions uint64
            var maxLagBytes int64
            disk0 := diskBytes()
            wallStart := time.Now()
            for i := 0; i < blocks; i++ {''')
edit(host/'matcher_test.go', '\t\t\t\tpending = version\n', '''\t\t\t\tpending = version
                if applied, lagBytes := db.DiagnosticApply(); version-min(applied, version) > maxLagVersions || lagBytes > maxLagBytes { maxLagVersions = max(maxLagVersions, version-min(applied, version)); maxLagBytes = max(maxLagBytes, lagBytes) }
''')
edit(host/'matcher_test.go', '\t\t\tif err := db.Verify(); err != nil {', '''
            wall := time.Since(wallStart)
            disk1 := diskBytes()
            close(stopPeak); <-peakDone
            applied, _ := db.DiagnosticApply()
            endLag := db.Version() - min(applied, db.Version())
            if profile != nil { pprof.StopCPUProfile(); if err := profile.Close(); err != nil { t.Fatal(err) } }
            if err := unix.Getrusage(unix.RUSAGE_SELF, &usageAfter); err != nil { t.Fatal(err) }
            counters := db.DiagnosticCounters()
            for i := range counters { counters[i] -= countersBefore[i] }
            catchup := time.Now()
            if err := db.Checkpoint(); err != nil { t.Fatal(err) }
            scale := 1e6*float64(blocks)
            t.Logf("V3_METRICS run=%d blocks=%d execution_ms=%.6f commit_return_ms=%.6f wall_s=%.6f wall_msgs_s=%.0f wall_blocks_s=%.6f hash_ms=%.6f journal_append_ms=%.6f journal_write_ms=%.6f journal_MB_s_wall=%.3f journal_bytes_block=%.3f apply_batches=%d apply_busy_s=%.3f max_lag_versions=%d max_lag_MB=%.3f end_lag_versions=%d catchup_ms=%.1f peak_anon_MiB=%.1f disk_write_MB_s=%.1f minor_faults_block=%.6f final_root=%x", run, blocks, float64(execution.Nanoseconds())/scale, float64(commits.Nanoseconds())/scale, wall.Seconds(), float64(per*blocks)/wall.Seconds(), float64(blocks)/wall.Seconds(), float64(counters[0])/scale, float64(counters[1])/scale, float64(counters[2])/scale, float64(counters[3])/1e6/wall.Seconds(), float64(counters[3])/float64(blocks), counters[5], float64(counters[4])/1e9, maxLagVersions, float64(maxLagBytes)/1e6, endLag, float64(time.Since(catchup).Microseconds())/1000, float64(peakAnon)/1024, float64(disk1-disk0)/1e6/wall.Seconds(), float64(usageAfter.Minflt-usageBefore.Minflt)/float64(blocks), roots[blocks-1])
            if err := db.Verify(); err != nil {''')
