# PagerDB v3: image file instead of a second in-RAM state copy

PagerDB commits on branch `v3`: `12b7bbe` (image file, applier, CURRENT, crash test), `88753dc` (per-leaf chunk-mask redo),
`180fcdd` (dense leaf index), `b8ba9ec` (unmap the image before fdatasync on Linux). Final measured commit: **b8ba9ec**.
The Wasm host on `pagerdb-v3` now pins `v0.0.0-20261015164433-b8ba9eceb6fa` (no replace directive).
One c8id.8xlarge (16 cores, 32 threads, Xeon 6975P-C, 64 GiB, instance NVMe), all runs pinned with `taskset -c 0-15`,
GOMAXPROCS=16. Runs took place 2026-10-16 01:06 to 02:01 JST.

## Result

Anonymous memory is now about 1x the state. On the same box, throughput is within -2.3% to +2.7% of the old code.

| Linux, TestMemoryFootprint | before (73515a5) RssAnon | after (b8ba9ec) RssAnon |
|---|---|---|
| 4 GiB state | 8.29 GiB | 4.29 GiB |
| 16 GiB state | 32.54 GiB | 16.49 GiB |

After b8ba9ec, RssFile is 3.4 MB at the end of both runs: the applier unmaps the image after each batch, so image
pages stay only in the page cache. VmHWM at 16 GiB is 18.97 GiB, because image pages count in RSS while a batch has
them mapped. macOS, 4 GiB (diagnostic, 12b7bbe): physical footprint 8480 MB before, 4379 MB after.

Swap row: 23.84 GiB state, sigma 1%, cgroup MemoryMax 12.8 GB (state x 1/2), 500 + 500 blocks, the same spec as the
earlier `f-half-s1-off` (`../swap-c8id`, other box, 73515a5):

| | f-half-s1-off (73515a5) | swap-half-s1-off (b8ba9ec) |
|---|---|---|
| writes/s | 1,844,238 | 1,853,071 |
| swap used at end | 38.4 GiB | 22.6 GiB |
| anon in RAM at end | 11.5 GiB | 3.7 GiB |
| majflt / pswpin / pswpout per block | 80 / 110 / 56 | 39 / 60 / 0 |
| write p50/p99 ms | 13.4 / 15.9 | 9.9 / 18.1 |
| commit p50/p99 ms | 12.9 / 15.3 | 16.2 / 20.2 |
| fill s | 38.5 | 41.7 |

Anonymous memory (RAM plus swap) falls from 2.1x to 1.1x the state; the rest is about 2.6 GiB of Go heap.
Without a cap (`swap-fits-s1-off`) the same run gives 2,087,731 writes/s with 26.2 GiB peak RssAnon; the earlier
uncapped sigma 1% rows gave 1.90M to 1.91M writes/s with 48.1 GiB of anonymous huge pages. Applier lag: at most
171 versions (452 MB of journal) capped and 121 versions (318 MB) uncapped. NVMe writes: 760 MB/s capped (includes
swap-out after fill), 1244 MB/s uncapped.

## Same-box throughput, b8ba9ec versus 9eac657

Both builds ran on this instance with the same instrumentation pattern (`instrument.py` here, the round 3
`../cloud-c8id-r3/instrument.py` for 9eac657). The applier runs in every b8ba9ec row.

| row | 9eac657 run 1 / 2 | b8ba9ec run 1 / 2 | pair mean change |
|---|---|---|---|
| EVM uniform, writes/s | 3.864M / 3.868M | 3.967M / 3.975M | +2.7% |
| EVM sigma 1%, writes/s | 16.901M / 16.806M | 16.860M / 16.620M | -0.7% |
| EVM sigma 0.1%, writes/s | 33.158M / 33.215M | 33.261M / 33.149M | +0.1% |
| Wasm original, wall msgs/s | 1.790M / 1.794M | 1.791M / 1.801M | +0.2% |
| Wasm synthetic, wall msgs/s | 1.023M / 1.023M | 0.999M / 1.000M | -2.3% |

Against the round 3 box (EVM 3.94M / 17.09M / 33.5M, Wasm 1.845M / 1.089M) b8ba9ec is +1.1% / -1.9% / -1.5% on EVM
and -2.7% / -8.0% on Wasm. The 9eac657 rerun is 2.9% (original) and 5.8% (synthetic) slower here than on the round 3
box, so the same-box table is the fair comparison.

Applier and disk metrics, b8ba9ec, run 1 / run 2:

| row | max lag versions | max lag MB | end lag versions | Checkpoint catch-up ms | NVMe write MB/s | journal MB/block | peak RssAnon MiB |
|---|---|---|---|---|---|---|---|
| EVM uniform | 204 / 204 | 440 / 440 | 109 / 106 | 617 / 602 | 219 / 220 | 2.2 | 1113 / 1115 |
| EVM sigma 1% | 453 / 445 | 546 / 536 | 376 / 382 | 434 / 436 | 195 / 189 | 1.2 | 771 / 752 |
| EVM sigma 0.1% | 700 / 700 | 214 / 214 | 53 / 55 | 13 / 13 | 188 / 187 | 0.3 | 726 / 726 |
| Wasm original | 77 / 76 | 714 / 704 | 35 / 32 | 304 / 281 | 537 / 543 | 9.9 | 1677 / 1902 |
| Wasm synthetic | 43 / 43 | 616 / 604 | 26 / 27 | 501 / 480 | 465 / 466 | 16.4 | 2539 / 3006 |

Lag is sampled after each Commit: versions the published image trails the head, and journal bytes not yet applied.
The EVM sigma 0.1% run lasts about one second, so the applier first runs at the final Checkpoint (max lag = all 700
blocks). No run came near the 4 GiB bound, so no run hit backpressure. NVMe MB/s is sectors written to the ext4
partition (`/proc/diskstats`) over the timed interval: journal, image and any writeback already in flight.
EVM peak RssAnon is the whole benchmark process (128 MB state, snapshots, undo, 32 retained versions).
Journal bytes per block fall from 10.67 to 9.93 MB (original) and from 17.83 to 16.39 MB (synthetic): redo carries
each changed 32-byte chunk once, while 9eac657 journaled undo for every write.

## Correctness

- PagerDB b8ba9ec on Linux: go vet, go test and go test -race pass. TestCrashRecovery, 1000 SIGKILL iterations, pass:
  apply 143, applying 143, image-synced 143, manifest-written 143, manifest-renamed 143, rotated 142, timer 143.
  Each iteration kills the child at that point, reopens, and compares version, root and every state byte with an
  in-process reference.
- Earlier trees: Linux 1000 iterations each at 12b7bbe and 180fcdd; macOS 2000 before the first commit, 400 at the
  12b7bbe tree and 400 at 180fcdd. All 5800 iterations pass. Two earlier macOS runs failed from a test harness bug
  (lines printed after the kill were not parsed); the harness was fixed, not the database.
  macOS keeps msync, so the b8ba9ec Linux sync path is covered by the Linux run only.
- v3 has no fuzz targets, so no fuzzers ran.
- Wasm host, instrumented workspace: vet and tests pass; both 600-block fixtures pass twice with every native hash
  and the repeated root sequence. Final roots match round 3 (`5e72f0ac...` original, `497cfdff...` synthetic).
- Wasm host with the pinned module and no workspace: vet and tests pass, both fixtures pass twice.
- EVM: each 700-block root sequence matches across the two repetitions, and Verify rebuilds the final root.

## Fixes found on the box

The first version (12b7bbe) regressed EVM to 3.13M / 7.6M / 13.1M writes/s (`raw/earlier/evm-12b7bbe.txt`). The disk
worker was 100% busy: copying each written range's final bytes from the snapshot in write order missed the cache
(2.84 s of 13.8 s CPU on sigma 1%), so Commit waited for snapshot slots. 88753dc writes redo per leaf with a 128-bit
mask of changed 32-byte chunks and reads the snapshot in leaf order. 180fcdd replaces the per-record binary search
with a dense leaf index (4 bytes per possible leaf, touched lazily). After that, EVM was at parity.

On the Wasm host the applier still cost about 5%: execution 25.8 versus 24.3 ms/block with the applier deferred
(`raw/earlier/exp-noapply-original-180fcdd.txt`). msync write-protects each mapped page it cleans, which sends
TLB-flush interrupts to every thread. b8ba9ec drops the mapping's page table entries first (MADV_DONTNEED keeps the
data dirty in the page cache) and then calls fdatasync. Original went from 1.767M / 1.776M to 1.791M / 1.801M.

For synthetic, a deferred applier (`raw/exp-defer-synthetic.txt`, 1.017M / 1.020M) and a 5 s apply interval
(`raw/exp-i5s-synthetic.txt`, 1.000M / 1.004M) do not close the remaining 2.3% gap. The cause is not isolated.

## Not verified

- Power loss and OS crash. The crash test kills the process only. The sync order follows the README protocol but
  was not tested against real power loss.
- Recovery time with a large unapplied journal (up to the 4 GiB bound) was not timed.
- No benchmark reached the 4 GiB lag bound. TestApplierLagBound checks the wait logic with a small bound.
- Mac timings are diagnostic only.
