# PagerDB v3 reusable buffers: cloud repeat

Core **589861c**, Wasm host **44dbfd6**. Compared with round 3 (core a1d6d0f, host 2c590d4), mean original throughput improves **18.4%** and synthetic improves **32.3%**. Original remains below the requested 1.9M threshold, so one replay was CPU-profiled. All repeated roots and native output hashes pass, and both final host roots match round 3.

One new c8id.8xlarge, 16 physical cores pinned with taskset 0-15 and GOMAXPROCS=16, local ext4 NVMe. Same Go 1.26.0, Rust 1.99.0, Xeon 6975P-C and kernel 7.0.0-1014-aws as round 3. All times are JST. Source archives and the original fixture were streamed directly to the box; the synthetic fixture was generated there. No production source was changed. The round 3 diagnostic script is unchanged.

## Comparison with round 3

Paired throughput values are run 1 / run 2. Percent change uses the arithmetic mean of each pair.

| Workload | Round 3 throughput | This run throughput | Mean change |
|---|---:|---:|---:|
| Wasm original, M msgs/s | 1.508 / 1.505 | 1.782 / 1.786 | +18.4% |
| Wasm synthetic, M msgs/s | 0.778 / 0.789 | 1.032 / 1.040 | +32.3% |
| EVM uniform, M writes/s | 2.848 / 3.206 | 3.540 / 3.526 | +16.7% |
| EVM sigma 1%, M writes/s | 15.673 / 16.007 | 15.819 / 15.843 | -0.1% |
| EVM sigma 0.1%, M writes/s | 32.410 / 32.779 | 30.394 / 30.820 | -6.1% |

EVM sigma 1% is effectively unchanged. Sigma 0.1% regresses by about 6.1%, despite lower median Commit-return time. Its p99 write phase rises to 7.50 / 7.44 ms from 2.10 / 2.11 ms. This pair does not identify the cause of those tails; the host CPU profile is not an EVM profile. No cache clearing or tuning was performed between runs.

## Wasm phase measurements

Each fixture has 600 blocks of 50,000 messages, replayed twice. Times are means per block except the explicit p99 output in raw logs.

| Fixture | Execution ms/block | Commit-return ms/block | Round 3 Commit-return ms | Minor faults/block | Round 3 faults/block |
|---|---:|---:|---:|---:|---:|
| original | 24.923 / 24.677 | 2.529 / 2.708 | 7.705 / 7.845 | 505.9 / 339.4 | 2,253.1 / 2,876.7 |
| synthetic | 42.532 / 42.069 | 5.083 / 5.126 | 20.302 / 19.563 | 881.7 / 274.2 | 9,548.6 / 8,260.0 |

| Fixture | Hash + fold ms/block | Journal append ms/block | Journal write ms/block | Journal write MB/s | Journal MB/block | Checkpoint-image copy ms/block |
|---|---:|---:|---:|---:|---:|---:|
| original | 7.626 / 7.603 | 4.527 / 4.519 | 2.492 / 2.492 | 4,283 / 4,282 | 10.673 | 6.585 / 6.590 |
| synthetic | 13.833 / 13.843 | 7.654 / 7.585 | 4.239 / 4.248 | 4,207 / 4,198 | 17.834 | 14.982 / 15.002 |

Wall throughput includes fixture reads, output-hash and dirty-count checks, snapshot Commit, previous-root checks, and final root/journal drain. It excludes deployment, native-oracle preparation and final independent Verify. Commit-return includes any ring-slot, previous-hash or disk-consumption waits. Background phase times overlap; do not add them to execution time. MB/s uses decimal MB and the duration inside File.Write, not physical device bandwidth. Journal append additionally includes packing, CRC and rotation.

**Automatic checkpoint count is zero, and measured checkpoint time is zero**, for all timed runs. Default CheckpointInterval=1000 exceeds 600 host and 700 EVM blocks. Setup's genesis checkpoint is excluded. Checkpoint-image copying is measured separately, but it is not a persisted checkpoint. Flush drains writes without fsyncing the journal. Recovery still returns the last checkpoint. These numbers do not measure sustained automatic checkpoint cost.

## EVM phases

700 blocks x 50,000 read-increment-write operations, 2M slots, retain 32, count=2. Final Flush is timed; final independent Verify is outside the timer. Every root is collected without serializing the current hash with execution, and the second run compares the entire 700-root sequence.

| Distribution | p50 write ms/block | p50 Commit-return ms/block | p99 write ms/block | p99 Commit-return ms/block |
|---|---:|---:|---:|---:|
| Uniform | 6.352 / 6.376 | 7.640 / 7.638 | 12.740 / 12.800 | 8.481 / 8.600 |
| Sigma 1% | 2.622 / 2.599 | 0.351 / 0.348 | 8.159 / 8.098 | 0.560 / 0.576 |
| Sigma 0.1% | 1.375 / 1.368 | 0.057 / 0.057 | 7.501 / 7.437 | 0.241 / 0.219 |

## Profile and next gap

The profile covers one original replay, 16.88 seconds wall and 59.47 sampled CPU-seconds. Both profile-run replays still pass all native hashes and repeated roots. Profile timings are excluded from the main tables.

The earlier snapshot-allocation hotspot is absent in this profile: the allocation line has no sampled CPU time, versus 3.31 CPU-seconds in round 3. The caller-side snapshot function falls from 3.78 to 0.42 CPU-seconds. Minor faults fall sharply but do not disappear, because the measurement includes startup growth and host work.

**The next measured gap is the remaining snapshot sort and copy on the Commit path.** Sorting dirty IDs consumes 0.41 CPU-seconds, about 0.68 ms/block on the caller. Snapshot copying consumes 5.72 CPU-seconds across workers. Commit-return still takes 2.53–2.71 ms/block. These samples do not measure ring waits, so they cannot attribute every millisecond of Commit to copying. Avoiding the sort or reducing snapshot-copy work is the next focused experiment, not a proven speedup.

Execution is essentially unchanged at 24.7–24.9 ms/block. PagerDB Touch consumes 1.30 CPU-seconds, about 2.17 ms/block; 1.08 CPU-seconds is attributed to appending old state bytes. This is another measurable source of overhead versus the minimal undo callback in the ideal-store bound.

SHA-256 assembly consumes 29.80 CPU-seconds (50.11%); memory copying consumes 11.88 (19.98%). The disk worker consumes 6.68 cumulatively, including 2.78 in journal append. Hash wall time is about 7.6 ms/block and image-copy time is about 6.6 ms/block, but both overlap execution. Their CPU shares do not prove they determine the wall critical path. CPU sampling also cannot distinguish all JIT execution in external frames.

The original mean remains about 11.7% below the 2.02M RAM-only ideal-store reference. That reference uses 1 KiB leaves and no journal or checkpoint-image work; this store uses 4 KiB leaves and retains/writes undo. The reference is a design bound, not an equivalent persistence workload.

## Verification and reproduction

Both host fixtures pass every native output hash, dirty-byte/leaf check, repeated root and final independent Verify. Final host roots match round 3. All six EVM runs pass, including complete repeat-root comparisons and Verify. Pristine host/core archives pass vet and full tests; instrumented host passes vet/full tests, and instrumented core passes vet/race tests. The original fixture checksum matches the local original. No fixture or binary profile is committed.

[COMMANDS.md](COMMANDS.md) records exact commands and timer boundaries. [instrument.py](instrument.py) reproduces all box-local diagnostics without a go.mod replace. [analyze.py](analyze.py) validates raw outputs and cross-round host roots and produces metrics.json. CPU text is in profile-top.txt, profile-cumulative.txt, profile-snapshot.txt, profile-journal.txt and profile-touch.txt. Launch requests, responses and connection data remain outside the repository.

## Lifecycle

Launch: 2026-10-15 18:57:22 JST. Boot shutdown deadline: 21:57:39 JST. Terminate-on-shutdown and root deletion were verified before running. No SSH rule changes were needed, and no other instance was accessed. See teardown.txt for shutdown and AWS termination proof.

Shutdown was requested at **2026-10-15 19:06:17 JST**. AWS independently confirmed instance `i-0d1ae22bda0b0b5c1` **terminated at 19:08:32 JST**. Launch-to-verification was 0.1863 hours. At the prior recorded rate of $1.7741/hour, estimated compute cost is **$0.33**, excluding EBS, public IPv4 and tax. This is an estimate, not a billing quote.
