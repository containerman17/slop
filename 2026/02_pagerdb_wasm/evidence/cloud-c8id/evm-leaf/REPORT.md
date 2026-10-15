# Uniform EVM workload: 1 KiB versus 4 KiB leaves

Measured on 2026-10-15, 16:49:08 to 16:49:35 JST, on the existing c8id.8xlarge. One 200-block run per leaf size, baseline first. Each block performs 50,000 uniform read-increment-write operations across 2,000,000 slots, seed 1, 32-byte values, retain 32. All temporary databases reside on local ext4 NVMe. CPUs 0-15 are 16 physical cores; GOMAXPROCS=16.

| Leaf size | Writes/s, including final Seal | Write ms/block, p50 | Commit ms/block, p50 | Write ms/block, p99 | Commit ms/block, p99 | Final Seal ms |
|---|---:|---:|---:|---:|---:|---:|
| 1024 B | 1,885,955 | 10.78 | 9.273 | 14.72 | 10.18 | 1,220 |
| 4096 B | 1,917,964 | 10.71 | 9.155 | 14.99 | 10.15 | 1,163 |

4 KiB improves measured throughput by 1.70% in this single pair. Median write and Commit times are almost unchanged. Neither reaches 2M writes/s. This short test does not establish a repeatable advantage or sustained behavior across multiple automatic seals. Each run reports one sealed epoch. No filesystem cache clearing was performed between runs.

These are the existing benchmark's percentile timers, not arithmetic means. The write phase includes RNG, unique-slot accounting, and read-increment-write. Throughput includes generation, counting, read/write, Commit, and the final Seal; setup and Close are excluded. Its dirty-leaf counter hardcodes slot/16, so the 4 KiB dirty-leaf metrics in the raw output are invalid as actual leaf counts and are not compared here.

Both benchmark invocations passed. The existing TestEVMSameSeedRoot passed for both copies and all three small distributions. That test checks repeat-root equality on a small workload. The timed benchmark checks operation errors but discards Commit roots; its full 200-block root sequence was not independently verified. No claim of cross-format root equality or complete 4 KiB compatibility is made.

## Source and commands

Source: unchanged module github.com/containerman17/pagerdb v0.0.0-20261014093015-af2c4c95bb9d, corresponding to af2c4c9. Both writable copies were made on the box from the module cache. A recursive comparison confirmed that the baseline matched the source and that only pagerdb.go differed in the candidate. The only edit changed `LeafSize = 1024` to `LeafSize = 4096`. The benchmark source is byte-identical in both copies; SHA-256 hashes are in provenance.txt. No source patch or fixture is committed.

```sh
source /mnt/nvme/env
src=/mnt/nvme/go/pkg/mod/github.com/containerman17/pagerdb@v0.0.0-20261014093015-af2c4c95bb9d
cp -a "$src" /mnt/nvme/evm-leaf1024
cp -a "$src" /mnt/nvme/evm-leaf4096
chmod -R u+w /mnt/nvme/evm-leaf1024 /mnt/nvme/evm-leaf4096
sed -i 's/LeafSize = 1024/LeafSize = 4096/' /mnt/nvme/evm-leaf4096/pagerdb.go

# Run separately, one benchmark at a time, in each copy:
cd /mnt/nvme/evm-leaf1024 # then /mnt/nvme/evm-leaf4096
TMPDIR=/mnt/nvme/tmp GOMAXPROCS=16 PAGERDB_EVM_RETAIN=32 taskset -c 0-15 \
  go test -run '^$' -bench '^BenchmarkEVM$/uniform$' -benchtime=200x -count=1 -timeout=5m
TMPDIR=/mnt/nvme/tmp GOMAXPROCS=16 taskset -c 0-15 \
  go test -run '^TestEVMSameSeedRoot$' -count=1 -v -timeout=2m
```

Go 1.26.0; Intel Xeon 6975P-C; kernel 7.0.0-1014-aws. See provenance.txt for idle load and topology. Raw benchmark and root-test outputs are adjacent to this report. No new instance was launched. The existing box remains running for coordinator teardown verification, with scheduled shutdown at 17:15 JST; termination is unverified.
