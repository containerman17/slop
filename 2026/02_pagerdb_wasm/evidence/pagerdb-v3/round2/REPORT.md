# Reusable commit buffers: host verification

2026-10-15 JST. Dependency bumped to PagerDB v3
`589861cc4ada376fdca63f26b05f50ec77b2b55e`, with no replace directive.
No host production code changed. The preceding cloud evidence at slop 637aa79
remains the baseline; a new cloud throughput result is not claimed here.

The core now uses three reusable snapshot slots instead of sync.Pool. Buffers
grow only when needed, are never cleared on reuse, and remain available across
GC. Parallel snapshot copy remains. Commit metadata, completion objects, undo,
dirty IDs, hash scratch space and journal assembly storage are also reused.
A full three-slot ring waits for its oldest disk consumer, including any pending
checkpoint, before reusing bytes. This is bounded backpressure rather than the
initial v3 unbounded queue. Public signatures and disk formats are unchanged.

Core BenchmarkCommitReusable, under the shared lock on macOS arm64, reports
0 B/op and 0 allocs/op in all six 1000-block runs: three at 8 dirty leaves and
three at 512 dirty leaves. It includes write, Commit, hashing and journal drain
following warmup, without automatic checkpoints or epoch rotation. Growth,
checkpoint and rotation paths may allocate. The core adds GC/high-water tests,
zero-allocation regression checks, a full-ring stall test and rewind after reuse.
Core report and raw allocation output:
https://github.com/containerman17/pagerdb/blob/589861cc4ada376fdca63f26b05f50ec77b2b55e/evidence/v3/round2/REPORT.md

Both original and generated Wasm fixtures pass all 600 blocks twice against this
exact dependency. Every native output hash and byte/leaf count passes; all roots
match between the two runs, and final DB.Verify passes. Raw output is adjacent.
The complete host suite also passes under the race detector, including its normal
20-block matcher pair; go vet passes. The full 600-block runs are not race builds.
All Mac invocations that print timings held ~/.cache/pagerdb-bench.lock. Their
rates are correctness diagnostics only, not a performance claim.

Reproduce under that lock:

```
go vet ./...
go test -race -count=1 -timeout=30m ./...
PAGERWASM_FIXTURE=/Users/ilia/temp-1m-clob/bench/fixture.bin go test -run '^TestMatcher$' -count=1 -v -timeout=30m
PAGERWASM_FIXTURE=/Users/ilia/.cache/cx/wasmhost.synthetic600.bin go test -run '^TestMatcher$' -count=1 -v -timeout=30m
```

Core vet, unit and race checks pass locally on macOS arm64 and Linux arm64.
Hosted vet/race/no-cgo CI passes on Linux amd64 and macOS arm64:
https://github.com/containerman17/pagerdb/actions/runs/38715461474

No cloud machine was started. Cloud throughput and sustained checkpoint behavior
with ring backpressure remain for the coordinator's next run. The older cloud
instrumentation script may need new source anchors: completion channels are now
reusable WaitGroups and snapshot/journal allocation expressions changed.
