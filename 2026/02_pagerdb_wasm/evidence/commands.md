# Commands and scope

All runs: Apple M3, macOS arm64, single process per measured run, shared lock
`~/.cache/pagerdb-bench.lock` held. External system load is uncontrolled.
Times and rates exclude fixture generation, native expected hashes, deployment,
output hashing, database close and final sealing drain. The host execution rate
includes input/output copies, metering, stack limits, barriers, Touch, bitmap
cleanup, dirty-page sorting, and gas accounting. Commit timing is separate.

```sh
cd ~/slop/2026/02_pagerdb_wasm
until mkdir ~/.cache/pagerdb-bench.lock 2>/dev/null; do sleep 20; done

go vet ./...
go test -count=1 -v -timeout=45m ./... > evidence/tests.txt 2>&1

# Source fixture stays outside the repository. The initial source run compared
# bench/golden.txt before that file was removed from this prototype.
PAGERWASM_FIXTURE=~/temp-1m-clob/bench/fixture.bin \
  go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
PAGERWASM_FIXTURE=~/temp-1m-clob/bench/fixture.bin PAGERWASM_DIAGNOSTICS=1 \
  go test -run '^TestDiagnostics$' -count=1 -v -timeout=10m .

# Generate synthetic data outside the repository, then measure its shape.
go run ./cmd/fixture 600 > ~/.cache/cx/wasmhost.synthetic600.bin
cargo run --quiet --locked --release --manifest-path testdata/matcher/Cargo.toml \
  -p matcher-tools -- shape ~/.cache/cx/wasmhost.synthetic600.bin
cargo run --quiet --locked --release --manifest-path testdata/matcher/Cargo.toml \
  -p matcher-tools -- shape ~/temp-1m-clob/bench/fixture.bin
PAGERWASM_FIXTURE=~/.cache/cx/wasmhost.synthetic600.bin \
  go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
PAGERWASM_FIXTURE=~/.cache/cx/wasmhost.synthetic600.bin PAGERWASM_DIAGNOSTICS=1 \
  go test -run '^TestDiagnostics$' -count=1 -v -timeout=10m .

rmdir ~/.cache/pagerdb-bench.lock
```

`original-matcher.txt`: 600 source blocks, two runs, all precomputed source hashes
and every root sequence entry checked. This run preceded the added restriction
that guest globals must be scratch; the matcher leaves its stack-pointer global
unchanged on success. `synthetic-matcher.txt` uses the final host and native oracle.
`original-controls.txt` and `synthetic-controls.txt`: plain and gas/stack controls
on anonymous memory; all native expected output hashes checked. `tests.txt` is the
final full default suite. The optional diagnostics test skips in the default suite.

The historical 2026-10-12 plain wazero figure is 7.83M messages/s (600 blocks,
median of three, Rust block mode). It used a different Wasm build. The rebuilt
same-module controls are the useful denominator for this prototype's overhead.

Toolchain recorded 2026-10-14 23:31 JST: Go 1.26.0 darwin/arm64;
Rust 1.99.0 (b940084d7 2026-09-28); Binaryen wasm-opt 133;
macOS 27.0.1 (26A434), arm64. Test Wasm artifacts are committed; Rust and
Binaryen compiler versions are recorded here but are not enforced by a toolchain
manager. Host wazero and Rust instrumentation dependencies are pinned separately.

Commit attribution (run each command sequentially while holding the same lock):

```sh
PAGERWASM_FIXTURE=~/.cache/cx/wasmhost.synthetic600.bin \
  PAGERWASM_PROFILE_START=580 PAGERWASM_PROFILE=~/.cache/cx/wasmhost.synthetic-mature.cpu \
  go test -run '^TestCommitProfile$' -count=1 -v -timeout=15m .
PAGERWASM_FIXTURE=~/temp-1m-clob/bench/fixture.bin \
  PAGERWASM_PROFILE_START=580 PAGERWASM_PROFILE=~/.cache/cx/wasmhost.original-mature.cpu \
  go test -run '^TestCommitProfile$' -count=1 -v -timeout=15m .
go tool pprof -top -cum -tagfocus=stage=commit PROFILE.cpu
go tool trace -pprof=sync PROFILE.cpu.trace > /tmp/commit-sync.pprof
go tool trace -pprof=syscall PROFILE.cpu.trace > /tmp/commit-syscall.pprof
go tool pprof -top -cum -focus='pagerdb[.][(][*]DB[)][.]Commit' /tmp/commit-sync.pprof
go tool pprof -top -cum -focus='pagerdb[.][(][*]DB[)][.]Commit' /tmp/commit-syscall.pprof
```

`PAGERWASM_PROFILE_START` defaults to zero; the early profiles used the first 20
blocks. Mature profiles warm up 580 blocks and capture only blocks 580 through
599. CPU labels separate execution from Commit and propagate to parallel Commit
workers. The sealer is an independent goroutine and has no Commit label. Trace
profiles identify syscall duration and synchronization waits on Commit's stack.
CPU ms across workers cannot be added to elapsed ms. Profiles add measurement
overhead and run under uncontrolled background load. Full-region copying for the
last block's byte comparison is outside its execution and Commit timers.

The symbol-density scan streams 2 MB blocks, loads each as little-endian u16
values, and counts `set(words[1::20])` per block (symbol is at byte offset 2 of
each 40-byte record). On this little-endian Mac, reproduce with Python's
`array.array('H')`, `frombytes(block)`, and that slice. Read the block dimensions
with `struct.unpack('<II', header[8:16])`. The sorted 600 counts at indices
299/539/593 give the reported p50/p90/p99. `symbol-density.txt` stores only these
aggregate counts, not symbol values.

## Round 2

Hold the same lock and run measured commands sequentially. Controls now separate
plain, helper-only, stack-only, helper+stack, inline-only and inline+stack modes.
The full-host test also reports log record bytes, dirty 1 KiB leaf coverage and
Commit p99. `PAGERWASM_DIAGNOSTICS=1` enables the last-block full-memory comparison.

```sh
PAGERWASM_DIAGNOSTICS=1 PAGERWASM_FIXTURE=~/temp-1m-clob/bench/fixture.bin \
  go test -run '^TestDiagnostics$' -count=1 -v -timeout=30m .
PAGERWASM_DIAGNOSTICS=1 PAGERWASM_FIXTURE=~/.cache/cx/wasmhost.synthetic600.bin \
  go test -run '^TestDiagnostics$' -count=1 -v -timeout=30m .
PAGERWASM_DIAGNOSTICS=1 PAGERWASM_FIXTURE=~/temp-1m-clob/bench/fixture.bin \
  go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
PAGERWASM_DIAGNOSTICS=1 PAGERWASM_FIXTURE=~/.cache/cx/wasmhost.synthetic600.bin \
  go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
```

Each full-host command checks 600 native hashes on two fresh databases and
compares all 600 roots. `evidence/round2.md` records the measured stages.
