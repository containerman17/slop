# Anonymous store bounds on c8id.8xlarge

Only the tested RAM-only pipeline crosses 2M, with no checkpoint: the 1 KiB repeat reaches **2,016,162 / 2,016,780 wall msgs/s** (earlier pair: 2,024,107 / 2,022,020). The packed log reaches **1,848,611 / 1,865,994** with fdatasync per block.

2026-10-15 JST. Diagnostic only, on the same 16 physical cores and local NVMe as round 1. Original fixture, 600 blocks of 50,000 messages, two replays per configuration. Production code is unchanged.

## What is measured

The production Rust instrumenter, compiled matcher, ABI gas rules, bitmap and 64-byte first-write barrier remain active. Anonymous private memory replaces the mapped PagerDB region. The callback appends a uint32 line index and the original 64 bytes to a preallocated Go undo buffer. No PagerDB instance is created.

Commit hashes dirty leaves with SHA-256 using 16 persistent workers, then updates a complete binary Merkle tree. Small upper levels run serially. Leaves hash their bytes directly; parents hash the concatenation of two 32-byte child digests. This is a full-state root, with a different format from PagerDB. Contiguous dirty leaves are grouped into pwrite calls against one preallocated NVMe file. The fdatasync variant flushes this file after each block. The undo buffer is reused after Commit.

Overlap uses two preallocated snapshot buffers and two undo buffers. After executing a block, the host waits for the previous Commit, copies the current dirty leaves, and submits one Commit job. That job hashes and writes the immutable snapshot while the next block executes. At most one Commit overlaps one execution. The final Commit is drained before stopping the wall timer.

Execution includes guest work, input copy, undo capture, bitmap scan, gas finalization and global checks. Hash time includes leaf hashing and Merkle folding. Write time includes pwrite calls; fdatasync is separate. Snapshot and wait times are separate. Wall throughput includes fixture reads, native output-hash checks, dirty-leaf collection, snapshots, waits and final drain. It excludes setup, the native oracle, initial tree/file creation and final verification. Serial timed throughput uses execution plus Commit, to match round 1; overlap conclusions use wall throughput. Minor faults are process-wide during the wall interval, including runtime allocations and snapshots.

Initial file creation and one fdatasync occur before timing in every variant. Large undo/snapshot buffers are preallocated; ordinary per-block metadata and output allocations remain in the measured path. There is no cache dropping or writeback tuning. MADV_NOHUGEPAGE explicitly disables huge pages for the control mapping; MADV_HUGEPAGE requests them for the other mapping. Mapping properties in the logs show what Linux actually allocated.

This is a performance bound, not a database implementation. It has no durable undo log, atomic root publication, a version API or retention policy, recovery protocol or crash tests. Per-block fdatasync measures data flushing, not recoverable transactions. File readback verifies logical contents through the page cache, not survival of power loss.

## Verification

Final `go vet ./...` and `go test ./...` passed on the box with exit status 0 ([latest vet](ideal-vet-leaf-sweep.txt), [tests](ideal-tests-final.txt), [leaf race tests](ideal-leaf-tests.txt)). The complete matrix contains **25 measured entries, 50 full replays and 30,000 checked blocks**. `analyze.py` checks every CSV root and every printed phase mean; every entry shares all 600 roots with other entries of the same leaf size (512 B, 1 KiB or 4 KiB). [METRICS.md](METRICS.md) records the sequence hashes.

Each replay checks every block output against the native matcher and checks undo-record counts against dirty-line counts. Replay 2 compares every root with replay 1. After each replay, the test rebuilds the full tree from all guest memory, independently of dirty tracking, and compares its root, then compares every byte of the in-place state file with memory. Packed logs are replayed; the root-only variant verifies its root records and makes no claim to persist state bytes. Overlap smoke tests also run under the Go race detector. This checks Go accesses; it does not instrument JIT guest stores.

## Main results and comparison

Original fixture, two replays. New rows below use THP and 1 KiB hash leaves. Times are means per block. The M3 rows had uncontrolled background load; the old rows use execution-plus-Commit timers, while pipeline rows use full wall time.

| Design | Execution ms | Commit ms | Combined Mmsgs/s | Timing |
| --- | ---: | ---: | ---: | --- |
| M3 round 5, PagerDB | 23.621 / 23.345 | 24.624 / 26.175 | 1.036 / 1.010 | Execution + Commit |
| c8id round 1, PagerDB | 39.996 / 40.021 | 11.687 / 11.666 | 0.967 / 0.967 | Execution + Commit |
| Packed log, serial | 23.899 / 23.936 | 15.246 / 15.239 | 1.243 / 1.239 | Wall |
| Packed log, serial + fdatasync | 24.056 / 24.159 | 20.274 / 20.371 | 1.101 / 1.094 | Wall |
| Packed log, overlap + serial snapshot | 24.208 / 24.129 | 12.191 / 12.203 | 1.731 / 1.730 | Wall |
| Packed log, overlap + parallel snapshot | 24.742 / 24.590 | 12.035 / 12.032 | 1.863 / 1.869 | Wall |
| Packed log, overlap + parallel snapshot + fdatasync | 24.902 / 24.610 | 17.072 / 17.061 | 1.849 / 1.866 | Wall |
| RAM-only roots, serial | 22.044 / 22.065 | 5.086 / 5.076 | 1.774 / 1.773 | Wall |
| RAM-only roots, overlap + serial snapshot | 22.576 / 22.426 | 5.259 / 5.261 | 1.872 / 1.882 | Wall |
| RAM-only roots, overlap + parallel snapshot | 22.748 / 22.672 | 5.137 / 5.147 | 2.024 / 2.022 | Wall |

Old rows: [round 5](../../round5.md) and [cloud round 1](../REPORT.md). The complete matrix, including no-THP controls, 4 KiB leaves, serial timed throughput, faults and write throughput, is in [METRICS.md](METRICS.md). Raw paired CSVs retain every block root and phase time.

| Pipeline | Snapshot ms | Hash + fold ms | Write ms | fdatasync ms | Minor faults/block |
| --- | ---: | ---: | ---: | ---: | ---: |
| Packed log, overlap + serial snapshot | 3.574 / 3.601 | 5.366 / 5.353 | 6.762 / 6.784 | 0.000 / 0.000 | 834.195 / 614.552 |
| Packed log, overlap + parallel snapshot | 1.009 / 0.997 | 5.243 / 5.231 | 6.727 / 6.736 | 0.000 / 0.000 | 833.832 / 624.528 |
| Packed log, overlap + parallel snapshot + fdatasync | 1.025 / 1.005 | 5.263 / 5.246 | 6.717 / 6.723 | 5.028 / 5.028 | 833.978 / 615.007 |
| RAM-only roots, overlap + serial snapshot | 3.063 / 3.032 | 5.249 / 5.252 | 0.009 / 0.009 | 0.000 / 0.000 | 834.245 / 821.933 |
| RAM-only roots, overlap + parallel snapshot | 0.896 / 0.897 | 5.128 / 5.138 | 0.009 / 0.009 | 0.000 / 0.000 | 826.002 / 764.185 |

## Final best-config leaf sweep

Requested after the first final test suite, 2026-10-15 16:05 JST. All rows use RAM-only root writes, THP, overlapped Commit, 16 hash workers and parallel snapshots. Undo remains 64 bytes and state gas still counts 1 KiB leaves. Each row is two fresh 600-block replays; the 1 KiB row repeats the earlier baseline. Only the accepted leaf-size validation and tree unit tests changed for this sweep.

| Hash leaf | Execution ms | Snapshot ms | Hash + fold ms | Commit ms | Wall msgs/s | Dirty leaves/block |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 512 B | 23.160 / 22.600 | 0.743 / 0.751 | 5.269 / 5.270 | 5.277 / 5.279 | 2,001,889 / 2,039,594 | 43,077.890 / 43,077.890 |
| 1 KiB, repeat | 22.838 / 22.813 | 0.903 / 0.906 | 5.165 / 5.161 | 5.174 / 5.170 | 2,016,162 / 2,016,780 | 30,674.992 / 30,674.992 |
| 4 KiB | 23.017 / 22.906 | 1.442 / 1.432 | 7.289 / 7.305 | 7.299 / 7.315 | 1,966,375 / 1,976,277 | 16,088.407 / 16,088.407 |

The 512-byte leaves reduce snapshot bytes and copy time, but do not show a consistent throughput improvement over 1 KiB: one replay is slower and one is faster. The 1 KiB repeat remains just above 2M; across its four replays the rate is 2.016M to 2.024M. The 4 KiB pipeline falls below 2M and spends more time copying and hashing. These results favor keeping 1 KiB for the next design experiment; they do not establish a large margin over the target.

All three pairs pass native output hashes, undo counts, repeated roots, full-state root rebuilding and root-file readback. The new 1 KiB and 4 KiB sequences also match all earlier runs with the same leaf size. Tree and undo tests pass under `-race` for 512 B, 1 KiB and 4 KiB; the full normal suite passes again before the sweep.

## Ranked conclusion

1. **Only the tested RAM-only design crosses 2M:** anonymous THP memory, 64-byte undo, 1 KiB SHA-256 leaves, overlapped Commit and a parallel snapshot reach **2,024,107 / 2,022,020 wall msgs/s**, followed by **2,016,162 / 2,016,780** in the baseline repeat. It writes only roots and performs no checkpoint. The measured wall cost is 24.702 / 24.728 ms/block, leaving only 0.298 / 0.272 ms/block before the 2M target is missed; the repeat leaves about 0.20 ms/block. The 512-byte pair reaches 2.002M / 2.040M but does not give a consistent gain, and 4 KiB falls below 2M. Checkpoint cost is unmeasured.
2. **Packed state writes come close but stay below 2M:** the same pipeline with one pwritev per block reaches **1,862,907 / 1,869,199** without fdatasync and **1,848,611 / 1,865,994** with fdatasync per block. The flush adds about 5 ms to Commit and mostly overlaps execution. These logs pass full replay, but recovery, compaction and retention work are absent.
3. **Avoid scattered leaf writes for this workload:** 1 KiB in-place writes require about 13,609 syscalls/block and about 71 ms of write time. Packing reduces that to one pwritev and about 6.7 ms. Larger in-place leaves reduce syscall count but remain far behind the packed layout.

Parallel snapshots reduce the packed pipeline copy from about 3.6 ms to about 1.0 ms. The RAM-only control rises from 1.872 / 1.882M with a serial snapshot to 2.024 / 2.022M with parallel copying. Despite small Commit wait times, packed-log execution remains about 2 ms/block higher than the root-only control. These measurements identify a cost from adding data writes to the pipeline but do not isolate its CPU/cache/writeback components. They are measurements of these diagnostic designs, not a universal upper bound on every possible rewrite.

## Layout extension

The first in-place 1 KiB run needed 13,608.878 pwrite calls per block after merging adjacent dirty leaves. Its write cost dominated execution and hashing. An additional layout control writes a packed sequential log to distinguish syscall and random-write costs from the memory and hashing bound. This is an extension to G/H, not a replacement for their in-place results.

The log starts with the complete initial memory image. Each block appends a header with leaf count, leaf size and root, then sorted uint32 leaf IDs and contiguous dirty-leaf bytes. It uses one pwritev call per block in the absence of short writes, with the index header and contiguous leaf payload as two vectors. The file is preallocated to the maximum possible size before timing. Serial Commit packs the dirty bytes after hashing; this time is reported separately and included in Commit. Overlap reuses the existing immutable dirty-leaf snapshot and only packs the header. Final verification replays the file and checks every recorded block root as well as every final memory byte. This layout still has no crash-recovery or atomic-publication protocol.

The optional parallel-snapshot control copies leaves with the same 16 workers after the prior Commit has completed. It does not add workers. This isolates the 3.2 ms serial snapshot cost observed in H. It is reported separately from the default serial-copy pipeline.

## RAM-only bound

`IDEAL_LAYOUT=root` keeps state in anonymous RAM and writes only the 32-byte root per block. It still performs undo capture, all dirty-leaf hashes and the full-state Merkle fold. The overlap form still snapshots dirty leaves before hashing. It has no state checkpoint during the measured run. An OS crash requires resync; checkpointing every N blocks would add unmeasured work. Native hashes, repeated roots, the independent final full-state root and all root-file bytes are checked.

Write MB/s divides bytes by time inside pwrite/pwritev calls. These are buffered writes to the page cache, not direct measurements of NVMe bandwidth. Per-block fdatasync is timed separately.

## Reproduction and lifecycle

[COMMANDS.md](COMMANDS.md) records the switches. `TestIdealStore` runs two replays internally. CPU affinity is 0-15, GOMAXPROCS is 16, and all inputs, state files and temporary files stay on NVMe. The same Xeon 6975P-C, kernel 7.0.0-1014-aws, Go 1.26.0 and Rust 1.99.0 from round 1 are used. Each measured command runs alone. Most logs include pre-run uptime and vmstat samples; the first in-place no-THP log lacks that header. Infrastructure launch settings stay outside the repo.

The instance launched at 14:14:46 JST on 2026-10-15. Round 2 started at 14:59 JST. The scheduled shutdown remains 17:15:02 JST, with terminate-on-shutdown. The coordinator requested that the box remain running and owns termination verification. At 16:06:12 JST, SSH, uptime and two 100%-idle vmstat samples confirmed the box was still running. [Final health evidence](ideal-final-health.txt) confirms the scheduled shutdown, unchanged writeback settings, original fixture SHA256 and zero leftover diagnostic state files. Termination is unverified. No security-group rule was added and no other instance was touched.

Using the round 1 on-demand estimate of $1.7741/hour, the scheduled 3.0044-hour lifetime costs **$5.33 compute**. This excludes EBS, public IPv4 and taxes; actual termination time and billed cost are unverified. Rate sources: [Spare Cores](https://sparecores.com/server/aws/c8id.8xlarge) and [AWS Pricing](https://aws-pricing.com/c8id.8xlarge.html).

The later [leaf-sweep health record](ideal-leaf-sweep-final-health.txt) contains the final source checksums and another idle check after the requested extra runs. The earlier health record predates the 512-byte option.
