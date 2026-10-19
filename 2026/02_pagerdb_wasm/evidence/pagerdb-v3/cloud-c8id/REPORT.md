# PagerDB v3 on 16 physical cores and NVMe

PagerDB v3 improves the original Wasm workload to **1.505–1.508M wall messages/s**, about **1.56x** the v2 0.967M reference. It remains about **25% below** the RAM-only ideal-store best of 2.02M. EVM gains are larger, especially for concentrated writes. All paired native hashes and root comparisons pass.

Measured 2026-10-15 JST on one new c8id.8xlarge, Intel Xeon 6975P-C, Ubuntu 24.04, kernel 7.0.0-1014-aws, Go 1.26.0, Rust 1.99.0. CPUs 0-15 are pinned physical cores and GOMAXPROCS=16. Every DB and temporary file uses local ext4 NVMe. Host source is 2c590d4; PagerDB is a1d6d0f. Exact commands, timer definitions and diagnostic instrumentation are in [COMMANDS.md](COMMANDS.md) and [instrument.py](instrument.py).

## Wasm host

Two 600-block replays per fixture, 50,000 messages/block. Values below are run 1 / run 2. Phase times are means per block.

| Fixture | Execution ms/block | Commit-return ms/block | Wall messages/s | Minor faults/block |
|---|---:|---:|---:|---:|
| original | 24.832 / 24.759 | 7.705 / 7.845 | 1,508,357 / 1,505,499 | 2,253.1 / 2,876.7 |
| synthetic | 43.088 / 42.959 | 20.302 / 19.563 | 778,037 / 788,588 | 9,548.6 / 8,260.0 |

| Fixture | Hash + fold ms/block | Journal append ms/block | Journal write ms/block | Journal write MB/s | Journal MB/block | Checkpoint image copy ms/block |
|---|---:|---:|---:|---:|---:|---:|
| original | 6.927 / 6.936 | 5.608 / 5.695 | 2.550 / 2.554 | 4,185 / 4,179 | 10.673 | 6.598 / 6.620 |
| synthetic | 13.034 / 13.033 | 9.664 / 9.599 | 4.337 / 4.340 | 4,112 / 4,109 | 17.834 | 14.878 / 14.893 |

Wall time includes input reads, output-hash and dirty-count checks, snapshot Commit, previous-root checks, final root and journal drain. It excludes deployment, oracle preparation and the final independent Verify. Blocks/s is wall messages/s divided by 50,000. Asynchronous phase times overlap and cannot be summed. Journal MB/s is bytes divided by time inside File.Write, which writes to the OS page cache; it is not measured physical NVMe throughput.

**No automatic checkpoint occurred in any timed run.** The default interval is 1,000 commits, beyond these 600-block host and 700-block EVM runs. Measured checkpoint count and time are zero, not an estimate of checkpoint cost. Genesis checkpoint occurs during setup. The image-copy column measures preparation of the private checkpoint image, not a persisted checkpoint.

The ideal-store reference is RAM-only with 1 KiB leaves, THP and overlapping hash, with no checkpoint or journal writes. V3 uses 4 KiB leaves, retained undo, journal records and a private checkpoint image. The v2 0.967M reference uses execution-plus-Commit timing, while the v3 wall measurement also includes reads and correctness checks. These are design comparisons, not isolated changes under identical durability and timer boundaries. V3 Flush does not fsync the journal, and restart recovers the last checkpoint. This short run does not measure sustained automatic checkpoint cost or long-term journal reclamation.

## EVM workload

Two runs of 700 blocks, 50,000 read-increment-write operations per block over 2M slots, retain 32. Final Flush is timed. Current v3 roots are compared across both runs, not against the different v2 leaf format. Write and Commit times below are medians, not means.

| Distribution | v3 writes/s, run 1 / run 2 | v2 reference writes/s | v3 p50 write ms/block | v3 p50 Commit-return ms/block |
|---|---:|---:|---:|---:|
| Uniform | 2,848,025 / 3,206,418 | 1.98M | 6.114 / 5.991 | 7.752 / 7.653 |
| Sigma 1% | 15,673,243 / 16,007,304 | 3.91M | 2.669 / 2.621 | 0.368 / 0.378 |
| Sigma 0.1% | 32,409,881 / 32,779,405 | 7.91M | 1.369 / 1.368 | 0.087 / 0.075 |

Uniform is 1.44–1.62x the v2 reference; sigma 1% is 4.01–4.09x; sigma 0.1% is 4.10–4.14x. Uniform varies materially between these two runs. No cache clearing was performed. The v2 benchmark includes final Seal, while v3 includes final Flush and has no v2 sealer; the storage work differs.

## Profile: why the host remains below the ideal store

The first original replay was profiled for 20.04 seconds, with 62.92 CPU-seconds sampled across goroutines. Both profile-run replays still passed all output and root checks. Profile timings are excluded from the result table.

- SHA-256 assembly: 27.89 CPU-seconds, 44.33% of sampled CPU. Hash plus fold takes about 6.93 ms/block in unprofiled runs and normally overlaps execution.
- Memory copying: 13.15 CPU-seconds, 20.90%; memory clearing: 4.01 CPU-seconds, 6.37%.
- Snapshot allocation at `make([]byte, size)`: 3.31 CPU-seconds, about 5.52 ms/block on the caller. Snapshot worker copies add 6.65 CPU-seconds across workers. The unprofiled Commit-return mean is 7.7–7.8 ms/block, versus roughly 0.9 ms snapshot time in the best 1 KiB ideal-store run. Different leaf sizes and designs prevent treating this difference as one isolated cost.
- The disk worker uses 7.47 CPU-seconds cumulatively, including 3.49 for journal append. Its measured checkpoint-image copy takes about 6.6 ms/block; journal append takes about 5.6–5.7 ms/block, of which 2.55 ms is File.Write.

The evidence supports reducing snapshot allocation/clearing and extra copies as the first investigation. The snapshot buffer pool can need a larger buffer as dirty-leaf counts change, and sync.Pool can discard buffers at GC. This profile does not distinguish those causes or prove a particular speedup. Minor faults remain high, about 2,253–2,877/block on original and 8,260–9,549 on synthetic. CPU shares are not wall-time shares; unknown external frames include Wasm JIT execution and cannot all be assigned to one source function. See profile-top.txt, profile-cumulative.txt, profile-snapshot.txt and profile-journal.txt.

## Verification and evidence

The host compares every native output hash, validates dirty byte/leaf counts, compares every repeated root, drains the journal, and independently rebuilds the final state root with Verify. Each EVM repetition collects all 700 roots, the second checks the complete sequence, and Verify rebuilds the final root outside the timer. All six EVM runs pass. No production storage algorithm was changed; box-local counters and root checks are reproduced by instrument.py.

Raw outputs: original.txt, synthetic.txt, evm.txt, profile-run.txt. Host vet/full tests and core vet/race tests pass; pristine archive checks are recorded separately. [analyze.py](analyze.py) checks run counts, success, repeated host roots, EVM root-check markers and wall throughput arithmetic, and produces metrics.json. Private account, network, login and launch fields are excluded. Fixtures and binary profiles are excluded.

## Lifecycle

Launch was 18:15:06 JST, with boot shutdown scheduled for 21:15:24 JST. Terminate-on-shutdown and root-volume deletion were verified. No security-group change was needed. Final shutdown and independently observed termination are recorded in teardown.txt.

Shutdown was requested with `sudo shutdown -h now` at **18:28:27 JST**. AWS independently reported **terminated at 18:29:03 JST** for instance `<instance>`. The launch-to-verification interval is 0.2325 hours. At the prior run's recorded rate of $1.7741/hour, estimated compute cost is **$0.41**; this is not a billing quote and excludes EBS, public IPv4 and tax. See [teardown.txt](teardown.txt).

The original fixture SHA-256 on the box matched the original local file: `cb042fbb90f5988ad205e34b4083b6cb45a96a75f8323969dc93d47ef494f923`. Both pristine archives passed vet and full tests. The instrumented core passed vet and race tests, and the instrumented host passed vet and full tests. No instance or temporary fixture remains on the terminated box.
