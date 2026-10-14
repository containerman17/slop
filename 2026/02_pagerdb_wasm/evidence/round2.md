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
