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
a 64-byte header: count (u64), value size (u64), layout version (u64, currently 2),
40 zero bytes. Versions 0 and 1 used different bucket or probing rules and cannot
be reopened by this adapter; replay them into a fresh database. The padding keeps
64-byte storage entries and their values aligned to PagerDB's 32-byte redo
chunks, so an entry never straddles a leaf. Entries follow. Capacity starts at
16, accepts any integer capacity, and grows by 1.5x (rounded up) before an
insert exceeds 85% load or probe distance 64. Bucket selection uses fastrange,
`(Home(key) * capacity) >> 64`, with a 128-bit product. `Home` uses one
multiply-xorshift step per little-endian key word, then a final multiply and
shift (see `table.go`). Robin Hood probing walks adjacent buckets with wraparound. An incoming entry
displaces a resident whose distance from its home is shorter. Distances are
computed from keys, with no stored metadata. Lookup stops at an empty bucket or
a resident whose distance is shorter than the lookup distance. Insertion checks
the full displacement chain before changing entries, so probe-cap growth cannot
lose a displaced key. An all-zero value is empty. Zero writes delete by shifting
following entries back until an empty bucket or an entry at its home. A write of
the current value changes nothing. Growth rehashes in old bucket order and keeps
the PagerDB region handle stable.

`Home` is not collision resistant. Nor was the earlier SHA-256 bucket index: a
bucket is a bounded hash result, so grinding keys into one bucket costs the same with
either. Known open risk: keys ground into one bucket force the probe-cap
growth at low load, which multiplies table memory.

On 2026-10-19, 50M-key fills on macOS measured these table costs. Probe
statistics are distances from the home bucket (zero at home), not bucket visits.
Both key sets use the existing home-sorted fill order through `Table.Set`.
The trace and mixed-operation tests exercise unsorted writes and deletion.

- 75% load: 85.33 B/slot; random max/mean 26/1.500739, consecutive max/mean
  14/1.165011; zero growth events in both fills.
- 85% load: 75.29 B/slot; random max/mean 44/2.834748, consecutive max/mean
  19/1.855585; zero growth events in both fills.
- 90% load: 71.11 B/slot; random max/mean 62/4.500481, consecutive max/mean
  24/2.625291; zero growth events in both fills.

The selected limit is 85%. At 90%, the random fill leaves only two buckets of
headroom below the cap of 64. `Home` is unchanged: histograms of 1M consecutive
big-endian keys under two prefixes pass, and the 50M array keys above are
`keccak256(0) + i` with big-endian addition. A dense 5M random fill at 85% uses
75.29 B/slot, with maximum distance 39, mean 2.840990 and zero growth events.

The 1.5x growth ladder still matters. Its first capacities that hold 5M and 50M
slots remain 6,961,464 and 79,295,427, so table cost at those preallocated
capacities remains 89.11 and 101.50 B/slot. Measured max/mean distances are
20/1.278733 and 17/0.853617, with zero fill growth events. Raising the limit does not shrink a
table. At 500M, the first sufficient ladder capacity falls from 903,224,480 to
602,149,653 entries: an estimated 115.61 to 77.08 table B/slot. A table
preallocated directly for 85% would instead use about 75.29 B/slot. The 500M
figures assume no additional probe-cap growth; they are capacity estimates,
not a full fill or RSS measurement. Table bytes
exclude PagerDB hashes, undo history, snapshots and runtime memory, so comparison
with an arena MPT's total memory still needs a full-scale RSS run. Mac timings
are diagnostic only.

The 600-block Matcher trace peaks at 4,242,421 live slots before ending at
3,446,600. With 5,000,000 initial entries, 85% permits the peak without growth
(84.85% peak load). Final table cost falls from the old 193.90 to 92.85 B/live
slot. The maximum distance over scans at every block boundary is 38; the final
maximum is 17 and mean is 1.109491. There are zero growth events. The final load
is only 68.93% because the trace deletes slots after its peak; tables do not
shrink. The old run started with 6,961,464 entries and grew to 10,442,196 on the
probe cap at block 162, with 3,843,172 live slots (55.2% load). Its
block-boundary maximum was 51; after growth its final max/mean was 19/0.245906.

For the same 4.25M peak-live budget, the 75% trace candidate uses 5,666,667
entries and ends at 105.22 B/slot (block-boundary maximum 23, final max/mean
14/0.776963). The 90% candidate uses 4,722,223 entries and ends at 87.69 B/slot
(block-boundary maximum 63, final max/mean 20/1.352461). Both have zero growth
events. The 90% trace result reinforces the 85% choice: it leaves just one bucket
below the probe cap.

With 16,777,216 virtual 1 KiB leaves and 5,000,000 initial entries, the new
PagerDB trace roots are:

- Block 150: `0x97b222ba4fb900965a3fbea8b9f4f9867bdb08c7c524401712224ebeb6643997`.
- Block 600: `0x2627914a99906bced7ecc9c285ddb610f2ee19fa3a4a0c5adccf1f731c636d7e`.

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
activation and SAE transition support are not claimed. Tests cover three million mixed table
operations against Go maps, high-load churn, reachability from every key's home,
consecutive keys, deletion, growth, collision limits, PagerDB undo,
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
