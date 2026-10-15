# PagerDB Wasm prototype

A Go host for integer Wasm with linear memory backed directly by a PagerDB region.
Wazero v1.12.0 uses `experimental.WithMemoryAllocator`; the guest and `DB.Map` share
the same bytes. PagerDB is a normal module dependency pinned to the v3 branch commit in `go.mod`.

```go
ctx := context.Background()
host, err := pagerwasm.New(ctx, db)
// Check err; close host before db.
program, err := host.Deploy(ctx, wasmBytes)
// Check err; commit deployment before calls.
_, err = db.Commit()
result := program.Run(ctx, pagerwasm.Call{Input: input, Gas: 1 << 40})
// Status zero means success. Nonzero status has already reverted the call.
_, err = db.Commit()
```

One goroutine owns the host, programs and database. Deployment creates a region;
there is no API for reopening existing programs, historical execution, cross
calls, concurrent calls, or upgrades.

## Execution contract

Imports belong to `env`: `block_time() i64`, `block_number() i64`, `gas_left() i64`,
`caller(i32)` (20 bytes), `value(i32)` (32 bytes), `call_type() i32`,
`emit_log(i32,i32)`, `input(i32)`, `input_len() i32`, and `output(i32,i32)`.
Export `call() i32`: zero succeeds, any other value reverts. The instrumenter
adds `touch(i32)` and `touch_range(i32,i32)`; source modules may not import them. Trap status is `0xffffffff`;
trap messages are not returned. Reverts discard logs. Explicit status reverts
keep output; traps discard output. Returned output and logs are owned copies.

`Call` contains all block and transaction values. No WASI, clock, randomness or
other ambient host input is available. Floats (including types), SIMD, threads,
shared memory and post-MVP features other than sign extension and
`memory.copy`/`memory.fill` are rejected. Other bulk operations, including
`memory.init`, `data.drop` and table operations, remain forbidden. Start functions and non-function imports are rejected. Guest mutable
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
The host records old bytes with `DB.Touch`, then sets the bit. ABI writes coalesce
consecutive untouched lines into one Touch range, stopping at already dirty lines.
The bitmap is never touched in PagerDB. Every exit path scans the bitmap in
address order to produce sorted `DirtyLines`,
counts unique written bytes and touched 4 KiB leaves, charges state gas, and clears the bits before undo or commit. No dirty-line
sort is needed, and the bitmap contributes only zeros to the root. A crash during
a call discards live anonymous memory. PagerDB opens the last complete checkpoint;
the caller must re-execute later blocks.

The host marks before execution and undoes on a status revert, trap or gas
failure. It does not commit calls. `InstructionGas`, `HostGas`, `ByteGas`, `LeafGas`,
`BulkByteGas` and `StackLimit` live in `host.go`. The existing wasm-instrument
algorithm meters
blocks and locals (including its historical fixed gas-helper surcharge). The helper
is inlined before the stack pass, so no gas function call remains. Host imports
have a fixed charge. At call end, state gas is `BytesWritten * ByteGas +
LeavesTouched * LeafGas`: six gas per unique written byte (64 bytes per dirty
line), plus 2,000 gas per distinct 4 KiB leaf. Both counts are returned, including
on reverts. Repeated writes do not add state gas. Bulk copy/fill
also charge `BulkByteGas` per byte through an inline unsigned length check and
decrement. Each bulk operation calls the injected range barrier once before
execution. The host touches consecutive unset destination lines. Copy source
and destination bounds are checked before any destination is journaled; sums
use 64-bit arithmetic, and zero-length operations still check their endpoints.
Exhaustion at any of these points reverts. Prices are prototype constants, not calibrated costs.

## Deployment and build shortcuts

Deployment calls a local Rust tool and caches the compiled result by the original
code's SHA-256 hash for the lifetime of the Host. It uses wasm-instrument 0.4.0
for gas and stack-height checks, plus integer-store and bulk-operation rewrites.
This is one deploy pipeline with separate internal passes, not a fused single traversal. The source
tree and Cargo must be available at deploy time. Cargo dependencies are locked.
The subprocess boundary and source-path lookup are prototype shortcuts.

`instrument/src/main.rs` extends the existing metering experiment; it reuses the
crate algorithm rather than rewriting it. The matcher crate under
`testdata/matcher/matcher` reuses the existing experiment. Its matching algorithm
is unchanged; one native-only quote accessor supports shape measurement. Its
Wasm adapter changes: one `call()` reads input, runs the matcher and returns
output. Its fixed maximum is 512 MiB. Rust's precompiled standard library can emit
bulk-memory instructions despite compiler flags. The matcher build now keeps
copy/fill and uses Binaryen only to normalize padded linker immediates, with no
lowering pass. Prebuilt test Wasm files are included.

`instrument/vendor/wasm-instrument` preserves the pinned 0.4.0 source and licenses
with a narrow bulk stack-effect patch; see its `PATCH.md`. Enabling parity-wasm
bulk support alone otherwise fails to compile that crate. The deploy pipeline
removes the unused DataCount section after validation because copy/fill do not
reference data segments, avoiding the old builder's section-ordering problem.

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
end-of-call byte and leaf gas, line-boundary stores, address overflow, stack exhaustion,
reserved bitmap writes, bitmap root neutrality, bulk copy/fill gas and bounds,
overlapping copies, zero length, import/feature rejection, and
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

The final barrier changes scan the bitmap instead of sorting and coalesce ABI
Touch ranges. See [round 4 results](evidence/round4.md) for the original fixture
replays and the before/after end-work and Touch measurements.

Bulk copy/fill support removes the matcher byte-loop lowering. See
[round 5 evidence](evidence/round5.md) for both fixtures, plain and metered controls,
full-host replays and the comparison with round 4.

`TestScratchDiagnostics` is an opt-in snapshot diagnostic. With the shared lock
held, set `PAGERWASM_SCRATCH_DIAGNOSTICS=1` and `PAGERWASM_FIXTURE` to the original
fixture, then run `go test -run '^TestScratchDiagnostics$' -count=1 -v -timeout=30m`.
It checks every native hash, compares every dirty line with the pre-call snapshot,
and observes ABI ranges and stack frames through test-only wazero listeners.
The journal estimate removes unchanged lines and lines overlapping input/output
ranges, while preserving existing Touch boundaries. This is a hypothetical saving,
not proof that those bytes can be zeroed safely. The test pins the matcher hash
because its static/heap boundary is specific to this build. See `evidence/round7.md`.


## PagerDB v3 branch

PagerDB uses 4 KiB commitment leaves and anonymous state memory. State gas counts
these 4 KiB leaves; 64-byte first-write tracking is unchanged. Commit returns a
version after copying dirty leaves. RootAt(version) waits for hashing, and
CommitWait retains the previous blocking shape. TestMatcher submits each block
and checks the previous root after executing the next block, then drains the final
root and journal and independently verifies the state. Its timing logs are
correctness diagnostics, not a new performance claim.

The old TestCloudDiagnostics source depended on v2 private log fields and its
sealer. It is removed on this branch; historical evidence and the source at
b9c17c9 remain available. The anonymous-store diagnostic remains a separate bound.
PagerDB recovery now returns the last checkpoint, not the most recent commit.
