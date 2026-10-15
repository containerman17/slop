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

The replay now rebuilds the Ethereum state MPT from PagerDB tables after the last
block and compares it with that header's `Root`. `StateRoot` in `database.go`
walks the account region, each account's slot table and the code table with
`Table.Each`, and feeds sorted leaves into a libevm `StackTrie`. Keys are
`keccak(address)` and `keccak(raw slot key)`. Storage values are RLP of the value
with leading zeros trimmed. Account values carry nonce, balance, storage root,
code hash and coreth's IsMultiCoin flag. Each referenced code region must hash to
its stored code hash. A mismatch stops the command with `STATE ROOT MISMATCH` and
exit status 1.

The root matched mainnet at both checkpoints. No adapter bug was found, so the
adapter state code did not change.

| Run | Header root | Rebuild | Replay elapsed | Wall (`time -p`) |
|---|---|---|---|---|
| 1..100,000 | `0x03c2dc48...89e9ecdf` | 218 ms | 11.546 s | 13.18 s |
| 1..1,000,000 | `0xfab4187f...3effd88d` | 820 ms | 172.381 s | 173.84 s |
| 1..1,000,000, `-root-every 100000` | 10 of 10 match | 221 to 817 ms | 177.468 s | 180.13 s |

The 100,000-block run executed 93,870 EVM transactions, 6,178 atomic imports and
712 atomic exports, with 11,730,435,622 gas. The 1,000,000-block run executed
1,128,919 EVM transactions, 52,563 imports and 16,184 exports, with
124,101,522,756 gas. Gas used, receipt root and bloom matched in every block.
A smoke run of blocks 1..10 with `-root-every 1` also matched every block root.
Logs: `replay-100k.txt`, `replay-1m.txt`, `replay-1m-root-every-100k.txt`.

Commands, from this directory:

```sh
go run ./cmd/replay -data "$HOME/temp-1m-clob/blockdump/data/mainnet-C" -to 100000
go run ./cmd/replay -data "$HOME/temp-1m-clob/blockdump/data/mainnet-C" -to 1000000
go run ./cmd/replay -data "$HOME/temp-1m-clob/blockdump/data/mainnet-C" -to 1000000 -root-every 100000
```

Earlier, the differential test found stale slots after account recreation. The
adapter now resets old storage when stock StateDB opens an empty storage
namespace. That test now also compares `StateRoot` with the stock coreth MPT root
after every one of its 30 blocks, and the reopen test checks that the rebuilt
root survives a checkpoint reopen.

`go vet ./...`, `go test ./...`, and `go test -race ./...` pass (`checks.txt`).
Random table tests perform 20,000 operations for each value size (slots and
accounts). A deliberately colliding set checks the probe cap, deletion and undo
across growth. The StateDB test compares 6,000 random operations with geth's
in-memory state, including reverts, destruction, recreation, balance, nonce,
code, storage, and multicoin flags. Three shuffled initialization orders produce
identical PagerDB roots. Checkpoint reopening preserves accounts, code, slots,
and the root.

## Scope and limits

Replay timers include block decompression, EVM execution, receipt checks, PagerDB
commits, periodic checkpoints, state-root rebuilds, the final Flush and independent
PagerDB Verify. Wall time also includes `go run` compilation, genesis setup and
cleanup. They exclude
genesis setup and final close/removal. PagerDB uses four workers, two retained
versions, a 10,000-version checkpoint interval and default 16 GiB virtual state
capacity. Background machine load and test activity were not controlled. These
Mac timings are diagnostic, not throughput benchmarks.

The state root is compared at the final block (and every N blocks with
`-root-every N`), not at every block by default. A match at block N proves the
whole state at N; it does not prove intermediate states that were not checked.
The replay does not validate consensus signatures or atomic shared-memory proofs.
There is no mainnet deployment, SAE wiring, remote machine access, or PagerDB
source change. The replay is deliberately limited to pre-Berlin blocks.

Phase 2: SAE settled/current version selection and recovery; historical RPC
views, proofs and iteration; Wasm account snapshots and reverts; later-fork
replay coverage; controlled memory/disk/throughput benchmarks; code reclamation.
