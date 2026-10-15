# Phase 1 verification

Verified on 2026-10-15 JST, Apple M3, macOS arm64, Go 1.26.8.

The adapter uses stock libevm StateDB over region-backed database/trie interfaces.
The stock journal handles EVM reverts. Sorted transaction-boundary flushes persist
accounts, slots and code through PagerDB Write; block Commit is asynchronous.
Only the PagerDB root commits state. The interface's storage marker is not stored
in account records. Receipt hashing alone retains the standard receipt trie.

Source files: `table.go` implements the common open-addressed table;
`database.go` implements account/code/storage persistence and deterministic flush;
`cmd/replay/main.go` configures coreth and checks the local dump. Tests are in
`table_test.go` and `database_test.go`. `README.md` specifies byte offsets,
endianness, growth and deletion rules, supported calls, and reproduction commands.

## Results

The final 100,000-block run passed gas-used, receipt-root and bloom checks for
every block. It executed 93,870 EVM transactions, 6,178 atomic imports and 712
atomic exports. Gas used was 11,730,435,622. Elapsed time was 11.399 seconds,
8,772.4 blocks/s. See `replay-100k.txt`.

The first one-million-block run also passed all requested checks, with 1,128,919
EVM transactions and 124,101,522,756 gas. It took 174.579 seconds, 5,728.1 blocks/s.
The final command rerun also passed: 52,563 atomic imports, 16,184 atomic
exports, the same EVM transaction and gas totals, 173.129 seconds and 5,776.1
blocks/s. See `replay-1m.txt`.

No mismatch or atomic-transfer failure occurred. The differential test initially
found stale slots after account recreation. The adapter now resets old storage
when stock StateDB opens an empty storage namespace. The fixed test passes.

`go vet ./...`, `go test ./...`, and `go test -race ./...` pass. Random table tests
perform 20,000 operations for each value size (slots and accounts). A deliberately
colliding set checks the probe cap, deletion and undo across growth. The StateDB
test compares 6,000 random operations with geth's in-memory state, including
reverts, destruction, recreation, balance, nonce, code, storage, and multicoin
flags. Three shuffled initialization orders produce identical PagerDB roots.
Checkpoint reopening preserves accounts, code, slots, and the root.

## Scope and limits

Timers include block decompression, EVM execution, receipt checks, PagerDB commits,
periodic checkpoints, the final Flush and independent PagerDB Verify. They exclude
genesis setup and final close/removal. PagerDB uses four workers, two retained
versions, a 10,000-version checkpoint interval and default 16 GiB virtual state
capacity. Background machine load and test activity were not controlled. These
Mac timings are diagnostic, not throughput benchmarks.

The replay does not compare header state roots, validate consensus signatures,
or validate atomic shared-memory proofs. Receipt equality is the requested
execution check, not a proof that all historical state matches the original MPT.
There is no mainnet deployment, SAE wiring, remote machine access, or PagerDB
source change. The replay is deliberately limited to pre-Berlin blocks.

Phase 2: SAE settled/current version selection and recovery; historical RPC
views, proofs and iteration; Wasm account snapshots and reverts; later-fork
replay coverage; controlled memory/disk/throughput benchmarks; code reclamation.
