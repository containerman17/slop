# Reproduce: PagerDB v3 state larger than RAM through Linux swap

PagerDB source: branch `v3`, commit `73515a5` (= `9eac657` plus the test-only file `swap_bench_test.go`; no production change).
The committed test is the final version. Two earlier variants ran during the session (see "Spec history" below).

## Machine

One c8id.8xlarge in us-west-2a (16 physical cores, 32 threads, 64 GiB, 1.7 TiB instance-store NVMe),
Ubuntu 24.04 amd64, kernel 7.0.0-1014-aws, 40 GiB gp3 root deleted on termination.
Boot user data `shutdown -h +180`, shutdown behavior terminate. Launch settings stay in the private coordinator cache.
Settings at start: zswap off, swappiness 60, page-cluster 3, THP `madvise`, THP defrag `madvise`,
khugepaged max_ptes_swap 64, swap VMA readahead on. Full record in `raw/environment.txt`.

```sh
# NVMe split: 200 GiB swap partition, the rest ext4 for TMPDIR, Go cache and the PagerDB journal.
sudo parted -s /dev/nvme1n1 mklabel gpt mkpart swap linux-swap 1MiB 200GiB mkpart data ext4 200GiB 100%
sudo mkswap /dev/nvme1n1p1 && sudo swapon /dev/nvme1n1p1
sudo mkfs.ext4 -q -F /dev/nvme1n1p2 && sudo mkdir -p /mnt/nvme && sudo mount /dev/nvme1n1p2 /mnt/nvme && sudo chown ubuntu: /mnt/nvme
mkdir -p /mnt/nvme/tmp /mnt/nvme/pagerdb /mnt/nvme/results
curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -
# From the PagerDB v3 worktree: stream tracked files plus the new test to the box.
tar -cf - $(git ls-files) swap_bench_test.go | SSH 'tar -xf - -C /mnt/nvme/pagerdb'
cd /mnt/nvme/pagerdb && . /mnt/nvme/env.sh && go vet ./...
```

`SSH` is the private SSH helper. `env.sh` and `run.sh` are in this directory. `run.sh NAME CAP RUNS [VAR=value...]`
runs `go test -run '^TestSwapEVM$' -v` pinned with `taskset -c 0-15`, GOMAXPROCS=16, and writes
`results/NAME.txt` (log plus global pswpin/pswpout before and after) and `results/NAME.csv` (per block).
With a cap it runs inside a transient systemd scope:
`systemd-run --scope -p MemoryMax=CAP -p MemorySwapMax=infinity --uid=ubuntu --gid=ubuntu`.

## Benchmark knobs (TestSwapEVM)

- `PAGERDB_SWAP_SLOTS` (default 400000000): 64-byte slots. 400M slots = 25.6 GB = 23.84 GiB state, MaxLeaves 2^23.
- `PAGERDB_SWAP_RUNS`: comma-separated `sigma:mode:goroutines:warmup:blocks`. sigma is a fraction of the slot count
  (0 = uniform). mode is `off`, `pipe` (touch block k+1 while block k runs), `inline` (touch block k inside its
  timer before the writes) or `willneed` (pipelined MADV_WILLNEED per leaf instead of touching).
  Prefetch touches one byte of the state leaf, of the checkpoint image leaf and of the tree leaf-hash node.
- `PAGERDB_SWAP_CSV`: per-block CSV (negative block numbers are warmup).
- Fill writes the same slot contents as `evmSlots`, 16 MiB per Commit. CheckpointInterval is MaxUint64 (no checkpoint).
  The journal still writes to `TMPDIR` on NVMe. 50,000 read-increment-write ops per block, as in BenchmarkEVM.
- Each run in one process uses rng seed `run index + 1`.

## Runs, in execution order (all times JST, 2026-10-15/16)

Caps: half = 12800000000 bytes (state x 1/2), third = 8533333333 bytes (state x 1/3), onex = 25600000000 (state x 1).

```sh
/mnt/nvme/run.sh fits-1   none        "<see spec history>"
/mnt/nvme/run.sh half-1   12800000000 "0.001:off:0:50:200,0.01:off:0:50:200,0.01:pipe:16:50:200,0.01:pipe:64:50:200,0:pipe:64:5:30,0:pipe:16:5:30,0:willneed:64:5:30,0:inline:64:5:30,0:off:0:3:15"
/mnt/nvme/run.sh third-1  8533333333  "0.001:off:0:50:200,0.01:off:0:50:200,0.01:pipe:16:50:200,0.01:pipe:64:50:200,0:pipe:64:5:30,0:pipe:16:5:30,0:willneed:64:5:30,0:inline:64:5:30,0:off:0:3:10"
/mnt/nvme/run.sh half-gmp128 12800000000 "0:pipe:64:5:30,0:pipe:128:5:30,0:inline:128:5:30,0:off:0:3:10" GOMAXPROCS=128
KEY="0.001:off:0:50:200,0.01:off:0:50:200,0.01:pipe:16:50:200,0:pipe:16:5:30,0:pipe:64:5:30,0:off:0:3:10"
/mnt/nvme/run.sh half-2   12800000000 "$KEY"
/mnt/nvme/run.sh third-2  8533333333  "$KEY"
/mnt/nvme/run.sh fits-2   none        "0.001:off:0:50:200,0.01:off:0:50:200,0.01:pipe:16:50:200,0:off:0:20:100,0:pipe:16:20:100,0:pipe:64:20:100"
/mnt/nvme/run.sh onex-1   25600000000 "$KEY"
# Fresh process per cell (final test version, seed = run index + 1):
for cap in half:12800000000 third:8533333333; do n=${cap%%:*}; c=${cap#*:}
  /mnt/nvme/run.sh f-$n-s1-off     $c "0.01:off:0:500:500"
  /mnt/nvme/run.sh f-$n-s1-pipe16  $c "0.01:pipe:16:500:500"
  /mnt/nvme/run.sh f-$n-s01-off    $c "0.001:off:0:500:500"
  /mnt/nvme/run.sh f-$n-u-off      $c "0:off:0:3:10"
  /mnt/nvme/run.sh f-$n-u-pipe64   $c "0:pipe:64:5:30"
done
/mnt/nvme/run.sh f-half-sigwarm-u-pipe64 12800000000 "0.01:off:0:0:800,0:pipe:64:5:30"
grep -E "^(thp_|compact_stall)" /proc/vmstat > /mnt/nvme/results/vmstat-thp-before-never.txt
echo never | sudo tee /sys/kernel/mm/transparent_hugepage/enabled
/mnt/nvme/run.sh f-half-thpnever-u-pipe64 12800000000 "0:pipe:64:5:30,0:off:0:3:10"
echo madvise | sudo tee /sys/kernel/mm/transparent_hugepage/enabled
grep -E "^(thp_|compact_stall)" /proc/vmstat > /mnt/nvme/results/vmstat-thp-after-never.txt
/mnt/nvme/run.sh big-uncapped none "0.01:off:0:500:500,0:off:0:2:5,0:pipe:16:5:20" PAGERDB_SWAP_SLOTS=800000000
echo Y | sudo tee /sys/module/zswap/parameters/enabled   # lzo, max_pool_percent 20 (raw/zswap-params.txt)
/mnt/nvme/run.sh z-half-s1-off   12800000000 "0.01:off:0:500:500"
/mnt/nvme/run.sh z-half-u-pipe16 12800000000 "0:pipe:16:5:30,0:off:0:3:10"
echo N | sudo tee /sys/module/zswap/parameters/enabled
grep -E "^(thp_|compact_stall|pswp)" /proc/vmstat > /mnt/nvme/results/vmstat-final.txt
```

## Spec history

- `fits-1` ran an earlier spec format `sigma:mode:goroutines:blocks` with one global warmup of 20 blocks:
  `0.001:off:0:200,0.01:off:0:200,0:off:0:100,0.01:pipe:16:200,0.01:pipe:64:200,0:pipe:16:100,0:pipe:64:100,0:inline:64:100,0:willneed:64:100`.
  The equivalent current spec puts `:20:` before each block count.
- `fits-1` through `onex-1` used rng seed 1 for every run, so a later run in the same process replayed the slot
  sequence of an earlier one. For sigma rows that makes later rows start warm (see REPORT). The `f-*`, `big-*`
  and `z-*` runs use the committed code (seed = run index + 1).

## Summary table

```sh
python3 summarize.py raw/fits-1.txt raw/fits-2.txt raw/f-half-*.txt raw/half-1.txt raw/half-2.txt raw/half-gmp128.txt \
  raw/f-third-*.txt raw/third-1.txt raw/third-2.txt raw/onex-1.txt raw/big-uncapped.txt raw/z-half-*.txt > all-results.md
```

Teardown: `sudo shutdown -h now` on this instance only, then `describe-instances` on its id until `terminated`.
