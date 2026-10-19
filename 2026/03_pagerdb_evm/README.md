# PagerDB EVM adapter, phase 1

Stock libevm StateDB and coreth execute C-Chain transactions over PagerDB v3
regions. Execution does not maintain a state MPT. The persisted state commitment is
`Pages.RootAt(version)` after `CommitBlock`. Ethereum receipt commitments still
use the receipt trie because replay checks the original receipt hashes. Replay
also rebuilds the Ethereum state MPT from persisted tables for verification.

## Integration choice

This uses the `state.Database` / `state.Trie` interface route. Stock StateDB owns
transaction journals, snapshots, refunds, access lists, transient storage, logs,
and self-destruct rules. Coreth's registered hooks retain C-Chain semantics.
A custom EVM StateDB would duplicate these rules and require more code.

Call `Flush` after **every transaction**, then `CommitBlock` after atomic
transfers at block end. Flush runs `IntermediateRoot`, sorts the staged account,
slot and code writes, and writes them through PagerDB `Write`. It does not commit
a PagerDB version. Sorting makes physical placement independent of Go map
iteration. Transaction-boundary flushing also removes destroyed storage before
same-block recreation. An empty storage marker clears old storage on recreation.
Stock StateDB handles frame reverts before these writes reach PagerDB.

`CommitBlock` returns a version after PagerDB's asynchronous Commit. A fresh
StateDB is opened each block. The zero hash passed to `state.New` selects current
state; it is not a state root. Nonzero selectors are rejected. Trie `Hash` returns
an internal storage-presence marker, never a consensus commitment. No per-account
root is persisted. Do not call stock `StateDB.Commit`, enable a trie prefetcher,
or use StateDB copies with this linear replay adapter. Unsupported interfaces
fail explicitly. This is not yet a general SAE state backend.

The adapter owns an exclusive PagerDB. Region 1 is the account table and region 2
is the code table. `NewDatabase` opens those regions after checkpoint recovery.
The caller must resume execution at the recovered PagerDB version; the replay
command intentionally starts from genesis in a fresh directory.

## Region layout

Integers are little endian unless specified otherwise. Each table begins with
a 64-byte header: count (u64), value size (u64), layout version (u64, currently 1),
40 zero bytes. Version 0 tables used low-bit bucket selection and cannot be
reopened by this adapter; replay them into a fresh database. The padding keeps
64-byte storage entries and their values aligned to PagerDB's 32-byte redo
chunks, so an entry never straddles a leaf. Entries follow. Capacity starts at
16, accepts any integer capacity, and grows by 1.5x (rounded up) before an
insert exceeds 75% load or probe distance 64. Bucket selection uses fastrange,
`(Home(key) * capacity) >> 64`, with a 128-bit product. `Home` uses one
multiply-xorshift step per little-endian key word, then a final multiply and
shift (see `table.go`). Probing is linear with wraparound. An all-zero value is
empty. Zero writes delete with backward shifting. A write of the current value
changes nothing. Growth rehashes in old bucket order and keeps the PagerDB
region handle stable.

`Home` is not collision resistant. Nor was the earlier SHA-256 bucket index: a
bucket is a bounded hash result, so grinding keys into one bucket costs the same with
either. Known open risk: keys ground into one bucket force the probe-cap
growth at low load, which multiplies table memory.

Storage entries are 64 bytes: raw normalized C-Chain slot key (32) and value
(32). Coreth normalizes EVM keys before the adapter receives them. Atomic native
asset storage uses coreth's bypass flag. A contract gets one region on its first
storage update. Tables do not shrink.

Account entries are 112 bytes: address in the first 20 key bytes, 12 zero key
bytes, then an 80-byte record. Record offsets are:

- 0..7: nonce (u64).
- 8..39: uint256 balance, big endian.
- 40..71: EVM code hash.
- 72..75: storage region handle (u32), zero if absent.
- 76: coreth multicoin flag.
- 77: presence flag, keeps an empty account distinct from an absent account.
- 78..79: zero padding.

Code-table entries are 36 bytes: code hash (32) and region handle (4). Code bytes
occupy one immutable region per distinct code hash. Deleted accounts release
storage regions; code regions remain shared and are not garbage-collected.
Physical layout, sorting, capacity rules and PagerDB capacity affect the root.

## Replay and verification

Dependencies pin PagerDB v3 (the exact commit is in `go.mod`) and coreth/avalanchego
v1.15.1, with coreth's libevm version. There are no module replacements.

```sh
go vet ./...
go test ./...
go test -race ./...
go run ./cmd/replay -data "$HOME/temp-1m-clob/blockdump/data/mainnet-C" -to 100000
go run ./cmd/replay -data "$HOME/temp-1m-clob/blockdump/data/mainnet-C" -to 1000000
# optional: also check the state root every 10,000 blocks
go run ./cmd/replay -data "$HOME/temp-1m-clob/blockdump/data/mainnet-C" -to 100000 -root-every 10000
```

The command uses a temporary PagerDB directory and removes it on exit. Use
`-state /path/outside/the/repository` to keep one. Block data and generated state
must stay outside this public repository. No dump, keys, or real account IDs are
included here.

Genesis comes from avalanchego's mainnet configuration. Coreth's four libevm
registrations, mainnet network upgrade schedule, `SetEthUpgrades`, transaction
executor, and atomic `EVMStateTransfer` implement the chain rules. Imports and
exports run after EVM transactions. BLOCKHASH uses the original header chain,
including the genesis hash. Every block checks gas used, receipt hash, and bloom;
the final block also checks the header state root. Use `-root-every N` to
check every N blocks as well. The final check is always on. It hashes addresses
and raw stored slot keys with Keccak, RLP-encodes trimmed storage values, and
encodes all five coreth account fields, including IsMultiCoin. Referenced code
bytes must hash to the stored code hash. The rebuild reads only PagerDB tables,
not StateDB caches, and persists no trie nodes. On 2026-10-15 the rebuilt root
matched the mainnet header root at blocks 100,000 and 1,000,000, and at every
100,000-block checkpoint in between. The command stops at the first
execution error or mismatch. It does not revalidate atomic shared-memory proofs
or consensus signatures in the already accepted local dump.

The command rejects block ranges reaching Berlin (1,640,340). Later precompile
activation and SAE transition support are not claimed. Tests cover random table
operations against Go maps, deletion, growth, collision limits, PagerDB undo,
random StateDB operations and reverts against geth's in-memory MPT, deterministic
roots under shuffled writes, and checkpoint reopening. The differential test also
compares the rebuilt state root with the stock MPT root after every block.
`evidence/REPORT.md` records the replay results and limits.

## Phase 2

SAE wiring needs current/settled version selection, recovery mapping, and
explicit historical-view behavior. RPC views, proofs and iteration need page
store implementations. Wasm accounts need shared snapshot/revert integration.
Later forks and precompile activations need replay coverage. Benchmarks need a
controlled workload, memory and disk accounting, and sustained checkpoint tests.
Code garbage collection and PagerDB journal reclamation are also deferred.
