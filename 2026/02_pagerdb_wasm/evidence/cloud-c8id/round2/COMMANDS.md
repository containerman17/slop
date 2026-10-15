# Anonymous store diagnostic

Source baseline: `1242b24` locally, original cloud archive `840d09d` remotely.
The two files `ideal_store_diagnostic_test.go` and `ideal_log_diagnostic_test.go`
are added to the remote source copy at `/mnt/nvme/ideal`. No production source or module dependency changes.
The original fixture stays at `/mnt/nvme/original.bin`; it is never copied again.
Infrastructure details remain outside the repo in the private coordinator cache.

```sh
source /mnt/nvme/env
cd /mnt/nvme/ideal
export CARGO_TARGET_DIR=/mnt/nvme/target-ideal
export PAGERWASM_FIXTURE=/mnt/nvme/original.bin PAGERWASM_IDEAL=1
# Each invocation executes two independent 600-block replays.
IDEAL_CSV=/mnt/nvme/results/ideal-g-1k-small \
  taskset -c 0-15 go test -run '^TestIdealStore$' -count=1 -v -timeout=15m .
```

Variant settings: `IDEAL_HUGE=1` uses MADV_HUGEPAGE; default explicitly uses
MADV_NOHUGEPAGE. `IDEAL_SYNC=1` adds fdatasync per block. `IDEAL_OVERLAP=1`
uses a bounded background Commit with dirty-leaf snapshots. `IDEAL_LEAF=4096`
uses 4 KiB hash and write leaves; `IDEAL_LEAF=512` selects 512-byte leaves.
First-write undo remains 64 bytes and state-gas
charges still count the production 1 KiB leaves. Defaults are 1 KiB, serial,
without fdatasync. CSV prefix changes for each configuration. Do not set
`IDEAL_BLOCKS` for measured runs; it only truncates correctness smoke tests.

Smoke checks use `IDEAL_BLOCKS=3` or `5`. The overlap smoke uses `-race`,
MADV_HUGEPAGE and fdatasync. Race timings are not performance measurements.
All commands use CPUs 0-15 and GOMAXPROCS=16. One benchmark runs at a time.
Temporary state files are preallocated on NVMe and removed after each replay.
No kernel writeback settings are changed in round 2.

The diagnostic source includes an optional packed-log layout. Set
`IDEAL_LAYOUT=log`; all other switches are unchanged.
The log retains all measured blocks, with no compaction or retention work.
Stream both diagnostic test files when preparing the source copy.

```sh
taskset -c 0-15 go vet ./...
PAGERWASM_IDEAL=1 IDEAL_BLOCKS=5 IDEAL_LAYOUT=log IDEAL_OVERLAP=1 \
  IDEAL_HUGE=1 IDEAL_SYNC=1 \
  taskset -c 0-15 go test -race -run '^TestIdeal' -count=1 -v -timeout=15m .
```

`IDEAL_PARALLEL_SNAPSHOT=1` is an additional overlap control. After the previous
Commit completes, it copies dirty leaves with the same 16 persistent workers
before submitting the next Commit. It does not add workers or overlap a copy
with an earlier hash job. Default snapshots use a serial copy loop.

`IDEAL_LAYOUT=root` writes only each block's 32-byte root and keeps state in RAM.
There is no per-block data write or measured checkpoint. The packed `log` layout
uses one pwritev call with a header vector and a contiguous payload vector.
`write_MB_s` is bytes divided by time inside the write calls, in decimal MB/s;
it excludes packing, hashing and fdatasync.

[RUNS.tsv](RUNS.tsv) lists all measured configurations. `inplace` means an empty
`IDEAL_LAYOUT`; other layout values are used directly. Set every switch explicitly
when reproducing a row, so a prior shell setting cannot change the next run:

```sh
PAGERWASM_IDEAL=1 PAGERWASM_FIXTURE=/mnt/nvme/original.bin \
  IDEAL_LAYOUT=log IDEAL_HUGE=1 IDEAL_OVERLAP=1 IDEAL_SYNC=1 \
  IDEAL_PARALLEL_SNAPSHOT=1 IDEAL_LEAF=1024 \
  IDEAL_CSV=/mnt/nvme/results/ideal-log-h-huge-parallel-sync \
  taskset -c 0-15 go test -run '^TestIdealStore$' -count=1 -v -timeout=15m .

# Normal final verification, with performance opt-ins disabled.
unset PAGERWASM_IDEAL PAGERWASM_FIXTURE IDEAL_BLOCKS
taskset -c 0-15 go vet ./...
taskset -c 0-15 go test -count=1 -v -timeout=30m ./...

# On the workstation, check every CSV root and printed phase mean.
python3 evidence/cloud-c8id/round2/analyze.py > evidence/cloud-c8id/round2/METRICS.md
```

Early smoke logs precede the one-pwritev layout change. The final packed and
parallel code is checked by `ideal-pwritev-race-smoke.txt`; root-only behavior is
checked by `ideal-root-race-smoke.txt`. Smoke timings are excluded from METRICS.
Raw text files have trailing whitespace removed; values and output lines are preserved.

The final requested leaf sweep uses `IDEAL_LAYOUT=root IDEAL_HUGE=1
IDEAL_OVERLAP=1 IDEAL_PARALLEL_SNAPSHOT=1`, with `IDEAL_LEAF=512`, `1024`,
and `4096`. Each size runs twice. The 1 KiB sweep entry repeats the earlier
baseline; it is a separate measured command. State gas still counts 1 KiB leaves.
