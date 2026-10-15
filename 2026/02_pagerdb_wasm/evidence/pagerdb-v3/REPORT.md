# PagerDB v3 integration

2026-10-15 JST. Correctness only on macOS arm64; the shared
`~/.cache/pagerdb-bench.lock` was held for every test invocation that prints timing.
No cloud machine was rented and no new performance result is claimed.

The dependency is the pushed PagerDB v3 commit
`a1d6d0f2efc027985e43f1bc62ab85b6e12b962f`, using a normal Go pseudo-version with
no replace directive. Production host calls to Alloc, Map, Touch, Mark and Undo
remain unchanged. The existing leaf-size constant now counts 4 KiB gas leaves;
64-byte first-write tracking remains unchanged. The second-leaf gas test now
writes at offset 4096.

TestMatcher uses asynchronous Commit and waits for the previous root after
executing the next block. Each replay drains its final root and journal and calls
DB.Verify to rebuild the final state root independently of dirty tracking. Other
tests use CommitWait for the old blocking helper shape.

Both original and generated fixtures pass **600 blocks twice each**, 2400 blocks
in total. Every native output hash and byte/leaf count passes. The second replay
matches every root from the first replay within the new 4 KiB format. These are
not comparisons with the old 1 KiB roots. The full fixture pairs use the final
pinned commit; raw outputs are `original600.txt` and `generated600.txt`.

`go vet ./...`, the normal test suite and `go test -race -count=1 ./...` pass.
`tests.txt` records the normal suite on the initial v3 commit b7b8ba0; `race.txt`
records the complete suite on the final commit, whose additional changes only
validate missing/inconsistent checkpoint manifests. The full 600-block runs are
not race-instrumented. The race suite includes the normal 20-block matcher pair.
Go's race detector does not instrument JIT stores.

Reproduction (hold the shared benchmark lock):

```
go vet ./...
go test -race -count=1 -timeout=30m ./...
PAGERWASM_FIXTURE=/Users/ilia/temp-1m-clob/bench/fixture.bin go test -run '^TestMatcher$' -count=1 -v -timeout=30m
PAGERWASM_FIXTURE=/Users/ilia/.cache/cx/wasmhost.synthetic600.bin go test -run '^TestMatcher$' -count=1 -v -timeout=30m
```

The synthetic fixture can be regenerated with `go run ./cmd/fixture 600`. Fixtures
are local inputs and are not committed. Printed rates exclude final drain and
verification, and are not a throughput benchmark. Root completion overlaps
execution, so the old blocking Commit timers are not comparable.

The old TestCloudDiagnostics depended on v2 private journal fields and sealing;
it is removed, with its evidence and source at b9c17c9 retained in history.
The scratch diagnostic's byte model now uses a 92-byte block frame and 16-byte
range framing plus old bytes, without v2 segment splitting or new-byte images.
Its opt-in full attribution run was not repeated. The ideal-store diagnostic is
unchanged and remains a separate diagnostic bound.

Core verification includes local macOS/Linux arm64 vet, unit and race suites and
hosted Linux amd64/macOS arm64 vet, race and no-cgo suites:
https://github.com/containerman17/pagerdb/actions/runs/38710278928

Recovery returns the last complete checkpoint, not the newest commit. The caller
must re-execute later blocks. No reopen-program API was added to this Wasm host.
Cloud throughput, physical power-cut durability, sustained disk backlog behavior,
and phase 2 sealing/history/storage remain unverified or out of scope.
