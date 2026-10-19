# PagerDB bitmap scan and undo growth: cloud round 5

Core **9eac657**, Wasm host **7a3b5e1**. Original Wasm reaches **1.845M / 1.846M wall messages/s**, and synthetic reaches **1.089M / 1.083M**. EVM sigma 0.1% write-phase p99 improves to **1.806 / 1.823 ms/block**, compared with **7.501 / 7.437 ms** in round 4. All native hashes, repeated roots and independent final state checks pass. Both final host roots also match rounds 3 and 4.

One new c8id.8xlarge in us-west-2a, 16 physical cores pinned at CPUs 0-15, GOMAXPROCS=16, ext4 instance-store NVMe. Xeon 6975P-C, kernel 7.0.0-1014-aws, Go 1.26.0, Rust 1.99.0. Every database and temporary file uses NVMe. All user-facing times are JST. Source archives are pinned; diagnostics only modify box-local copies, with the same instrumentation used in both earlier rounds.

## Throughput comparison

Each cell gives run 1 / run 2. Host tests replay 600 blocks; EVM runs 700 blocks, count=2. Each block has 50,000 messages or read-increment-write operations.

| Workload | Round 3 | Round 4 | Round 5 |
|---|---:|---:|---:|
| Wasm original, M msgs/s | 1.508 / 1.505 | 1.782 / 1.786 | 1.845 / 1.846 |
| Wasm synthetic, M msgs/s | 0.778 / 0.789 | 1.032 / 1.040 | 1.089 / 1.083 |
| EVM uniform, M writes/s | 2.848 / 3.206 | 3.540 / 3.526 | 3.941 / 3.913 |
| EVM sigma 1%, M writes/s | 15.673 / 16.007 | 15.819 / 15.843 | 17.090 / 17.048 |
| EVM sigma 0.1%, M writes/s | 32.410 / 32.779 | 30.394 / 30.820 | 33.521 / 33.903 |

Pair-mean gains from round 4: Wasm original +3.4%, Wasm synthetic +4.8%, EVM uniform +11.2%, EVM sigma 1% +7.8%, EVM sigma 0.1% +10.1%. These are two-run comparisons, not confidence intervals. No cache clearing was performed.

## EVM per-block write tail

| Metric, sigma 0.1% | Round 3 | Round 4 | Round 5 |
|---|---:|---:|---:|
| p50 write ms/block | 1.369 / 1.368 | 1.375 / 1.368 | 1.391 / 1.391 |
| p99 write ms/block | 2.102 / 2.114 | 7.501 / 7.437 | 1.806 / 1.823 |
| p50 Commit-return ms/block | 0.087 / 0.075 | 0.057 / 0.057 | 0.051 / 0.051 |
| p99 Commit-return ms/block | 0.414 / 0.430 | 0.241 / 0.219 | 0.233 / 0.216 |

The existing benchmark reports the nearest-rank p99 across 700 block write durations, including early blocks. It is not a per-write percentile. The write phase includes random slot generation, dirty-leaf accounting and 50,000 read-increment-write operations. Commit timing is separate. The prior tail regression is absent in this pair. This measurement does not prove which internal change removed each outlier; the CPU profile below covers Wasm, not EVM.

## Host phase measurements

Means per block, run 1 / run 2. Commit-return includes any snapshot-ring, previous-hash and disk-consumption wait.

| Fixture | Execution ms/block | Commit-return ms/block | Minor faults/block |
|---|---:|---:|---:|
| original | 24.655 / 24.543 | 1.844 / 1.946 | 566.2 / 221.6 |
| synthetic | 41.840 / 41.758 | 3.248 / 3.549 | 890.5 / 481.7 |

| Fixture | Hash + fold ms/block | Journal append ms/block | Journal write ms/block | Journal write MB/s | Journal MB/block | Checkpoint-image copy ms/block |
|---|---:|---:|---:|---:|---:|---:|
| original | 7.617 / 7.612 | 4.488 / 4.476 | 2.459 / 2.460 | 4,340 / 4,339 | 10.673 | 6.515 / 6.510 |
| synthetic | 13.799 / 13.803 | 7.611 / 7.632 | 4.205 / 4.219 | 4,241 / 4,227 | 17.834 | 14.871 / 14.892 |

Wall throughput includes fixture reads, native-output checks, dirty-count checks, Commit, previous-root checks and final root/journal drain. It excludes deployment, oracle preparation and the independent final Verify. Blocks/s is wall messages/s divided by 50,000. Background phases overlap and cannot be summed. Journal append includes packing, CRC and rotation; journal write measures File.Write only. MB/s is decimal MB divided by File.Write duration, not physical device bandwidth.

Automatic checkpoint count and time are **zero** in every timed run: the default interval is 1,000 commits, longer than these workloads. Setup's genesis checkpoint is excluded. Checkpoint-image copying is not a persisted checkpoint. Flush does not fsync the journal, and restart recovers the last checkpoint. No sustained checkpoint or reclamation claim follows from these short runs.

## EVM phases, all distributions

| Distribution | p50 write ms/block | p99 write ms/block | p50 Commit-return ms/block | p99 Commit-return ms/block |
|---|---:|---:|---:|---:|
| Uniform | 6.310 / 6.313 | 7.328 / 7.268 | 6.293 / 6.396 | 7.078 / 7.203 |
| Sigma 1% | 2.605 / 2.613 | 3.247 / 3.234 | 0.271 / 0.270 | 0.506 / 0.497 |
| Sigma 0.1% | 1.391 / 1.391 | 1.806 / 1.823 | 0.051 / 0.051 | 0.233 / 0.216 |

EVM retains 32 versions and includes the final Flush in throughput. Roots are collected after the next Commit, preserving overlap, and every 700-block root sequence is compared across repetitions. Verify independently rebuilds the final root outside the timer. Host RetainVersions remains 2, unchanged from earlier rounds.

## CPU profile and next gap

Original wall throughput is below 1.9M, so the first original replay was profiled. The capture covers 16.26 seconds wall and 58.76 sampled CPU-seconds across goroutines. Both profile-run replays passed all native hashes and root comparisons. Profile timings are excluded from the main tables.

The previous sorting cost is removed. The caller-side snapshot function has only 0.02 sampled CPU-seconds, compared with 0.42 in round 4, where 0.41 was sorting. Snapshot allocation remains absent from sampled CPU time. Commit-return falls to 1.84–1.95 ms/block from 2.53–2.71 ms.

**The next measured gap is copying state into snapshots and undo records, together with the remaining host execution cost.** Snapshot-copy workers consume 5.56 CPU-seconds, versus 5.72 in round 4. PagerDB Touch consumes 1.29 CPU-seconds, about 2.15 ms/block, including 1.07 CPU-seconds attributed to appending old state bytes. Removing sorting does not remove these copies. CPU sampling does not quantify ring waits or establish how much a proposed copy change would improve wall time.

SHA-256 assembly consumes 29.87 CPU-seconds (50.83%); memmove consumes 11.75 (20.00%). The disk worker consumes 6.67 cumulatively, including 2.79 in journal append. Hash and disk work overlap execution, so their large CPU shares do not prove they limit wall throughput. External frames include JIT execution and are not fully attributed to source functions.

Original execution remains 24.54–24.66 ms/block. Mean wall throughput is about 2.9% below 1.9M and 8.6% below the 2.02M RAM-only ideal-store bound. That bound uses 1 KiB leaves and no journal/checkpoint-image work; v3 uses 4 KiB leaves, retained undo and journal writes. This is a design comparison, not equivalent persistence work.

## Verification and reproduction

Host checks retain every native output hash, dirty-byte/leaf count, every repeated root, final journal drain and independent Verify. EVM retains every 700-block root sequence, compares both runs, and verifies the final state independently. Final host roots match both earlier cloud rounds. Pristine host/core archives pass vet and full tests; the diagnostic host passes vet/full tests, and diagnostic core passes vet/race tests.

[COMMANDS.md](COMMANDS.md) records exact commands, sources and timer boundaries. [instrument.py](instrument.py) is unchanged from earlier rounds and only patches box-local copies through go.work, with no go.mod replace. [analyze.py](analyze.py) validates raw outputs and earlier roots, then produces metrics.json. Profile text is retained in the profile-*.txt files; binary profiles and fixtures are excluded. The original fixture SHA-256 matches the local original: cb042fbb90f5988ad205e34b4083b6cb45a96a75f8323969dc93d47ef494f923.

## Lifecycle

Launch: 2026-10-15 19:33:52 JST. Boot shutdown deadline: 22:34:08 JST. Terminate-on-shutdown and root-volume deletion were verified. No SSH rule change was needed, and no other instance was touched. Private launch and connection settings remain outside the repository. See teardown.txt for shutdown and AWS termination proof.

Shutdown was requested at **2026-10-15 19:43:31 JST**. AWS independently confirmed instance `<instance>` **terminated at 19:44:23 JST**. Launch-to-verification was 0.1753 hours. At the prior recorded rate of $1.7741/hour, estimated compute cost is **$0.31**, excluding EBS, public IPv4 and tax. This is an estimate, not a billing quote.
