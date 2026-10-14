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
The coordinator added state gas during this round. At call end, `BytesWritten`
is the number of distinct dirty lines times 64, and `LeavesTouched` is the number
of distinct 1 KiB PagerDB leaves. The charge is `BytesWritten * 6 +
LeavesTouched * 2000`, using `ByteGas` and `LeafGas` in host.go. Both counts
are returned even on reverts. Repeated writes add no state gas. BulkByteGas
remains a separate CPU charge. The ordered bitmap scan computes both counts.
Overlapping copies retain Wasm memmove semantics because the actual operation is
not replaced by a host copy.

## Reuse and shortcuts

Enabling parity-wasm 0.45's `bulk` feature alone failed to compile wasm-instrument
0.4.0: its stack-height match did not cover `Instruction::Bulk`. The pinned crate
is copied under `instrument/vendor/wasm-instrument` with its original licenses.
Its functional changes are the Cargo bulk feature and one stack-effect match: copy/fill pop
three operands and other bulk instructions return an error. One original trailing
space was also removed. The gas and stack
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
  state charge. Increasing length from one to two bytes costs exactly
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
`round5-tests.txt`, `round5-vet.txt`, `round5-state-tests.txt`, and both
`round5-*-final.txt` files. State-gas tests also check repeated writes, byte
counts, and that two lines in one leaf cost 2000 less than two in different leaves.

## Initial measurements, before the state-gas scope addition

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
initial runs, so the combined gain is smaller. The follow-up below diagnoses the
wait path and repeats the full fixtures with the final state-gas implementation.

## Commit tail diagnosis

The last-20-block profile did not capture a slow rotation, so the existing
TestCommitProfile now profiles all remaining fixture blocks after PROFILE_START.
The full 600-block trace measured Commit mean 25.233 ms. Slow calls recur near
blocks 181, 223, 271, 327, 381, 438, 496 and 554 (767.596, 318.529, 166.232,
389.981, 527.301, 676.349, 747.195 and 2067.258 ms). Across the trace, Commit
spent 4.06 seconds in DB.rotate -> sync.Cond.Wait, waiting for the sealer backlog
to fall below two epochs. Commit also spent 1.656 seconds in Msync. Hashing
accounts for most sampled Commit CPU (hashDirty worker 6.55 seconds cumulative).
The separately created sealer workers account for 43.70 sampled CPU seconds
(56.74% of all CPU samples), including 32.71 seconds in sealSection and
10.98 seconds in disk sorting. This confirms substantial background sealing
work. These are aggregate profile times, not additions to the benchmark means.

Original dirty lines and leaves are exactly unchanged from round 4: 149136.9
and 30675.0 per block. Faster guest execution gives the background sealer less
wall time between rotations, which is consistent with the observed backlog wait.
The trace establishes the wait mechanism, but does not isolate how much of the
regression comes from that pacing versus machine memory pressure. During the
profile, three unrelated Go processes held about 6 GB, physical memory was nearly
full, and about 5.1 GB was compressed. CPU still had about 48% idle capacity in
the sampled interval. No unrelated process was stopped. The final replay pair
below is a repeat under observed shared load, not an idle-machine experiment.

Evidence: round5-all-commit-profile.txt, round5-commit-{cpu,sync,syscall}.txt,
round5-sealer-cpu.txt,
round5-load-{before-profile,during-profile,final}.txt. Raw CPU/trace files remain
at ~/.cache/cx/wasmhost.round5-all-commit.cpu and its .trace suffix. Reproduce
with PAGERWASM_FIXTURE set to the original fixture and
PAGERWASM_PROFILE=~/.cache/cx/wasmhost.round5-all-commit.cpu,
then go test -run '^TestCommitProfile$' -count=1 -v -timeout=20m.

```sh
go tool pprof -top -cum -nodecount=30 -tagfocus=stage=commit ~/.cache/cx/wasmhost.round5-all-commit.cpu
go tool trace -pprof=sync ~/.cache/cx/wasmhost.round5-all-commit.cpu.trace > /tmp/round5-sync.pprof
go tool trace -pprof=syscall ~/.cache/cx/wasmhost.round5-all-commit.cpu.trace > /tmp/round5-syscall.pprof
go tool pprof -top -cum -nodecount=20 -focus='pagerdb.*Commit' /tmp/round5-sync.pprof
go tool pprof -top -cum -nodecount=20 -focus='pagerdb.*Commit' /tmp/round5-syscall.pprof
```
No PagerDB or per-block journal change was made.

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


## Final implementation replay

These runs include the final byte and leaf charges. The plain and inline controls
above remain valid: they do not execute host state-gas finalization.

| Fixture/run | Execution msgs/s | Execution + Commit msgs/s | Commit mean ms | Commit p99 ms | Dirty lines/block |
| --- | ---: | ---: | ---: | ---: | ---: |
| Original / 1 | 2,116,746 | 1,036,369 | 24.624 | 320.137 | 149,136.9 |
| Original / 2 | 2,141,791 | 1,009,690 | 26.175 | 673.444 | 149,136.9 |
| Generated / 1 | 1,041,899 | 551,728 | 42.635 | 553.987 | 243,122.9 |
| Generated / 2 | 1,070,423 | 569,697 | 41.055 | 384.821 | 243,122.9 |

Original bytes written/block: 9,544,761; leaves touched/block: 30,675.0.
End work: 0.764 / 0.762 ms/block. Execution: 23.621 / 23.345 ms/block;
execution + Commit: 48.245 / 49.520 ms/block. Round 4 execution was
51.490 / 49.141 ms and execution + Commit was 72.809 / 71.342 ms.
Both 600-block runs pass every output hash, root comparison and returned count.
The repeated p99 remains high, so the initial tail was not just a one-run outlier.

Generated bytes written/block: 15,559,868; leaves touched/block: 71,734.7.
End work: 1.162 / 1.329 ms/block. Execution: 47.989 / 46.710 ms/block;
execution + Commit: 90.624 / 87.765 ms/block. Both 600-block runs pass every
output hash, root comparison and returned count. There is no generated round 4
baseline. Final evidence: round5-original-final.txt and round5-generated-final.txt.
