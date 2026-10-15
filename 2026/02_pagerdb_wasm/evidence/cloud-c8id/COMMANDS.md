# Cloud reproduction

Source: origin/main `840d09dd9a2650c1a427dfdbb49e1783f8e2d080`.
Run `git archive origin/main` from `2026/02_pagerdb_wasm` to archive that subtree.
Stream archive to `/mnt/nvme/repo` and original fixture directly to
`/mnt/nvme/original.bin`. Generate synthetic fixture on the instance.

One c8id.8xlarge in us-west-2a, Ubuntu 24.04 AMI `ami-03d70b8354a16a8bc`.
Launch settings are kept outside the repo in the private coordinator cache. User data: `#!/bin/bash` then `shutdown -h +180`.
Instance shutdown behavior: terminate. Root volume: 40 GiB gp3, delete on termination.
Owner tag on instance, volume and ENI. Existing SSH /32 rule already matched;
no security group change. Format `/dev/nvme1n1` as ext4 and mount `/mnt/nvme`.
Install build-essential, perl, pkg-config, Go 1.26.0 and rustup stable (1.99.0).
No toolchain file exists in the instrumenter; Cargo.lock controls dependencies.
Use existing checked-in Wasm artifacts without rebuilding them.

```sh
export PATH=/usr/local/go/bin:$HOME/.cargo/bin:$PATH
export TMPDIR=/mnt/nvme/tmp GOMAXPROCS=16
export GOCACHE=/mnt/nvme/go-cache GOPATH=/mnt/nvme/go
export CARGO_TARGET_DIR=/mnt/nvme/cargo-target TZ=Asia/Tokyo
cd /mnt/nvme/repo
taskset -c 0-15 go vet ./...
taskset -c 0-15 go test -count=1 -v -timeout=45m ./...
taskset -c 0-15 go run ./cmd/fixture 600 > /mnt/nvme/synthetic.bin
PAGERWASM_FIXTURE=/mnt/nvme/original.bin PAGERWASM_DIAGNOSTICS=1 \
  taskset -c 0-15 go test -run '^TestDiagnostics$/(matcher.wasm|inline-stack.wasm)$' -count=2 -v -timeout=20m .
PAGERWASM_FIXTURE=/mnt/nvme/original.bin \
  taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
PAGERWASM_FIXTURE=/mnt/nvme/synthetic.bin \
  taskset -c 0-15 go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
PAGERWASM_FIXTURE=/mnt/nvme/original.bin PAGERWASM_PROFILE=/mnt/nvme/commit.cpu \
  taskset -c 0-15 go test -run '^TestCommitProfile$' -count=1 -v -timeout=30m .
GOMAXPROCS=8 PAGERWASM_FIXTURE=/mnt/nvme/original.bin \
  taskset -c 0-7 go test -run '^TestMatcher$/0$' -count=1 -v -timeout=30m .
```

TestMatcher runs twice internally. The 8-core command selects only run 0;
this checks native output hashes and returned counts, but has no second root replay.
Fresh DB per replay; no cache dropping. All DBs and temporary files are on NVMe.
The hard-coded text `M3 diagnostics` in raw test logs is an existing label,
not the machine used here. Host identity is in host.txt.

```sh
go tool pprof -top -cum -nodecount=40 -tagfocus=stage=commit /mnt/nvme/commit.cpu
go tool pprof -top -cum -nodecount=40 -focus='pagerdb.*seal' /mnt/nvme/commit.cpu
go tool trace -pprof=sync /mnt/nvme/commit.cpu.trace > /mnt/nvme/sync.pprof
go tool trace -pprof=syscall /mnt/nvme/commit.cpu.trace > /mnt/nvme/syscall.pprof
go tool pprof -top -cum -nodecount=30 -focus='pagerdb[.]\(\*DB\)[.]Commit' /mnt/nvme/sync.pprof
go tool pprof -top -cum -nodecount=30 -focus='pagerdb[.]\(\*DB\)[.]Commit' /mnt/nvme/syscall.pprof
```

## Additional diagnostics

The coordinator extended the run to tracking size, leaf size, sealing, input
journaling and overlap diagnostics, and requested waiting for automatic shutdown.
`cloud_diagnostic_test.go` is opt-in and defaults to the unchanged production host.
It uses the same native oracle and two-run root/count checks as TestMatcher.
It reads the pinned DB's current log cursor through reflection before and after
Run, outside the timer; Run does not rotate. Journal bytes include the exact live
entry bytes plus the 56-byte Commit record, but exclude epoch checkpoints and
compressed sealed files. getrusage snapshots bracket execution outside its timer.
They include all process threads, including background sealer and prefaulter.
CSV writes occur after Commit, outside both timers, and add a small pacing cost.

Copy the archived source to `/mnt/nvme/diag64`, then add the diagnostic test.
The baseline diagnostic uses `PAGERWASM_CLOUD=1`,
`PAGERWASM_CLOUD_CSV=/mnt/nvme/results/diag64.csv`, original fixture,
and `go test -run '^TestCloudDiagnostics$' -count=1 -v -timeout=30m .`.
Use the same NVMe environment and 16-core pinning as above. `perf stat` wraps
that complete test command with events cycles,instructions,branches,branch-misses,
cache-misses,page-faults,minor-faults,major-faults; sudo is required. Perf counts
include compilation, native oracle, deployment, cleanup and all subprocesses,
so they are not execution-only counts. The zero cache-miss event is unvalidated.

The existing barrier decomposition runs unchanged from `/mnt/nvme/repo`:

```sh
PAGERWASM_FIXTURE=/mnt/nvme/original.bin PAGERWASM_BARRIER_DIAGNOSTICS=1   taskset -c 0-15 go test -run '^TestBarrierDiagnostics$' -count=2 -v -timeout=30m .
```

These deliberately unsafe controls check native hashes; only the full mode
commits, and this existing control does not compare root sequences. The separate
full-host pairs provide the root checks. Bulk callbacks remain present in the
stub and empty scalar-callback controls, so their differences are not a pure
single-instruction cost.

Tracking variants are box-local copies of diag64 at `/mnt/nvme/diag1024` and
`/mnt/nvme/diag4096`. Set host.go's lineSize to 1024 or 4096 and change both
I64Const(6) address-to-line shifts in instrument/src/main.rs to 10 or 12.
Replace finalization's one-leaf calculation with a loop from
`line*lineSize/pagerdb.LeafSize` through `((line+1)*lineSize-1)/pagerdb.LeafSize`,
counting distinct leaves in sorted order. The new diagnostic test uses this same
range to verify counts. Change the unused scratch diagnostic's divide-by-ratio
expression to multiply-then-divide so the 4096-byte copy compiles; do not use
that old scratch diagnostic for these rows. Keep the bitmap reservation at 1 MiB.

Use a separate CARGO_TARGET_DIR for each source variant. The first 4096-byte
attempt shared the earlier target and failed at block zero; its instrumenter
binary still had the previous build timestamp. Rebuilding in `/mnt/nvme/target4096`
fixed it. The failed attempt is preserved and excluded. No source fixture is
inside these copies; all copies point at the same `/mnt/nvme/original.bin`.

The hash-leaf variant `/mnt/nvme/diag-leaf4096` retains 64-byte tracking and uses
its own Cargo target. Copy the pinned PagerDB module cache source to a writable
box-local directory and set only its LeafSize constant from 1024 to 4096.
A box-local go.work includes the diagnostic host directory and that PagerDB
copy. `go list -m` confirms workspace selection. No go.mod replace directive,
PagerDB patch file or workspace file is committed. Native outputs must remain
the same; roots are compared between repeats of this variant, not against the
1 KiB leaf format.

For no-seal, run the unchanged 64-byte diagnostic with
`PAGERWASM_CLOUD_NO_SEAL=1`. It sets EpochVersions=1000000 and EpochBytes=1<<40,
and checks that the epoch number did not advance during the 600 blocks.
One invocation retains both internal replays to preserve the root comparison.
Final DB.Close is outside both throughput timers; this is a deferred-work
experiment, not a proposal to omit sealing in production.

Input-scratch uses a separate 64-byte copy `/mnt/nvme/diag-scratch`. Only the
input import changes: instead of `p.write(offset, p.args.Input)`, it copies into
`p.read(offset, uint32(len(p.args.Input)))` directly. Bounds and input copy remain;
DB.Touch and bitmap marking for that ABI write are skipped. Subsequent guest
stores still use the normal barrier. This is an unsafe bypass, not a scratch
memory architecture: input remains inside mapped state. Repeated roots can be
deterministic despite untracked writes. Undo, recovery and complete state-root
correctness are not established by this experiment. Run the same opt-in
TestCloudDiagnostics pair and retain all native hash and repeat-root checks.

Fault investigation used `perf record -e minor-faults -c 100 -g` for call stacks,
then `perf record -d --data-mmap -e minor-faults -c 100` for addresses. Both wrap
the 64-byte diagnostic pair with `go test -work` so binary metadata remains on
the instance for analysis. `perf report --mem-mode --stdio --sort dso_daddr`
classifies faulted data objects. Only text reports are retained. An initial
`--mmap-data` spelling was rejected before running a test; the installed perf
option is `--data-mmap`. That error is retained in `perf-option-error.txt`.

The final writeback isolation pair adds PAGERWASM_CLOUD_NO_SEAL=1 and temporarily
sets vm.dirty_background_bytes=34359738368, vm.dirty_bytes=51539607552,
vm.dirty_expire_centisecs=360000, vm.dirty_writeback_centisecs=0. Original settings
were saved and restored. Direct writes of zero to the bytes fields were rejected;
restoring their ratio counterparts cleared them to zero. A final diff of all six
values returned zero, as recorded in `writeback-restore-verified.txt`.

Text outputs preserve the tool content; trailing whitespace and final blank lines are normalized for git.
