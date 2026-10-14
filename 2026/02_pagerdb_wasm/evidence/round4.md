# Final barrier changes, round 4

2026-10-15 JST. Starting from pushed `5db4795`. Single-process Apple M3
diagnostics, Go 1.26.0 darwin/arm64, wazero 1.12.0, PagerDB af2c4c9.
The shared `~/.cache/pagerdb-bench.lock` was held during measurement and released
afterward. Background machine load was not controlled. No PagerDB change,
new configuration, guest undo queue, or instrumentation change.

## Changes

- `host.go`: remove the live dirty-line list and its sort. On every exit, walk
  the 1 MiB bitmap as little-endian 64-bit words, enumerate set bits in order,
  build `DirtyLines`, clear the words, and charge dirty-line gas before undo.
  Reverted calls still report touched lines and restore bytes and globals.
- ABI writes find consecutive unset lines and call existing `DB.Touch` once per
  range, before copying bytes. An already dirty line splits a range, preserving
  the first undo image. Bitmap bits and gas still count individual 64-byte lines.
- `host_test.go`: check undo after overlapping ABI ranges with an already dirty
  line in the middle; check status, trap and out-of-gas reverts after a coalesced
  300-byte input. Existing barrier/memcmp, bitmap/root and final gas tests remain.
- `matcher_test.go`: reuse the existing timed-call wrapper to measure end work.
  The old per-line log-size formula is now explicitly an upper bound because
  coalescing reduces record headers. No new log-size measurement was added.
- `barrier_diagnostic_test.go`: remove its now-redundant list reconstruction and
  let production finalization build the list for all controls. Full/host/empty
  ABI first-touch counts are derived from total lines minus guest misses.
  The stub's ABI count is no longer measured. No separate scan time is excluded.

## Original fixture, two 600-block runs

Each block contains 50,000 messages. Every output matches the native oracle;
Commit follows every block, and the two new root sequences match at all 600
blocks. This does not compare against an exported old-commit root sequence.
The native oracle was matched to the external golden file in the prior rounds.

Before uses the production-host replay in `round3-original-correctness.txt`.
After uses `round4-original.txt`. Both exclude deployment, fixture reads, output
hashing, dirty-line summaries and final database close from the execution/Commit
timers. After adds the same small timed-call wrapper used by the round 3 controls.

| Measure | Before run 0 / 1 | After run 0 / 1 |
| --- | ---: | ---: |
| Execution msgs/s | 806,122 / 813,770 | 971,062 / 1,017,478 |
| Execution + Commit msgs/s | 610,344 / 601,742 | 686,727 / 700,851 |
| Execution ms/block | 62.025 / 61.442 | 51.490 / 49.141 |
| Execution + Commit ms/block | 81.921 / 83.092 | 72.809 / 71.342 |
| Commit mean ms/block | 19.896 / 21.650 | 21.319 / 22.201 |
| Commit p99 ms/block | 116.434 / 115.085 | 112.556 / 111.325 |

Dirty lines remain 149,136.893/block and dirty 1 KiB leaves remain 30,675.0/block.
Commit mean did not improve in these two runs. The execution improvement is the
main source of the better combined rate. These sequential diagnostics are not a
controlled statistical estimate under identical background load.

## End work and Touch, per block

| Measurement | Before | After |
| --- | ---: | ---: |
| End wall ms, unprofiled runs | 9.596 / 10.195 | 0.912 / 0.847 |
| End wall ms, separate CPU-profile runs | 10.081 | 0.905 |
| End sampled CPU ms | 7.32 | 0.58 |
| `DB.Touch` inclusive sampled CPU ms | 8.05 | 7.43 |
| Within Touch: old-byte copy sampled CPU ms | 6.23 | 5.95 |
| ABI input callback wall ms, profile runs | 1.109 | 1.682 |

Before unprofiled end time comes from the two full controls in
`round3-controls-{1,2}.txt`; after comes from the two ordinary matcher replays.
The profile comparison uses identical phase labels in `TestBarrierDiagnostics`.
Before Touch is 4.83 CPU seconds over 600 blocks; after is 4.46 seconds. Old-byte
copy samples are 3.74 and 3.57 seconds. Nested CPU rows overlap and are not additive.
The new profile checks all 600 output hashes and reports 48.735 ms execution,
20.421 ms Commit and 1,025,955 execution msgs/s.

Removing the sort accounts for most of the measured gain. Complete end work is
about 0.9 ms, not tens of microseconds: it also allocates and fills roughly 149k
returned line numbers. The after profile attributes 0.19 of 0.35 end CPU seconds
to slice growth. This round deliberately stops without another optimization.

Touch CPU improves modestly in this profile pair. The ABI callback wall time did
not improve in the profiled pair, so there is no claim of an isolated ABI wall-time
speedup. The callback includes bounds checks, bitmap work, Touch and input copy.
Exact Touch-only elapsed time remains unmeasured; CPU sampling and wall time
must not be confused. Guest misses still call Touch individually. Coalescing
reduces ABI calls and headers, not the old-byte volume or guest miss count.

## Verification and reproduction

Pass: `go vet ./...`, `go test ./...`, the two original 600-block hash/root
replays, and the separate original 600-block diagnostic profile. Default tests
also replay the generated 20-block fixture twice. The generated 600-block
performance fixture was not rerun, as this round requested the original only.

```sh
cd ~/slop/2026/02_pagerdb_wasm
go vet ./...
go test ./...
until mkdir ~/.cache/pagerdb-bench.lock 2>/dev/null; do sleep 20; done
PAGERWASM_FIXTURE="$HOME/temp-1m-clob/bench/fixture.bin" \
  go test -run '^TestMatcher$' -count=1 -v -timeout=15m
PAGERWASM_FIXTURE="$HOME/temp-1m-clob/bench/fixture.bin" \
  PAGERWASM_BARRIER_DIAGNOSTICS=1 PAGERWASM_BARRIER_MODE=full \
  PAGERWASM_BARRIER_PROFILE="$HOME/.cache/cx/wasmhost.round4.cpu" \
  go test -run '^TestBarrierDiagnostics$' -count=1 -v
go tool pprof -top -cum -tagfocus=phase=guest "$HOME/.cache/cx/wasmhost.round4.cpu"
go tool pprof -top -cum -tagfocus=phase=run "$HOME/.cache/cx/wasmhost.round4.cpu"
go tool pprof -list=logEntry -tagfocus=phase=guest "$HOME/.cache/cx/wasmhost.round4.cpu"
rmdir ~/.cache/pagerdb-bench.lock
```

Raw evidence: `round4-original.txt`, `round4-profile.txt`,
`round4-cpu-{guest,end,logentry}.txt`, `round4-test.txt`, `round4-vet.txt`.
The raw binary profile stays at `~/.cache/cx/wasmhost.round4.cpu`.
No fixture contents, binary profile, new compiled artifact or individual output
hashes are committed. The Rust deployment subprocess remains the existing
prototype shortcut. No additional barrier work is planned in this round.
