# Ordered bitmap and undo growth: host verification

2026-10-15 JST. PagerDB dependency is
`9eac65770c3d49fffce31873269e2f1f87fbc5bd`, without a replace directive.
No host production code changed. Core production Go line count is 1,443 before
round 3 and 1,464 after; tests grow from 911 to 992 lines. Public API, journal
format, roots, three-slot ring and checkpoint behavior are unchanged.

Commit now extracts ordered leaf IDs from uint64 dirty bitmap words instead of
sorting appended IDs. The scan bounds cover the first through last dirty word;
repeated writes to a dirty leaf do not rewrite its bit or bounds. Parallel copy
stays. Undo buffers grow by at least doubling; the smaller of the last two block
sizes seeds a fresh buffer, then ring reuse keeps that capacity.

Mac diagnostics under the shared lock measure every explicit Commit wait and
undo capacity growth for BenchmarkEVM sigma 0.1%, 700 blocks, two runs. Growth
falls from 1,224 events to 52: 17 in the first unknown-size block, one each in
blocks 1 through 35, then none. Baseline write p99 is 2.210 / 1.896 ms; final
p99 is 1.220 / 1.678 ms. Ring waits occur in Commit and do not explain the write
outliers, so the ring was not enlarged. Later outliers remain unexplained; the
cloud's 7.5 ms tail attribution and cloud throughput require the coordinator's
rerun. These Mac values are diagnostics only.

The warmed allocation repeat reports 0 B/op and 0 allocs/op in all six samples,
including Write, Commit, hash and journal drain. Exact allocation regression
checks pass. One earlier final-code sample reported 5 B/op and rounded 0
allocs/op; it is preserved and disclosed in the core evidence, not attributed to
a known source. Growth, checkpoints and rotation are excluded from the warmed
zero-allocation claim.

Core implementation, diagnostic script, per-block counts/times and raw output:
https://github.com/containerman17/pagerdb/tree/9eac65770c3d49fffce31873269e2f1f87fbc5bd/evidence/v3/round3

The new tests cover bitmap ordering/clearing, word boundaries, sparse and empty
commits, doubling, one growth per known-size cold block, isolation from a large
setup block, and no growth after ring reuse. Existing deterministic-root,
Undo/Mark, Rewind/View, proof, overlap, subprocess-crash and allocation tests pass.

## Verification results

Both original and generated fixtures pass 600 blocks twice against the exact
9eac657 dependency. Every native output hash and dirty-byte/leaf count passes;
roots match between runs, and the final independent DB.Verify passes. Raw logs
are original600.txt and generated600.txt. These full fixture pairs are not race
builds. The complete host suite also passes under the race detector, including
its normal 20-block matcher pair (race.txt). Host go vet passes.

Final core hosted Linux amd64/macOS arm64 vet/race/no-cgo CI passes:
https://github.com/containerman17/pagerdb/actions/runs/38720358339

## Verification commands

All Mac test invocations that emit timing hold ~/.cache/pagerdb-bench.lock via
fcntl.flock(LOCK_EX). These rates are correctness diagnostics only.

```
go vet ./...
go test -race -count=1 -timeout=30m ./...
PAGERWASM_FIXTURE=/Users/ilia/temp-1m-clob/bench/fixture.bin go test -run '^TestMatcher$' -count=1 -v -timeout=30m
PAGERWASM_FIXTURE=/Users/ilia/.cache/cx/wasmhost.synthetic600.bin go test -run '^TestMatcher$' -count=1 -v -timeout=30m
```

No cloud machine was started. Sealer, sealed epochs/chapters, S3, Export,
ViewRoot and history beyond the RAM window remain phase 2. Sustained checkpoint
backpressure and real disk-full/power-cut behavior remain unverified here.
