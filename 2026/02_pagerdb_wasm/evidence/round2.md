# Inline metering and barrier granularity

Work started from `81ace55`. All timings are single-process M3 diagnostics under
uncontrolled background load, with `~/.cache/pagerdb-bench.lock` held. Each control
runs all 600 blocks and checks every output against the native matcher. Full-host
runs also compare two root sequences. PagerDB is unchanged.

## Metering

The mutable-global backend in wasm-instrument 0.4.0 calls a local Wasm function
at each metered block. The stack limiter also wraps those calls. The new helper
inliner keeps that library's block placement, unsigned underflow check, failure
sentinel, and exact gas charges, including the historical fixed helper surcharge.
It removes the helper function after replacing its calls with inline global
checks and decrements. No host import is used for instruction metering.

| Control | Original msgs/s | Generated msgs/s |
| --- | ---: | ---: |
| Plain | 3,765,458 | 1,840,335 |
| Helper metering only | 1,063,981 | 538,942 |
| Stack limit only | 3,760,745 | 1,915,007 |
| Helper metering + stack | 949,394 | 476,315 |
| Inline metering only | 3,057,870 | 1,561,365 |
| Inline metering + stack | 3,028,219 | 1,562,353 |

Raw data: `round2-original-controls.txt`, `round2-generated-controls.txt`.
Stack-only generated timing is faster than plain within run-to-run noise; it is
not evidence that stack checks improve execution. Inline gas plus stack reduces
elapsed execution time by about 69% versus helper gas plus stack on both fixtures.

The earlier 37 to 55% loss is a different denominator: SIMD/bulk builds. The
2026-10-13 devlog already measured the scalar MVP build at 3.67M plain versus
1.06M with helper metering. Bulk lowering creates byte loops with a gas check on
each iteration. This prototype uses that scalar form. Stack-only controls show
that the helper, rather than ordinary guest stack checks, is the main cost.

`TestInlineMeterMatchesHelper` compares results, traps and remaining gas for
budgets 0 through 256 and MaxInt64, with loops, branches and an indirect call.
The existing revert and gas-at-call-end tests also pass after inlining.

## Measurement definitions

Execution includes ABI copies, gas, stack checks, barriers, Touch, bitmap cleanup,
sorting and final gas charging. Commit is measured separately. Fixture generation,
native oracle, deployment, hashing outputs, final database close and sealing drain
are outside the timers. Commit p99 is the nearest-rank 99th percentile of 600
block durations. Last-block snapshots and byte comparisons are outside timers.

Log bytes are calculated from the pinned PagerDB record format: each Touch appends
32 header bytes plus old and new images of the tracked unit; each Commit appends
56 bytes. These are logical record bytes, not allocated filesystem space. They
exclude deployment and epoch/checkpoint headers. Logged old bytes alone are half
the image payload, and are the comparable figure to the earlier 70 MB observation.
Dirty 1 KiB leaves count distinct leaves covered by touches; tracking units count
individual bitmap bits. Equal-value and temporary writes still count as touches.

## Full host after inline metering, 4 KiB tracking

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Log bytes/block | Dirty 1 KiB leaves/block | Commit mean ms | Commit p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Original / 0 | 676,145 | 187,368 | 132,311,112 | 64,353.6 | 192.906 | 2,018.383 |
| Original / 1 | 743,938 | 199,021 | 132,311,112 | 64,353.6 | 184.020 | 1,979.866 |

Original tracking bits/block: 16,088.4. Block 599 has 1,687,676 final changed
bytes versus 70,602,752 logged old bytes and 141,757,088 total Touch record bytes.
Both runs pass all 600 native hashes and match every root. Raw:
`round2-original-4k.txt`.

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Log bytes/block | Dirty 1 KiB leaves/block | Commit mean ms | Commit p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Generated / 0 | 358,132 | 99,679 | 285,729,956 | 138,973.7 | 361.996 | 3,192.004 |
| Generated / 1 | 375,628 | 119,766 | 285,729,956 | 138,973.7 | 284.371 | 1,412.668 |

Generated tracking bits/block: 34,743.4. Block 599 has 2,667,872 final changed
bytes versus 187,797,504 logged old bytes and 377,062,176 Touch record bytes.
Both runs pass all 600 native hashes and match every root. Raw:
`round2-generated-4k.txt`. Commit times vary materially between repetitions.

## 1 KiB leaves

The host and store rewrite now use fixed 1 KiB units. `DirtyLeaves` reports the
leaf indices, `touch(leaf)` records 1 KiB, and `DirtyLeafGas=100` is charged per
leaf at call end. ABI writes use the same bitmap. The maximum 512 MiB guest needs
exactly 64 KiB of bitmap, so the reserved extent does not grow. Tests now include
stores crossing 64 B, 1 KiB and 4 KiB boundaries, and a full byte comparison.

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Log bytes/block | Dirty leaves/block | Commit mean ms | Commit p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Original / 0 | 1,009,596 | 418,169 | 63,804,039 | 30,675.0 | 70.044 | 1,234.086 |
| Original / 1 | 803,235 | 378,802 | 63,804,039 | 30,675.0 | 69.747 | 1,286.655 |

Both original runs pass all 600 native hashes and match every root. Block 599
has 1,687,676 final changed bytes, 32,132,096 logged old bytes and 65,268,320 Touch
record bytes. The latter is 38.7 times the final changes; even old bytes alone
are 19.0 times. This satisfies the requested greater-than-10 threshold for a
64-byte trial after completing the 1 KiB measurements. This gate uses the mature
last block, not a claimed all-block average of final changes.

The inline-gas 4 KiB stage is local commit `656cde6`. Its full default suite and
`go vet ./...` passed. No commits from this round have been pushed.

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Log bytes/block | Dirty leaves/block | Commit mean ms | Commit p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Generated / 0 | 378,499 | 207,212 | 149,208,284 | 71,734.7 | 109.198 | 1,170.259 |
| Generated / 1 | 385,309 | 216,614 | 149,208,284 | 71,734.7 | 101.060 | 653.819 |

Both generated runs pass all 600 native hashes and match every root. Block 599
has 2,667,872 final changed bytes, 93,529,088 logged old bytes and 189,980,960
Touch record bytes (71.2 times the final changes; old bytes alone 35.1 times).
The generated fixture also meets the 64-byte trial gate. Raw 1 KiB results:
`round2-original-1k.txt`, `round2-generated-1k.txt`.

## 64-byte trial

The 1 KiB stage is local commit `c0427ac`, with passing tests and vet. The trial
uses fixed 64-byte lines, `DirtyLines`, `DirtyLineGas=100`, and a reserved 1 MiB
bitmap (16 Wasm pages). Guest usable memory remains capped at 512 MiB. This is a
constant change, not a runtime option. The larger allocation changes the region
layout, so roots are compared between two runs of the same stage, not across
stages. Boundary and revert tests check the whole enlarged bitmap as well.

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Log bytes/block | Dirty 1 KiB leaves/block | Dirty lines/block | Commit mean ms | Commit p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Original / 0 | 816,600 | 602,897 | 23,861,959 | 30,675.0 | 149,136.9 | 21.703 | 118.482 |
| Original / 1 | 684,187 | 433,980 | 23,861,959 | 30,675.0 | 149,136.9 | 42.133 | 292.339 |

Both original runs pass all 600 native hashes and match every root. Leaf coverage
is identical to the 1 KiB stage. Execution alone can be slower because more first
writes call Touch; execution plus Commit improves in both repetitions. Block 599
logs 8,624,512 old bytes and 21,561,280 Touch record bytes for 1,687,676 final changed
bytes (5.1x old-image payload, 12.8x total records). No smaller-unit trial is planned.

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Log bytes/block | Dirty 1 KiB leaves/block | Dirty lines/block | Commit mean ms | Commit p99 ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Generated / 0 | 413,772 | 324,770 | 38,899,725 | 71,734.7 | 243,122.9 | 33.116 | 120.751 |
| Generated / 1 | 399,024 | 318,914 | 38,899,725 | 71,734.7 | 243,122.9 | 31.476 | 113.249 |

Both generated runs pass all 600 native hashes and match every root. Leaf coverage
again matches the 1 KiB stage. Block 599 logs 17,238,720 old bytes and 43,096,800
Touch record bytes for 2,667,872 final changed bytes (6.5x old payload, 16.2x total).
Raw 64-byte results: `round2-original-64b.txt`, `round2-generated-64b.txt`.

**Decision: keep 64-byte lines.** Execution plus Commit beats both 1 KiB repetitions
on each fixture: original 434k to 603k versus 379k to 418k msgs/s; generated 319k
to 325k versus 207k to 217k. This is not a claim of a stable production rate.
Original execution alone is sometimes slower. The reduction in Commit cost wins
on the requested combined measure. The final API reports `DirtyLines`, while the
diagnostic also reports distinct 1 KiB leaf coverage. Gas is per dirty line;
its price remains the uncalibrated prototype constant 100.

## Scope, shortcuts and limits

Changed implementation files: `instrument/src/lib.rs` (helper inlining),
`instrument/src/main.rs` (64-byte barrier and bitmap capacity), `host.go` (line
Touch, gas, results and fixed bitmap allocation). Diagnostic and correctness
changes are in `instrument/src/bin/meter-only.rs`, `diagnostic_test.go`,
`matcher_test.go`, `profile_test.go`, `metering_test.go`, `host_test.go`, and the
small WAT/Wasm test cases. README and this evidence directory describe the stages.
No PagerDB change, dependency change, runtime granularity option, cloud run,
remote push, PR, or new repository was used.

Existing prototype shortcuts remain: Rust subprocess at deploy with source tree
and Cargo required; per-Host code-hash cache; separate instrumentation passes;
reserved in-region bitmap cleared on each call; fixed maximum guest memory;
mutable guest globals restricted to scratch; bulk-memory lowering and a committed
matcher Wasm artifact; native oracle shares the matcher algorithm. The inliner
retains the old fixed helper gas surcharge to avoid changing instruction charges.
The 64-byte stage reserves a fixed 1 MiB even for small guests. It changes region
layout and dirty-unit gas costs. There is no state migration or reopen API.

No production throughput, isolated unloaded-Mac latency, cross-platform roots,
crash recovery, fuzzing, or adversarial deployment audit is established here.
The bitmap crash limitation is unchanged. Final-change amplification is sampled
at block 599; it is not measured for every block. Logical log bytes are calculated
from the pinned record format, not measured disk allocation. New CPU/trace profiles
were unnecessary for the requested controls; previous traces remain historical.

## Final validation

`go vet ./...` and `go test -count=1 -v -timeout=45m ./...` pass on the final code;
see `round2-final-vet.txt` and `round2-final-tests.txt`. The optional diagnostic
and profile tests skip by default and were run separately as recorded above.
All three measured host stages pass two complete 600-block runs on each fixture,
checking every native output and every replay root. The native oracle was also
checked directly against all 600 entries of the external original
`bench/golden.txt`; all match (`round2-golden-check.txt`). No original data or
original hash list is committed.
