# Bulk copy/fill without byte-loop lowering, round 5

2026-10-15 JST, starting from round 4 commit `6744e97`. Single-process Apple M3
diagnostics, Go 1.26.0 darwin/arm64, wazero 1.12.0, unchanged PagerDB af2c4c9.
Measurements held `~/.cache/pagerdb-bench.lock`, now released. Background machine
load was not controlled. No cloud runs, PagerDB changes or per-block journal change.

## What changed

`testdata/matcher/build.sh` enables bulk memory and removes
`--llvm-memory-copy-fill-lowering`. The checked-in matcher now retains eight
`memory.copy` and four `memory.fill` instructions. No memory.init, data.drop or
table operation was needed. A no-pass Binaryen re-encode remains: the Rust linker
emits padded call_indirect table-zero immediates which fail the strict MVP parser
with `zero byte expected`; re-encoding normalizes them without lowering copies.

`instrument/src/main.rs` validates with bulk parsing enabled, but explicitly
allows only copy/fill. Other bulk instructions fail deployment; passive data and
element segments remain forbidden. Floats, SIMD, threads, reference mutation,
WASI and imports outside the ABI remain rejected. The injected `touch_range`
import is reserved, like `touch`; source programs cannot import either one.

Each bulk operation receives:

1. An inline unsigned gas check/decrement proportional to its byte length.
   `BulkByteGas = 1` lives beside the other constants in `host.go` and is passed
   to the Rust tool. Original instruction gas remains; no gas helper call is
   added for the length charge. The metered control uses the same dynamic pass.
2. For copy, a 64-bit source-plus-length bounds check against usable guest memory.
3. One `touch_range(dst,len)` host call. It receives the normal fixed HostGas
   charge and reuses the coalescing loop extracted from `Program.write` into
   `Program.touchRange`. Destination bounds are checked before any Touch; already
   dirty lines split the ranges. The original copy/fill then executes natively.

Both endpoints are checked for a zero-length copy, and the destination is checked
for zero-length fill. Length zero adds no byte gas and dirties no destination line,
but still pays the original instruction and fixed host-import charges. Address
sums use 64 bits, so 32-bit wrap cannot enter the bitmap. An invalid source,
destination, or length gas budget cannot partially journal the bulk destination.
Previous writes in the call are still undone through Mark/Undo on any trap.
DirtyLineGas remains a charge per distinct 64-byte line at call end, not per range.
Overlapping copies retain Wasm memmove semantics because the actual operation is
not replaced by a host copy.

## Reuse and shortcuts

Enabling parity-wasm 0.45's `bulk` feature alone failed to compile wasm-instrument
0.4.0: its stack-height match did not cover `Instruction::Bulk`. The pinned crate
is copied under `instrument/vendor/wasm-instrument` with its original licenses.
Only its Cargo bulk feature and one stack-effect match are changed: copy/fill pop
three operands and other bulk instructions return an error. The gas and stack
algorithms are reused. See the vendored `PATCH.md`. Cargo.lock retains the same
external dependency versions; the crate now uses this portable in-repo path.
No Go replace directive was added.

The old parity builder also moved the DataCount section after code/data when
instrumenting this matcher, causing `section out of order`. The deploy pipeline
validates the input first, rejects data-segment operations, and removes the now
unused DataCount before those passes. Active initialization data is retained.
The diagnostic meter removes it too. Rust remains a deploy-time subprocess with
the existing code-hash cache. No new runtime option or general bulk-memory API
was introduced.

Other files: `instrument/src/lib.rs` contains the dynamic length meter;
`instrument/src/bin/meter-only.rs` uses it for controls; `host.go` registers the
range barrier and factors the old ABI coalescing loop; `bulk_test.go` and the
bulk/rejection WAT fixtures provide regression coverage. Existing diagnostics
receive the new gas constant. The older scalar miss controls keep bulk range
calls enabled and report those separately; they were not used for this round's
performance comparison. README documents the new contract.

## Correctness

All pass:

- `go vet ./...` and `go test -count=1 -v ./...`.
- Copy/fill dirty-line sets equal a full byte comparison on the small fixtures;
  copied/filled bytes match Go's overlap-safe copy or fill expectation.
- Forward and backward overlap, cross-line writes, low-byte fill semantics,
  valid zero-length operations at the memory endpoint, and invalid zero-length
  endpoints.
- Source/destination bounds, address overflow, oversized lengths, and length-gas
  exhaustion. Invalid bulk destinations add no dirty lines beyond the ABI input.
- Explicit status and post-write traps restore bytes and root; the next call
  succeeds. Exact gas succeeds with zero left; one less gas reverts at the final
  dirty-line charge. Increasing length from one to two bytes costs exactly
  BulkByteGas when the dirty-line set is unchanged.
- Deployment rejects memory.init/data.drop, table.copy and source touch_range,
  alongside the existing WASI clock, float, SIMD and reserved-touch cases.
- Each fixture: two full 600-block host replays match every native output hash
  and every root between the two runs. Each plain and metered control also runs
  all 600 blocks twice and matches every native output hash.

Default tests also replay 20 generated blocks twice. Roots are compared between
runs of the new module, not against an exported old-module root sequence. The
original native oracle was compared with the external golden file in prior rounds;
that separate oracle-vs-file comparison was not repeated here. Raw test output:
`round5-tests.txt`, `round5-vet.txt`, and both `round5-*-host.txt` files.

## Measurements

Every block has 50,000 messages. Controls use anonymous mapped memory, have no
write barrier or PagerDB Commit, and exclude fixture reads and output hashing.
The inline control includes static instruction gas, per-byte bulk gas and stack
limits. Full execution includes ABI copies, both barriers, Touch, gas, bitmap
finalization and result construction. Commit is separate. Deploy, oracle creation,
output hashing and final database close are outside these timers. Commit p99 is
nearest-rank over each 600-block run. All figures are single-process M3 diagnostics.

| Fixture/control | Run 1 msgs/s | Run 2 msgs/s | Execution ms/block, run 1 / 2 |
| --- | ---: | ---: | ---: |
| Original plain | 7,736,832 | 8,035,341 | 6.463 / 6.223 |
| Original inline gas + stack | 7,225,352 | 7,037,772 | 6.920 / 7.105 |
| Generated plain | 4,880,246 | 4,541,758 | 10.245 / 11.009 |
| Generated inline gas + stack | 4,311,739 | 4,387,234 | 11.596 / 11.397 |

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Commit mean ms | Commit p99 ms | Dirty lines/block |
| --- | ---: | ---: | ---: | ---: | ---: |
| Original / 1 | 1,912,971 | 884,291 | 30.405 | 713.554 | 149,136.9 |
| Original / 2 | 1,639,448 | 800,374 | 31.973 | 741.940 | 149,136.9 |
| Generated / 1 | 967,165 | 547,022 | 39.707 | 363.226 | 243,122.9 |
| Generated / 2 | 1,020,082 | 535,216 | 44.405 | 515.688 | 243,122.9 |

Dirty 1 KiB leaves/block: original 30,675.0; generated 71,734.7. End work ms/block:
original 0.763 / 0.757; generated 0.978 / 1.448. The printed uncoalesced log-byte
figures are only upper bounds; actual coalesced log volume was not measured here.

## Before/after round 4, original fixture

Round 4 did not measure the generated 600-block fixture, so there is no round 4
generated baseline to claim. Before rows below are from `round4-original.txt`.

| Measure | Round 4, run 1 / 2 | Round 5, run 1 / 2 |
| --- | ---: | ---: |
| Execution msgs/s | 971,062 / 1,017,478 | 1,912,971 / 1,639,448 |
| Execution + Commit msgs/s | 686,727 / 700,851 | 884,291 / 800,374 |
| Execution ms/block | 51.490 / 49.141 | 26.137 / 30.498 |
| Execution + Commit ms/block | 72.809 / 71.342 | 56.542 / 62.471 |
| Commit mean ms/block | 21.319 / 22.201 | 30.405 / 31.973 |
| Commit p99 ms/block | 112.556 / 111.325 | 713.554 / 741.940 |
| Dirty lines/block | 149,136.9 / 149,136.9 | 149,136.9 / 149,136.9 |

The coordinator's pre-change plain controls measured 3.75M / 3.68M msgs/s for the
lowered module; the new plain runs measure 7.74M / 8.04M. Source: project devlog
2026-10-15, "Why the Wasm host runs at 1M, not 8M". These new controls agree with
that root-cause measurement. No fresh lowered-control rerun was added here.

Full execution improves substantially. Commit means and tails are worse in these
runs, so the combined gain is smaller. These runs do not isolate the cause of the
Commit change or separate background load from changed overlap with the sealer.
No journal redesign, new CPU profile, or additional storage experiment was done.

## Reproduce

```sh
cd ~/slop/2026/02_pagerdb_wasm
export PATH="$HOME/.cargo/bin:$PATH"
sh testdata/matcher/build.sh
sh testdata/build-cases.sh
go vet ./...
go test -count=1 -v ./...
until mkdir ~/.cache/pagerdb-bench.lock 2>/dev/null; do sleep 20; done
PAGERWASM_FIXTURE="$HOME/temp-1m-clob/bench/fixture.bin" PAGERWASM_DIAGNOSTICS=1 \
  go test -run '^TestDiagnostics$/(matcher.wasm|inline-stack.wasm)$' -count=2 -v -timeout=20m
PAGERWASM_FIXTURE="$HOME/temp-1m-clob/bench/fixture.bin" \
  go test -run '^TestMatcher$' -count=1 -v -timeout=20m
PAGERWASM_FIXTURE="$HOME/.cache/cx/wasmhost.synthetic600.bin" PAGERWASM_DIAGNOSTICS=1 \
  go test -run '^TestDiagnostics$/(matcher.wasm|inline-stack.wasm)$' -count=2 -v -timeout=20m
PAGERWASM_FIXTURE="$HOME/.cache/cx/wasmhost.synthetic600.bin" \
  go test -run '^TestMatcher$' -count=1 -v -timeout=20m
rmdir ~/.cache/pagerdb-bench.lock
```

The cached generated fixture is the existing seed-1, 600-block fixture from round
2. It can be regenerated with `go run ./cmd/fixture 600 > /tmp/synthetic.bin` and
that path substituted above. Each TestMatcher invocation already runs twice.
Raw data: `round5-{original,generated}-{controls,host}.txt`. Fixtures, individual
output hashes, Cargo target artifacts and Go test binaries are not committed.
