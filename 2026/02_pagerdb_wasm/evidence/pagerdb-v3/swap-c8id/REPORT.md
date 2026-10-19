# PagerDB v3 state larger than RAM through plain Linux swap (c8id.8xlarge)

Question: PagerDB v3 keeps state in anonymous memory (MAP_ANON + MADV_HUGEPAGE). Does it keep working when the
state does not fit in RAM, with plain swap on local NVMe and no file-backed mmap, and how fast is it?

Setup: one c8id.8xlarge (16 cores, 64 GiB), kernel 7.0.0-1014-aws, 200 GiB swap partition on instance-store NVMe,
zswap off, THP `madvise`, swappiness 60, page-cluster 3. PagerDB branch `v3` at `9eac657` plus the test-only file
`swap_bench_test.go` (commit `73515a5`). Workload: BenchmarkEVM shape (64-byte slots, 50,000 read-increment-write ops per block),
400M slots = 23.84 GiB state. "RAM smaller than state" is a cgroup v2 cap on the test process
(MemoryMax, MemorySwapMax=infinity). Checkpoints are off; the journal still writes. Commands: COMMANDS.md.
Every SWAPRESULT row: all-results.md. Raw logs and per-block CSV: raw/.

## Read this first: the RAM footprint is about 2x the state

v3 holds two full anonymous copies of the state: `db.state` and the disk writer's checkpoint image
(`disk.go`, `d.image = anonymous(len(db.state))`), plus the hash tree. Each block, the disk writer copies every
dirty leaf into the image, so image pages are as hot as state pages. After filling 23.84 GiB of state, the
uncapped process had 48.8 GiB anonymous memory (smaps_rollup). So "cap = state x 1/2" (12.8 GB) holds about a
quarter of the real footprint, and "cap = state x 1/3" (8.53 GB) about a sixth. A uniform miss costs up to three
page faults (state leaf, image leaf, tree leaf-hash page), not one.

## Main result (writes/s, 50k ops per block)

Fresh process per cell unless marked. Two numbers = two repeats. "multi" = a later row in a multi-row process.

| Access, prefetch | Fits (no cap) | Cap state x 1/2 (12.8 GB) | Cap state x 1/3 (8.53 GB) |
|---|---|---|---|
| sigma 0.1%, none | 4.36M, 4.38M | 4.38M | 4.37M |
| sigma 1%, none, after 500 warm blocks | 1.90M, 1.91M | 1.84M | 1.88M |
| sigma 1%, none, after 50 warm blocks (multi) | (same) | 1.06M, 1.07M | 1.06M, 1.10M |
| sigma 1%, pipelined 16 | 1.90M, 1.92M | 1.88M | 1.88M |
| uniform, none | 1.43M, 1.44M | 10.1k; multi 11.5k, 11.0k | 9.2k; multi 10.0k, 9.6k |
| uniform, pipelined 16 (multi) | 1.43M, 1.44M | 66.6k, 62.8k | 61.9k, 59.4k |
| uniform, pipelined 64 (multi, THP already split) | 1.43M, 1.43M | 66.9k, 72.9k | 66.0k, 66.5k |
| uniform, pipelined 64, fresh process | (same) | 9.2k | 11.5k |
| uniform, inline 64 (worst case, multi) | 1.43M | 67.3k | 62.0k |
| uniform, MADV_WILLNEED 64 (multi) | 1.19M | 12.4k | 13.4k |

Block time behind those numbers: fits uniform is about 34 ms per block (write p50 5.4 ms, commit p50 28.5 ms).
Capped uniform without prefetch is about 4.0 to 4.7 s per block, with 74k to 89k major faults per block, so about
50 to 55 microseconds per fault on the single executing goroutine. Pipelined prefetch brings it to about 0.6 to
0.75 s per block, almost all of it spent waiting for the prefetch (wait p50 0.49 to 0.66 s): about 140k faults/s.

Major faults per block (process-wide, includes prefetch goroutines), capped, fresh process, measured blocks:
sigma 0.1%: 5; sigma 1% after 500 warm blocks: 60 to 80; uniform: 78k to 89k. pswpout per block is close to
pswpin per block in all uniform capped rows (every swap-in evicts and writes another page): about 75k to 100k
pages, so about 300 to 400 MB of swap writes per block.

Fill (setup) time for 23.84 GiB: 17 s uncapped; 36 to 44 s under the caps (about 40 GiB swapped out during fill);
144 s with THP disabled. AnonHugePages after fill: 48.1 GiB uncapped, 10.3 GiB at cap 1/2, 5.9 GiB at cap 1/3.
Swap used at the end: 37 to 43 GiB in capped rows.

## Findings

1. Hot workloads run at near full speed once warm. sigma 0.1% (hot set about 150 MB of state) shows no
   measurable loss at either cap (4.37M to 4.38M vs 4.36M to 4.38M). sigma 1% converges to within 1% to 4% of
   uncapped (1.84M to 1.88M vs 1.90M to 1.91M).

2. Warm-up is slow without prefetch. sigma 1% at cap 1/2 from a cold resident set (mean per 100 blocks,
   `f-half-s1-off.csv`): faults 6,106 / 517 / 256 / 169 / 133 / 109 / 90 / 77 / 66 / 59; block time
   334 / 48 / 35 / 30 / 29 / 28 / 27 / 26 / 26 / 26 ms (uncapped: 25.3 ms). Measuring after only 50 warm
   blocks gives 1.06M to 1.10M (p99 write 53 ms), which is warm-up, not steady state. The sigma 1% pipelined rows
   in the multi-row processes (1.90M to 1.91M at both caps) started on an already warm set because of the seed
   reuse (COMMANDS.md); the fresh-process pipelined rows (1.88M) are the valid ones. With pipelined prefetch the
   first 100 cold blocks average 64 ms instead of 334 ms, then 25 to 26 ms.

3. Uniform access over a state that does not fit is not usable as a node. No prefetch: 140x slower than fits
   (about 10k vs 1.43M writes/s). Best prefetch: about 20x slower (60k to 73k writes/s). At cap = state x 1
   (25.6 GB, about half the footprint): 18.2k without prefetch, 100k with 16 goroutines, 122k with 64.

4. Prefetch concurrency is limited by GOMAXPROCS. A goroutine that takes a page fault blocks its OS thread and
   keeps its P, so with GOMAXPROCS=16 at most 16 faults run at once. 16 and 64 prefetch goroutines give the same
   result (66.6k vs 66.9k, 62.8k vs 72.9k). A run with GOMAXPROCS=128 (`half-gmp128`) was much worse (9.3k to
   15.6k), but it was a fresh process and hit finding 5, so the effect of more concurrent faults is unverified.
   A real prefetcher probably needs a syscall-based path (for example `pread` on a file, or `process_madvise`
   with a populate mode) or more threads than Ps; not tested.

5. Transparent huge pages break pipelined prefetch until they are split. In a fresh process right after the
   fill, most resident state, image and tree memory is in 2 MiB pages (AnonHugePages 9 to 11 GiB at cap 1/2).
   There, uniform pipelined prefetch did not help: 9.2k (cap 1/2) and 11.5k (cap 1/3) writes/s, with commit p50
   4.0 to 4.8 s, prefetch wait near zero, and 125k to 137k faults per block instead of 75k to 85k. The same row
   gets 71.2k after 800 sigma 1% blocks in the same process (AnonHugePages down to 6 GiB) and 72.8k with THP set
   to `never` (`f-half-thpnever-u-pipe64`). By then the kernel had counted 360k `thp_swpout` and 360k `thp_split_pmd`
   (raw/vmstat-thp-before-never.txt; 382k and 429k at the end): whole 2 MiB pages are swapped out as a unit and split on swap-in. The likely cause is
   that reclaiming a 2 MiB unit also evicts 4 KiB pages that were just prefetched, so the commit path faults them
   again; this mechanism is not verified. THP `never` makes the fill 3.8x slower (144 s vs 38 s) and was not
   measured on the hot rows.

6. MADV_WILLNEED per leaf does not work as a prefetch here. Capped: 12.4k to 13.4k (no better than no prefetch);
   pswpin per block is about 127k against 43k to 49k major faults, so readahead reads pages that are not used in
   time. Uncapped it costs 17% (1.19M vs 1.43M; commit p50 36.6 vs 28.5 ms).

7. Inline prefetch (worst case, inside the block timer) is as fast as pipelined for uniform (67.3k vs 66.9k at
   cap 1/2), because the prefetch time (0.6 s) dwarfs execution (about 50 ms). Uncapped, inline prefetch adds
   about 1 ms to write p50 (6.4 vs 5.4 ms) and does not change throughput. Pipelined 16/64 costs nothing
   uncapped (1.43M; sigma 1% 1.90M to 1.92M).

8. No sign of the Go heap or GC being swapped into the critical path. In capped sigma 0.1% runs the slowest
   blocks are 11.0 to 11.5 ms, the same as the slowest uncapped block (11.8 ms), and they have only 3 to 13
   faults. In capped sigma 1% runs block time correlates with major faults (r = 0.55 and 0.76). GC ran 0 to 2
   times per row; Go heap was 1.5 to 2.7 GiB (snapshot slots and the undo ring). Per-mapping swap of the Go heap
   was not measured, so this is an absence of evidence only.

9. Real memory pressure without a cgroup (state 48 GiB = 800M slots, footprint about 97 GiB = 1.5x RAM,
   uncapped): sigma 1% 1.71M writes/s with about 1 fault per block (commit 23.6 ms vs 20.6 ms at 24 GiB, larger
   tree); uniform 20.2k without prefetch and 150k with pipelined 16. The sigma hot set was already resident after
   the sequential fill, so this row did not exercise warm-up. Fill took 55 s.

10. zswap (lzo, max_pool_percent 20), cap 1/2, one run each: sigma 1% steady 1.86M (same as plain swap);
    uniform without prefetch 12.3k (vs 10.1k plain); uniform pipelined 16 in a fresh process 24.8k (THP effect
    from finding 5 applies, so not comparable). Not conclusive.

## What this means for "full speed when it fits, slower but working when it does not"

- Fits: yes, the swap partition costs nothing when unused (no swap activity in either uncapped repeat).
- Skewed access larger than RAM: works, near full speed after warm-up. Warm-up from a cold set takes hundreds of
  blocks unless the next block's leaves are prefetched.
- Uniform access larger than RAM: works but 20x to 140x slower, and writes about 300 to 400 MB of swap per
  block. That is not "slower but working" for a chain at 50k writes per block.
- Budget RAM against about 2.05x the state (state plus checkpoint image), not 1x.
- If swap is the plan, THP on the state region interferes with prefetch until pages are split.

## Not verified

- Raw random-read IOPS of the swap device (no fio run), so whether 140k faults/s is the device or the kernel limit.
- Whether more than 16 concurrent faulting threads helps once THP is split.
- The THP mechanism in finding 5 and THP `never` effect on hot rows.
- page-cluster and swappiness tuning (defaults only), zswap repeats, MGLRU settings.
- Behavior with checkpoints enabled (they read the whole changed image) and with persistence fsyncs.
- Long-run steady state beyond 1,000 blocks; uniform rows measured only 10 to 30 blocks.
- One instance only.

## Teardown and cost

Launch 2026-10-15 22:56:32 JST. Shutdown requested 2026-10-16 00:07:03 JST; AWS reported `terminated` at
00:07:44 JST (describe-instances on this instance id). About 71 minutes of instance time; at an assumed
on-demand rate of about 1.9 USD/h this is about 2.3 USD (rate not checked). No other instance was touched.
No security-group change.
