# Barrier cost diagnosis, round 3

2026-10-15 JST, starting at pushed `7724c6d`. Diagnostic code only. No production
host, deployed instrumentation, matcher, gas schedule or PagerDB changes.
All results are **single-process M3 diagnostics**, Apple M3, Go 1.26.0 darwin/arm64,
wazero 1.12.0, PagerDB af2c4c9. The shared `~/.cache/pagerdb-bench.lock` was held.
Other machine load was not controlled. Original fixture only: 600 blocks of
50,000 messages. Timings below are means per block, not p50.

## Result

The extra execution time is spread across the store checks, host transitions,
journaling, and an expensive dirty-line sort. It is not mainly Rust stack undo.
The full host takes 60.7 to 62.1 ms/block versus 16.2 to 16.4 ms with gas and
stack limits alone. Only 29.863 of 149,136.893 distinct dirty lines/block are
shadow-stack lines (0.0200%).

## Controls

`barrier_diagnostic_test.go` reuses deployment, `Program.Run`, metering, the
stack limiter and the production memory allocator. `instrument/src/bin/barrier-control.rs`
is a diagnostic post-pass over the production instrumented module. It does not
replace the production instrumenter.

- `metered`: gas and stack limits, without the store barrier or host write tracking.
- `stub`: every original store guard and bitmap test remains. On a miss, set the
  bitmap bit inline; remove the `touch` call. No PagerDB Touch.
- `empty`: exactly the stub body plus a Wasm-to-Go `touch` call on a miss. The host
  charges the normal HostGas and counts calls, but does not track or Touch.
- `host`: the original instrumented body and host bit/list tracking, with only
  the `DB.Touch` statement removed from the callback and ABI write path.
- `full`: the original body, real `Program.touch`, real ABI `Program.write`, real
  `Program.Run` and Commit after each block. The diagnostic ABI wrapper adds
  counters, input timing, and stack classification.

All 600 output hashes pass the native oracle in every control in both orders.
Order 1 is metered, stub, empty, host, full. Order 2 reverses it. The separate
profile also checks all 600 outputs. The previous round checked this native
oracle against the original external golden file; that comparison was not rerun.

The stub and empty controls reconstruct the dirty list by scanning the bitmap
**after** the timed guest call. This takes 1.09 to 1.11 ms/block and is excluded
from their reported Run time. It gives a sorted list, so their end cost cannot
be used to estimate production sorting. These controls bypass undo, do not
Commit and must never serve as persisted-state implementations. Their throughput
is an isolation control, not a proposed safe host. Appending a temporary local
in the diagnostic post-pass does not recompute stack-limit costs.

| Control | Guest ms, run 1 / 2 | End ms, run 1 / 2 | Run ms, run 1 / 2 | Execution msgs/s, run 1 / 2 |
| --- | ---: | ---: | ---: | ---: |
| Metered | 16.420 / 16.173 | 0.001 / 0.001 | 16.421 / 16.174 | 3,044,834 / 3,091,407 |
| Stub | 26.380 / 26.719 | 0.469 / 0.472 | 26.849 / 27.191 | 1,862,292 / 1,838,861 |
| Empty host call | 32.962 / 33.607 | 0.473 / 0.468 | 33.434 / 34.075 | 1,495,469 / 1,467,340 |
| Host, no Touch | 33.500 / 33.339 | 8.781 / 9.001 | 42.281 / 42.340 | 1,182,578 / 1,180,919 |
| Full | 52.536 / 50.526 | 9.596 / 10.195 | 62.132 / 60.721 | 804,744 / 823,436 |

Guest time includes ABI work inside `Function.Call`. End time is the remainder
of `Program.Run`, excluding the diagnostic bitmap scan. Deploy, fixture reads,
oracle construction, output hashing, classification, Commit and final database
close are outside Run. Full Commit means are 24.253 / 21.558 ms/block.
Raw evidence: `round3-controls-1.txt`, `round3-controls-2.txt`.

## Cost split

| Added work | Wall ms/block, run 1 / 2 | Calculation |
| --- | ---: | --- |
| Inline store barrier and miss bit-set | 9.868 / 10.456 | stub guest minus metered guest, less ABI input tracking difference |
| Wasm-to-Go transition, callback dispatch and HostGas | 6.582 / 6.888 | empty guest minus stub guest |
| Move bitmap/list bookkeeping into host | 0.538 / -0.268 | host guest minus empty guest; too small to resolve from these runs |
| PagerDB journaling path inside guest call | 19.036 / 17.187 | full guest minus host guest |
| Production end work | 9.596 / 10.195 | directly timed, mostly dirty-line sort |

The first row includes effective-address calculation, reserved-range/width checks,
spill/reload, cross-line handling, bitmap load/test, and the inline bit-set on a
first miss. It is not an isolated cost for just three Wasm instructions. The
removed touch calls retain their surrounding injected stack-limit code.

There are 117,937.978 guest miss callbacks/block. The empty-minus-stub difference
is approximately 55.8 / 58.4 ns per callback, including dispatch and the fixed
host gas charge. Another 31,198.915 first touches/block come from the ABI input
copy and do not cross from Wasm to Go individually. Their sum is the distinct
149,136.893 tracked lines. The ABI input callback takes 0.127 to 0.133 ms without
Touch, versus 4.332 / 1.794 ms in full mode. Subtracting that input difference
leaves 14.834 / 15.526 ms of the journaling delta in guest-triggered misses.

The full-minus-host difference is an **incremental elapsed cost of enabling
journaling and its downstream work**, not a pure timer around `DB.Touch`.
Only the full control commits and starts ongoing seals. Scheduling, mapped-log
writes, memory pressure and sealing can affect subsequent execution. Including
the extra end time gives 19.851 / 18.381 ms additional Run time for full versus
host. Do not add this latter row to the guest journaling row.

## CPU profile

The independent full run reports 61.700 ms Run, 23.292 ms Commit and 810,367
execution msgs/s. It samples 118.37 CPU seconds over 51.59 elapsed seconds because
PagerDB has worker goroutines. Profiling begins after deployment and ends after
block 600, before close/drain. `phase=guest` covers the Wasm call and host callbacks;
`phase=run` covers the surrounding end work. The labels do not attribute prior
worker goroutines to the currently executing call.

| Sampled path | Total CPU seconds | CPU ms per block |
| --- | ---: | ---: |
| Guest `DB.Touch`, inclusive | 4.83 | 8.05 |
| Within Touch: `logEntry`, inclusive | 4.56 | 7.60 |
| Within logEntry: copy old bytes into mapped log | 3.74 | 6.23 |
| End: `sort.Slice`, inclusive | 4.35 | 7.25 |
| End work, inclusive | 4.39 | 7.32 |
| Guest external/JIT code, unsymbolized | 16.23 | 27.05 |
| Background `parallelSeal.func2`, inclusive | 64.08 | 106.80 |

Nested rows overlap and must not be summed. The sealer accounts for 54.14% of
all process CPU samples, not 54.14% of Run wall time. The old-byte copy dominates
the sampled Touch path; locking and `markDirty` are not the main sampled costs.
Exact wall time of Touch alone is unverified. JIT samples do not distinguish
matcher instructions, the inline barrier, and transition stubs, hence the controls.
A faster miss callback alone does not remove the log copy or background sealing.

Raw summaries: `round3-cpu-guest.txt`, `round3-cpu-end.txt`,
`round3-cpu-logentry.txt`, `round3-cpu-all.txt`, `round3-profile.txt`.
Raw binary profile: `~/.cache/cx/wasmhost.round3.cpu` (not committed).

## Rust stack versus heap

The pinned matcher has one original mutable global, its downward
`__stack_pointer`, initially 1,048,576. Disassembly shows prologues subtracting
frame size from that global and epilogues restoring it. Static data ends at
1,051,112; allocator initialization uses heap base 1,051,136. The diagnostic
asserts the initial pointer and data boundary so a changed module cannot silently
reuse this classification.

| Distinct dirty 64-byte lines/block | Mean |
| --- | ---: |
| Shadow-stack addresses below 1,048,576 | 29.863 |
| Static data and allocator globals [1,048,576, 1,051,136) | 8.353 |
| Heap [1,051,136, 536,870,912) | 149,098.677 |
| Total | 149,136.893 |

All 17,918 first stack misses over 600 blocks overlap `[current __stack_pointer,
1,048,576)`. None is outside the active frame range. First/last block stack line
counts are 25/33; the lowest touched stack address is 1,045,440, a 3,136-byte span
below the stack top. A frame span is not a count of distinct touched bytes.

This is runtime address/frame classification of distinct lines, not static
pointer taint analysis. It does not count repeated stores to an already dirty
stack line, so it does not prove that stack stores have negligible inline-check
cost. Heap includes input, output, allocator metadata and temporary allocations,
not only persistent order-book changes. Their individual shares remain unmeasured.

Reproduce layout inspection with:
`wasm-dis testdata/matcher.wasm -o /tmp/matcher.wat`.
The allocator base appears as `i32.const 1051136`; the first mutable global is
initialized to 1048576. Local inspected copy: `~/.cache/cx/wasmhost.matcher.wat`.

## Why the earlier +31% is different

`~/temp-1m-clob/bench/RESULTS.md` records 3,988 to 5,243 microseconds p50 on the
M4 Wasmtime stitch benchmark, with 4 KiB tracking. Its source in
`bench/go/stitch/module/src/lib.rs` has an inline `mark(ptr,len)` called around
typed object/slice writes and allocator changes, with a bitmap and dirty list.
It has no per-miss host call and no PagerDB undo logging. It also has a different
allocator and bulk-memory build. The present post-pass instruments every lowered
integer store, including byte loops produced when bulk copies are lowered. The
31% is therefore neither the same engine/CPU nor the same barrier or denominator.
No historical benchmark was rerun this round.

## Proposed next changes, not implemented

The largest added elapsed component is the journaling path (about 17 to 19 ms
inside the call), with mapped-log copies and concurrent sealing visible in the
profile. Merely replacing each host callback with a guest copy targets roughly
6.6 to 6.9 ms of transitions; it does not remove that larger storage cost.

For that path, the smallest design worth testing is a bounded queue of first-write
records `(line, old[64])` in protected guest scratch. The injected miss path copies
the old bytes, records the line and sets the bit without an individual host call.
Flush once per batch and at call exit. Coalesce adjacent queued lines when their
old images are contiguous. This can reduce host crossings and record/index/sealing
work, but does not eliminate the old-byte copy volume. Its benefit, including
the extra guest copy, is unverified. Filtering final unchanged lines is a separate
idea, not part of this minimal proposal: a bounded queue already flushed earlier
records. Do not infer how many records could disappear from the final changed-byte
count, since partially changed lines still need undo.

An efficient direct flush needs a PagerDB operation that accepts a batch of old
images captured **before** guest writes, records them under one lock, marks leaves,
and captures the final bytes for Commit. The current `Touch` reads old bytes from
Map when called, so passing addresses to it after stores is incorrect. PagerDB
has not been changed. A no-API workaround could save final bytes, restore each
old image, call existing Touch, then restore final bytes, but that retains per-range
calls and adds copies. It is not the proposed fast path. Any such design also
needs an audit of concurrent sealing/history reads while an unflushed guest queue
contains the only undo image.

Keep the scratch range protected from guest stores and clear it on every exit.
Flush or restore pending undo on traps before the normal `Undo(mark)` path. ABI
writes must obey the same first-write ownership. Preserve per-line gas, original
HostGas charges and dirty-line reporting.
A bounded queue needs a host flush when full; an unbounded per-call buffer is not
necessary. A new scratch layout changes the region root layout. These correctness
requirements are part of the proposal, not implemented or validated features.

Within the existing API, coalescing consecutive not-yet-dirty lines in the ABI
`input` write into one `Touch(off,len)` per run is the smallest journaling change.
Still set/report/charge every line and never include an already touched line in
a new range. It targets the 31,199 ABI touches and the measured 1.7 to 4.2 ms
ABI journaling increment, not the remaining guest misses. This is also only a
proposal; its effect on Commit and history indexing is unmeasured.

There is also a smaller independent change with no PagerDB API dependency:
replace callback-based `sort.Slice` with stdlib `slices.Sort` for `[]uint32`, then
measure. It preserves the sorted result contract and targets the 9 to 10 ms end
cost with a one-line algorithm substitution. The gain is unmeasured; the profile
does not justify claiming that the entire sort cost disappears. This is the
lowest-risk first experiment. Neither this substitution nor the batched design
is implemented in this round.

## Reproduction and checks

Run from `~/slop/2026/02_pagerdb_wasm` on the Mac:

```sh
until mkdir ~/.cache/pagerdb-bench.lock 2>/dev/null; do sleep 20; done
export PAGERWASM_FIXTURE="$HOME/temp-1m-clob/bench/fixture.bin"
PAGERWASM_BARRIER_DIAGNOSTICS=1 go test -run '^TestBarrierDiagnostics$' -count=1 -v
PAGERWASM_BARRIER_DIAGNOSTICS=1 PAGERWASM_BARRIER_MODE=full,host,empty,stub,metered \
  go test -run '^TestBarrierDiagnostics$' -count=1 -v
PAGERWASM_BARRIER_DIAGNOSTICS=1 PAGERWASM_BARRIER_MODE=full \
  PAGERWASM_BARRIER_PROFILE="$HOME/.cache/cx/wasmhost.round3.cpu" \
  go test -run '^TestBarrierDiagnostics$' -count=1 -v
go tool pprof -top -cum -tagfocus=phase=guest "$HOME/.cache/cx/wasmhost.round3.cpu"
go tool pprof -top -cum -tagfocus=phase=run "$HOME/.cache/cx/wasmhost.round3.cpu"
go tool pprof -list=logEntry -tagfocus=phase=guest "$HOME/.cache/cx/wasmhost.round3.cpu"
go test -run '^TestMatcher$' -count=1 -v -timeout=15m
unset PAGERWASM_FIXTURE
go vet ./...
go test ./...
rmdir ~/.cache/pagerdb-bench.lock
```

`go vet ./...` and `go test ./...` pass. Default tests include two generated
20-block matcher replays and the existing revert, gas, rejection and barrier
checks. The diagnostic test skips unless explicitly enabled. Full original
600-block two-run root/output verification also passes and is recorded separately in
`round3-original-correctness.txt`. Its normal production-host runs report
806,122 / 813,770 execution msgs/s, Commit mean 19.896 / 21.650 ms and p99
116.434 / 115.085 ms. These checks confirm the diagnostic full control is in
the same range, without replacing the controlled comparisons above.

The first layout sanity check incorrectly assumed the initialized data ended at
the aligned heap base. It failed before measurement (actual data end 1051112).
The diagnostic now checks that actual boundary and uses the allocator constant,
which also leaves room for BSS. The first control matrix predates the static/heap
split and active-frame counters; the second and profiled matrices include them.

No generated 600-block performance rerun, alternate engine, byte-copy elimination,
batched undo implementation, new PagerDB API, power-loss experiment, or production
optimization was attempted. The only new code is opt-in diagnostic code. Raw
fixture data, per-block output hashes, binary profiles and compiled artifacts are
not committed.
