# Results, 2026-10-14 JST

Single-process Apple M3 diagnostics under background load. Shared benchmark lock
held. Mac only. One control run per fixture; two PagerDB matcher runs per fixture.
Execution and Commit are separate; neither includes deployment or final close/drain.

| Measure, 600 blocks | Local original | Generated |
|---|---:|---:|
| Plain same-build wazero, msgs/s | 3,693,288 | 1,904,249 |
| Metering + stack control, msgs/s | 1,005,809 | 511,554 |
| Full host execution, msgs/s | 441,435 to 460,700 | 251,761 to 256,820 |
| Execution + Commit, msgs/s | 161,739 to 168,960 | 117,037 to 118,334 |
| Dirty pages per block | 16,088.4 | 34,743.4 |
| Mean Commit ms per block | 182.661 to 200.610 | 227.843 to 228.612 |
| Additional execution latency vs metering + stack control | +118% to +128% | +99% to +103% |

The last row includes the inline barrier, PagerDB Touch, page tracking, result
handling and cleanup. It is not an isolated cost for the bitmap instruction sequence.
The historical 2026-10-12 plain wazero result was 7.83M msgs/s. That used a different
module build; the current matcher has the approved ABI, fixed memory, scalar
instructions and bulk-memory lowering. Do not attribute the historical gap solely
to the barrier.

Correctness: the original fixture matched all 600 existing golden hashes in both
runs, and all 600 PagerDB roots matched between runs. The final host also replayed
all 600 original blocks during mature profiling against freshly computed native
hashes. The generated 600-block fixture matched the native matcher in both runs,
with identical root sequences. The default 20-block generated fixture, focused
regressions and all ABI checks passed in the final `go test ./...`; `go vet ./...`
passed. Optional timing/profile tests skip in the default suite.

`shape.md` compares measured workload aggregates and documents the generator's
price-distribution and temporal-locality mismatch. `commit-diagnosis.md` explains
Commit's copying, checksumming, hashing, epoch rotation and sealer backpressure.
No optimization was attempted as part of that diagnosis.

Not verified: Linux or other CPUs, cross-engine roots, fuzzing or a hostile-module
corpus, crash recovery of a live Wasm call, reopening existing program instances,
and exact wall-time boundaries for each internal Commit phase. The native oracle
shares the matcher algorithm; it is not an independent implementation. The original
precomputed golden check provides a separate fixture-level reference.
