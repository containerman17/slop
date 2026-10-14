# PagerDB Wasm prototype

A Go host for integer Wasm with linear memory backed directly by a PagerDB region.
Wazero v1.12.0 uses `experimental.WithMemoryAllocator`; the guest and `DB.Map` share
the same bytes. PagerDB is a normal module dependency pinned to `af2c4c9`.

```go
ctx := context.Background()
host, err := pagerwasm.New(ctx, db)
// Check err; close host before db.
program, err := host.Deploy(ctx, wasmBytes)
// Check err; commit deployment before calls.
_, _, err = db.Commit()
result := program.Run(ctx, pagerwasm.Call{Input: input, Gas: 1 << 40})
// Status zero means success. Nonzero status has already reverted the call.
_, _, err = db.Commit()
```

One goroutine owns the host, programs and database. Deployment creates a region;
there is no API for reopening existing programs, historical execution, cross
calls, concurrent calls, or upgrades.

## Execution contract

Imports belong to `env`: `block_time() i64`, `block_number() i64`, `gas_left() i64`,
`caller(i32)` (20 bytes), `value(i32)` (32 bytes), `call_type() i32`,
`emit_log(i32,i32)`, `input(i32)`, `input_len() i32`, and `output(i32,i32)`.
Export `call() i32`: zero succeeds, any other value reverts. The instrumenter
adds `touch(i32)`; source modules may not import it. Trap status is `0xffffffff`;
trap messages are not returned. Reverts discard logs. Explicit status reverts
keep output; traps discard output. Returned output and logs are owned copies.

`Call` contains all block and transaction values. No WASI, clock, randomness or
other ambient host input is available. Floats (including types), SIMD, threads,
shared memory, bulk memory and post-MVP features other than sign extension are
rejected. Start functions and non-function imports are rejected. Guest mutable
globals must return to their entry values; a call that changes them reverts.
Persistent guest state therefore belongs in linear memory. Globals and the
injected stack counter are restored after every revert or trap.

A source module must declare a maximum of at most 512 MiB. Deployment fixes memory
at that maximum and reserves an additional 1 MiB for the bitmap. Guest
`memory.size` returns the usable size; `memory.grow` returns -1. Every integer
store checks its complete effective address, including offset and width, before
writing. Stores that cross a 64-byte boundary touch both lines. Overflowing addresses
and stores into reserved space trap. Guest loads of reserved bytes are allowed.
Host writes also go through line tracking. `DirtyLines` reports lines written,
including writes that store the same bytes, rather than lines whose final bytes
changed. It also reports touched lines for reverted calls.

Each store tests the bitmap inline. Only the first store to a line calls `touch`.
The host records old bytes with `DB.Touch`, records the line, then sets its bit.
The bitmap is never touched in PagerDB. Every exit path clears exactly the
recorded bits before undo or commit, so the bitmap contributes only zeros to the
root. A crash during a call may leave unlogged bitmap bytes; PagerDB recovery can
detect a root mismatch and require resync, consistent with its crash model.

The host marks before execution and undoes on a status revert, trap or gas
failure. It does not commit calls. `InstructionGas`, `HostGas`, `DirtyLineGas`
and `StackLimit` live in `host.go`. The existing wasm-instrument algorithm meters
blocks and locals (including its historical fixed gas-helper surcharge). The helper
is inlined before the stack pass, so no gas function call remains. Host imports
have a fixed charge, and dirty-line gas is charged at call end. Exhaustion at any
of these points reverts. Prices are prototype constants, not calibrated costs.

## Deployment and build shortcuts

Deployment calls a local Rust tool and caches the compiled result by the original
code's SHA-256 hash for the lifetime of the Host. It uses wasm-instrument 0.4.0
for gas and stack-height checks, plus an integer-store rewrite. This is one deploy
pipeline with separate internal passes, not a fused single traversal. The source
tree and Cargo must be available at deploy time. Cargo dependencies are locked.
The subprocess boundary and source-path lookup are prototype shortcuts.

`instrument/src/main.rs` extends the existing metering experiment; it reuses the
crate algorithm rather than rewriting it. The matcher crate under
`testdata/matcher/matcher` reuses the existing experiment. Its matching algorithm
is unchanged; one native-only quote accessor supports shape measurement. Its
Wasm adapter changes: one `call()` reads input, runs the matcher and returns
output. Its fixed maximum is 512 MiB. Rust's precompiled standard library can emit
bulk-memory instructions despite compiler flags, so the matcher build lowers
those instructions using Binaryen before deployment. The deployed host does not
support bulk memory. Prebuilt test Wasm files are included.

To rebuild test artifacts (Rust with `wasm32-unknown-unknown`, Binaryen required):

```sh
export PATH="$HOME/.cargo/bin:$PATH"
./testdata/matcher/build.sh
sh testdata/build-cases.sh
```

## Verification

```sh
go vet ./...
go test -count=1 -timeout=45m ./...
```

The default matcher test generates 20 blocks of 50,000 messages with seed 1.
A native build of the same matcher computes the expected output hash for every
block. Wasm runs the fixture twice, commits after every block, and compares both
root sequences. Set `PAGERWASM_BLOCKS=600` for the full synthetic workload. Set
`PAGERWASM_FIXTURE=/absolute/path/to/fixture.bin` for an optional local fixture;
only an explicitly requested absent fixture is skipped. Source data and its
precomputed hashes are not included. Cargo is required by deployment and native
oracle tests, even when Wasm artifacts have already been built.

`internal/fixture/generate.go` contains the seeded generator and a few measured
aggregate parameters. `go run ./cmd/fixture 600 > /tmp/synthetic.bin` writes the
CLOB1 format: a 16-byte header followed by 40-byte little-endian records. These
records are synthetic; their identifiers, symbol numbers and prices do not map
to source data. This approximates the source workload shape, not its exact joint
distribution or time evolution. In particular, generated blocks touch about
4.1 times as many symbols as source blocks; performance is reported separately. Native tooling reports aggregate shape:

```sh
cargo run --locked --release --manifest-path testdata/matcher/Cargo.toml \
  -p matcher-tools -- shape /tmp/synthetic.bin
```

Focused tests cover shared-memory pointer identity, status/trap/gas reverts,
end-of-call line gas, line-boundary stores, address overflow, stack exhaustion,
reserved bitmap writes, bitmap root neutrality, import/feature rejection, and
code-cache reuse. Dirty-line correctness is checked against a full byte comparison
on the small store fixture. The matcher does not copy its whole memory each block.

For Mac diagnostics, take the shared benchmark lock first and release it afterward:

```sh
until mkdir ~/.cache/pagerdb-bench.lock 2>/dev/null; do sleep 20; done
PAGERWASM_BLOCKS=600 PAGERWASM_DIAGNOSTICS=1 go test -run '^TestDiagnostics$' -count=1 -v .
PAGERWASM_BLOCKS=600 go test -run '^TestMatcher$' -count=1 -v -timeout=45m .
rmdir ~/.cache/pagerdb-bench.lock
```

The diagnostic controls run the same rebuilt matcher with ordinary anonymous
memory: plain, helper gas, inline gas, stack only, and each gas mode with stack
checks. They check every output hash against the native matcher. The block count follows
the fixture settings above. They do not commit. Comparing metered control execution to host execution
measures the barrier plus PagerDB Touch, line tracking and end-of-call work, not
just the inline bit test. The historical plain wazero baseline used another
module build; it is context, not an isolated measurement of barrier overhead.

Opt-in `TestCommitProfile` adds CPU labels and a Go execution trace without
changing PagerDB. Set `PAGERWASM_PROFILE` to an external output path; optionally
set `PAGERWASM_PROFILE_START=580` with a 600-block fixture to capture the last 20
blocks. It also compares a snapshot with the last block's final bytes to quantify
page write amplification. See `evidence/commands.md` for exact commands. CPU time
across workers and syscall/wait durations are reported separately.

The performance round keeps inline instruction metering and fixed 64-byte write
tracking. Both original and generated fixtures improved on execution plus Commit
versus 1 KiB tracking. See [the measured stages](evidence/round2.md) for every row,
Commit mean and p99, log accounting, correctness checks, and limits. The earlier
4 KiB figures in the evidence directory describe the previous implementation.

Opt-in `TestBarrierDiagnostics` separates the inline barrier, miss callback and
PagerDB Touch with unsafe diagnostic controls. Set `PAGERWASM_BARRIER_DIAGNOSTICS=1`
and use the same fixture and Mac lock rules. See [the barrier diagnosis](evidence/round3.md)
for control definitions, CPU profiling, stack/heap counts and proposed changes.
