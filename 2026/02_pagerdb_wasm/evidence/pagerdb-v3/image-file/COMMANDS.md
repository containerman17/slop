# Reproduce: PagerDB v3 image file

PagerDB `v3` at `b8ba9ec` (earlier rows: 12b7bbe, 180fcdd). Baseline for the same-box rows: `9eac657`.
Memory baseline: `73515a5` plus the new `memory_footprint_test.go` (public API only).
Wasm host: branch `pagerdb-v3`, subtree `2026/02_pagerdb_wasm`.

## Machine

One c8id.8xlarge in us-west-2a, Ubuntu 24.04 amd64, kernel 7.0.0-1014-aws, 40 GiB gp3 root deleted on termination.
Boot user data `shutdown -h +180`, shutdown behavior terminate. Launch request and response stay in the private
coordinator cache. Launch 2026-10-16 01:06 JST; `sudo shutdown -h now` at 02:01:41 JST; AWS `describe-instances`
on that instance only reported `terminated` at 02:02:16 JST. `raw/environment.txt` has the CPU, THP, zswap and
swappiness settings.

```sh
# NVMe: 200 GiB swap partition, the rest ext4 for TMPDIR, Go cache, PagerDB stores and results.
sudo parted -s /dev/nvme1n1 mklabel gpt mkpart swap linux-swap 1MiB 200GiB mkpart data ext4 200GiB 100%
sudo mkswap /dev/nvme1n1p1 && sudo swapon /dev/nvme1n1p1
sudo mkfs.ext4 -q -F /dev/nvme1n1p2 && sudo mkdir -p /mnt/nvme && sudo mount /dev/nvme1n1p2 /mnt/nvme && sudo chown ubuntu: /mnt/nvme
curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -
sudo apt-get install -y build-essential perl pkg-config && curl -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal
# Local, streamed over the private SSH helper (SSH):
git -C PAGERDB archive b8ba9ec | SSH 'tar -xf - -C /mnt/nvme/pagerdb'
git -C PAGERDB archive 73515a5 | SSH 'tar -xf - -C /mnt/nvme/pagerdb-before'   # plus memory_footprint_test.go
git -C SLOP archive HEAD:2026/02_pagerdb_wasm | SSH 'tar -xf - -C /mnt/nvme/repo'          # host at the old pin
# repo-pinned: the host working tree with go.mod bumped to b8ba9ec (no replace)
SSH 'cat > /mnt/nvme/original.bin' < original 600-block fixture
cd /mnt/nvme/repo && go run ./cmd/fixture 600 > /mnt/nvme/synthetic.bin
# Instrumented copies (diagnostic counters only, no storage change):
cp -a repo repo-diag; cp -a pagerdb pagerdb-diag
python3 instrument.py pagerdb-diag repo-diag
gofmt -w pagerdb-diag/{pagerdb.go,disk.go,evm_bench_test.go} repo-diag/matcher_test.go
go work init ./repo-diag ./pagerdb-diag      # /mnt/nvme/go.work
```

`raw/env.sh`, `raw/run.sh` and `raw/final2.sh` are the scripts as run. `final2.sh` produced every file directly in
`raw/`, in this order:

```sh
# Core, pristine archive (GOWORK=off): vet, tests, race, 1000-iteration SIGKILL crash test
go vet ./... && go test -count=1 ./... && go test -race -count=1 ./...
PAGERDB_CRASH_ITERS=1000 go test -count=1 -run '^TestCrashRecovery$' -v .                  # core-checks.txt
# Memory, before = 73515a5, after = b8ba9ec
PAGERDB_MEM_GIB=4|16 taskset -c 0-15 go test -count=1 -run '^TestMemoryFootprint$' -v .     # mem-*.txt
# Instrumented workspace, PAGERDB_DISK=nvme1n1p2
cd pagerdb-diag && PAGERDB_EVM_RETAIN=32 taskset -c 0-15 go test -run '^$' -bench '^BenchmarkEVM$' -benchtime=700x -count=2 -v   # evm.txt
cd repo-diag && go vet ./... && taskset -c 0-15 go test -count=1 ./...                       # host-checks.txt
PAGERWASM_FIXTURE=/mnt/nvme/original.bin  taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v .   # original.txt
PAGERWASM_FIXTURE=/mnt/nvme/synthetic.bin taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v .   # synthetic.txt
# Pinned host, GOWORK=off: same three commands                                               # pinned-*.txt
# Swap rows, pristine core, run.sh NAME CAP RUNS [VAR=value]
run.sh swap-half-s1-off 12800000000 "0.01:off:0:500:500" PAGERDB_DISK=nvme1n1p2
run.sh swap-fits-s1-off none        "0.01:off:0:500:500" PAGERDB_DISK=nvme1n1p2
```

Each TestMatcher invocation runs two replays. `run.sh` wraps a capped run in
`systemd-run --scope -p MemoryMax=CAP -p MemorySwapMax=infinity` and records global pswpin/pswpout before and after.

Same-box baseline (`raw/base-9eac657-*.txt`): archive 9eac657 into `base/pagerdb-diag`, copy `repo` to
`base/repo-diag`, apply `../cloud-c8id-r3/instrument.py`, use `base/go.work`, then run the two TestMatcher commands
and the BenchmarkEVM command above.

Applier experiments on synthetic (`raw/exp-*-synthetic.txt`), copies of the b8ba9ec instrumented workspace with
one line changed in `disk.go`:

```sh
# exp-defer: maxApplyLag = 1 << 40 and applyInterval = time.Hour (applier runs only at the final Checkpoint)
# exp-i5s:   applyInterval = 5 * time.Second
```

## raw/earlier

Results from the intermediate commits on the same box: `evm-12b7bbe.txt` (first version, before the redo fixes),
`evm-180fcdd-run1..3.txt` (run 1 was an outlier at 12.6M sigma 1%, runs 2 and 3 are at 16.6M to 16.9M),
`core-checks-*.txt` and `race-12b7bbe.txt` (the 12b7bbe race run first failed for missing cgo, then passed),
`mem-after-*-12b7bbe.txt` and `mem-after-*-180fcdd.txt` (msync builds: RssFile holds about 1x state of clean,
reclaimable image pages), `swap-*-180fcdd.*` (msync build: 1,720,021 writes/s capped, the kernel kept mapped image
pages and swapped state), `exp-noapply-original-180fcdd.txt` (applier deferred), `original-b8ba9ec-first.txt`,
`host-checks-12b7bbe-diag.txt` and `pinned-180fcdd-*.txt`.

The CPU profiles named in the report (sigma 1% on 12b7bbe, sigma 0.1% on 88753dc) were read on the box with
`go tool pprof -top` and not kept.
