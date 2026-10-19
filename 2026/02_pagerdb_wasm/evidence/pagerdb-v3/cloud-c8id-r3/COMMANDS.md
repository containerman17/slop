# PagerDB v3 on c8id.8xlarge

Host archive: `7a3b5e1`, subtree `2026/02_pagerdb_wasm`.
PagerDB archive: `9eac65770c3d49fffce31873269e2f1f87fbc5bd`, branch `v3`.
One c8id.8xlarge in us-west-2a, Ubuntu 24.04 amd64, 40 GiB gp3 root deleted on termination. Instance, root volume and network interface carry the required ownership tags. Launch request and response remain outside the repository. Boot user-data runs `shutdown -h +180`; instance shutdown behavior is `terminate`.

Launch: 2026-10-15 19:33:52 JST. Boot scheduled shutdown: 22:34:08 JST.

The instance-store device was verified as Amazon EC2 NVMe Instance Storage before formatting `/dev/nvme1n1` as ext4 and mounting at `/mnt/nvme`. No other instance was accessed or changed. Existing SSH access worked, so no security-group changes were needed.

```sh
# Local, run archive from repository root. Stream directly to the box.
git -C ~/slop archive 7a3b5e1:2026/02_pagerdb_wasm | SSH 'tar -xf - -C /mnt/nvme/repo'
git -C ~/.herdr/worktrees/pagerdb/v3 archive 9eac657 | SSH 'tar -xf - -C /mnt/nvme/pagerdb'
SSH 'cat > /mnt/nvme/original.bin' < ~/temp-1m-clob/bench/fixture.bin
```

`SSH` above denotes the private local SSH helper, with batch mode, forwarding disabled and no multiplexing. Connection settings remain outside the repository. No extra local fixture copies were made.

Installed build-essential, perl, pkg-config, Go 1.26.0 and rustup stable (Rust 1.99.0). The instrumenter and native oracle use their locked Cargo dependencies. All database and compiler temporary files use NVMe:

```sh
export PATH=/usr/local/go/bin:$HOME/.cargo/bin:$PATH
export TMPDIR=/mnt/nvme/tmp GOMAXPROCS=16 GOCACHE=/mnt/nvme/go-cache
export GOPATH=/mnt/nvme/go CARGO_TARGET_DIR=/mnt/nvme/cargo-target TZ=Asia/Tokyo
cp -a /mnt/nvme/repo /mnt/nvme/repo-diag
cp -a /mnt/nvme/pagerdb /mnt/nvme/pagerdb-diag
python3 /mnt/nvme/instrument.py /mnt/nvme/pagerdb-diag /mnt/nvme/repo-diag
gofmt -w /mnt/nvme/pagerdb-diag/{pagerdb.go,disk.go,evm_bench_test.go} /mnt/nvme/repo-diag/matcher_test.go
cd /mnt/nvme
go work init ./repo-diag ./pagerdb-diag
cd /mnt/nvme/repo-diag
go run ./cmd/fixture 600 > /mnt/nvme/synthetic.bin
go vet ./...
taskset -c 0-15 go test -count=1 -timeout=20m ./...
PAGERWASM_FIXTURE=/mnt/nvme/original.bin taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v -timeout=20m .
PAGERWASM_FIXTURE=/mnt/nvme/synthetic.bin taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v -timeout=20m .
cd /mnt/nvme/pagerdb-diag
PAGERDB_EVM_RETAIN=32 taskset -c 0-15 go test -run '^$' -bench '^BenchmarkEVM$' -benchtime=700x -count=2 -timeout=20m
```

Each TestMatcher invocation performs two replays. EVM root collection preserves execution/hash overlap: after submitting block N, it collects root N-1; the final root follows Flush. The second benchmark repetition compares every root, and both repetitions call Verify after the timer stops. Instrumentation is fully reproduced by instrument.py; it changes no production source in either repository and adds no go.mod replace directive.

Timer boundaries: host execution surrounds Program.Run; Commit-return surrounds Commit. Wall time starts before the first fixture block read and ends after final RootAt and Flush. It excludes deployment, oracle generation, and the independent final Verify. Hash time surrounds the background tree update, including SHA-256 and tree folding. Journal append includes record allocation, copy, CRC, rotation, and write; journal write surrounds only File.Write. MB/s uses decimal MB divided by summed File.Write time, not total elapsed time or physical device throughput. Image-copy time measures the disk worker applying snapshots and marking changed leaves. Minor faults are process-wide getrusage deltas across the wall interval. Background phase times overlap and must not be added to execution.

The host keeps its existing RetainVersions=2; EVM uses 32. Default CheckpointInterval=1000 is unchanged. Genesis checkpoint and deployment are excluded from measured counters. These 600/700-block workloads do not reach an automatic checkpoint. Flush drains writes but does not fsync the journal. Recovery is to the last checkpoint, not the latest Commit.

CPU profile (run if original wall throughput is below 1.9M messages/s):

```sh
cd /mnt/nvme/repo-diag
V3_CPU_PROFILE=/mnt/nvme/results/v3.cpu PAGERWASM_FIXTURE=/mnt/nvme/original.bin \
  taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v -timeout=20m .
go tool pprof -top -nodecount=60 /mnt/nvme/results/v3.cpu
go tool pprof -top -cum -nodecount=60 /mnt/nvme/results/v3.cpu
go tool pprof -list 'pagerdb.*(snapshot|copyLeaf)' /mnt/nvme/results/v3.cpu
go tool pprof -list 'pagerdb.*append' /mnt/nvme/results/v3.cpu
cd /mnt/nvme/pagerdb-diag
go vet ./...
taskset -c 0-15 go test -race -count=1 -timeout=10m ./...
# Also verify both unchanged source archives, without the diagnostic workspace:
cd /mnt/nvme/repo
GOWORK=off go vet ./...
GOWORK=off taskset -c 0-15 go test -count=1 -timeout=20m ./...
cd /mnt/nvme/pagerdb
GOWORK=off go vet ./...
GOWORK=off taskset -c 0-15 go test -count=1 -timeout=10m ./...
```

Profiling covers only the first replay's timed loop and final drain. The second replay still checks the complete root sequence. Only profile text summaries are retained in this repository.

Teardown used `sudo shutdown -h now` on this instance only, followed by an AWS `describe-instances` query restricted to its instance ID. AWS reported `terminated`; exact JST times and the verification command are in teardown.txt. The temporary root volume was configured DeleteOnTermination. No security-group cleanup was required.

Additional CPU attribution from the same profile: `go tool pprof -list 'pagerdb.Open.func2' /mnt/nvme/results/v3.cpu` (snapshot copy callback) and `go tool pprof -list 'pagerdb.*touch' /mnt/nvme/results/v3.cpu`.

EVM p99-write-ms is the 99th percentile of the 700 per-block write durations, each covering 50,000 operations. It is not a percentile of individual writes.
