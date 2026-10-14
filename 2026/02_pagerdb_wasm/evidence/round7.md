# Dirty lines and hypothetical scratch savings, round 7

2026-10-15 JST. Diagnostic only, single-process Apple M3 under the shared
`~/.cache/pagerdb-bench.lock`. Original fixture, 600 blocks, one call and Commit
per block. Every native output hash passes. No production code, guest module,
PagerDB, gas rule or persistence behavior changed. No throughput measurement:
the full snapshots and listeners deliberately add work.

## Main finding

Of 149,136.893 dirty lines/block, 132,555.575 (88.88%) change at call end and
16,581.318 (11.12%) finish unchanged. Stack accounts for only 29.863 lines/block.
The observed ABI input uses 31,251 lines/block; output uses 1,759.452. Other heap
accounts for 116,088.225 lines/block, including 99,507.173 changed lines.

If every unchanged line and every line overlapping an observed input/output
range were excluded from state, 49,591.770 lines/block would be excluded.
The calculated journal falls from 21.346 MB to 14.498 MB/block (32.08% less,
decimal MB). Hashed region leaves fall from 30,674.992 to 25,254.460 (17.67% less).
This would leave most hashing and journal work. These are hypothetical volume
savings, not a measured speedup or proof of safe scratch allocation.

## Changed bytes and leaves

A dirty line means first-written in the call, even if its final bytes are equal
to the pre-call bytes. Each occurrence is counted once per block, so totals below
sum block occurrences rather than distinct addresses across the whole fixture.
Histograms count changed lines by the number of final byte differences in 64 B.
p99 is the nearest rank (594th sorted value out of 600); p99s are not additive.

| Metric | Mean/block | p99/block | All-block total |
| --- | ---: | ---: | ---: |
| dirty_lines | 149,136.893 | 290,656 | 89,482,136 |
| changed_lines | 132,555.575 | 283,536 | 79,533,345 |
| unchanged_lines | 16,581.318 | 34,979 | 9,948,791 |
| changed_bytes | 2,150,582.293 | 5,355,371 | 1,290,349,376 |
| changed_1_8 | 36,830.507 | 72,677 | 22,098,304 |
| changed_9_32 | 92,097.025 | 183,901 | 55,258,215 |
| changed_33_64 | 3,628.043 | 27,295 | 2,176,826 |
| all_leaves | 30,674.992 | 52,451 | 18,404,995 |
| changed_leaves | 27,318.158 | 51,867 | 16,390,895 |

## Attribution

Each line has one category. Priority is input/output overlap, input, output,
active stack, static/BSS, then other heap. There were no input/output overlaps
and no dirty lines below the stack top that failed the active-frame check.
The stack category uses `[SP, stack_top)` observed at the first barrier miss;
SP is read before the host callback writes anything. Stack top is 1,048,576,
minimum observed SP is 1,045,456. Static/BSS is `[1048576, 1051136)`; heap starts
at 1,051,136. The module hash is pinned in the diagnostic so a new build requires
checking these boundaries again.

| Metric | Mean/block | p99/block | All-block total |
| --- | ---: | ---: | ---: |
| stack_lines | 29.863 | 47 | 17,918 |
| stack_changed | 29.855 | 47 | 17,913 |
| stack_unchanged | 0.008 | 0 | 5 |
| static_lines | 8.353 | 9 | 5,012 |
| static_changed | 8.095 | 9 | 4,857 |
| static_unchanged | 0.258 | 2 | 155 |
| input_lines | 31,251.000 | 31,251 | 18,750,600 |
| input_changed | 31,251.000 | 31,251 | 18,750,600 |
| input_unchanged | 0.000 | 0 | 0 |
| output_lines | 1,759.452 | 9,035 | 1,055,671 |
| output_changed | 1,759.452 | 9,035 | 1,055,671 |
| output_unchanged | 0.000 | 0 | 0 |
| other_heap_lines | 116,088.225 | 258,937 | 69,652,935 |
| other_heap_changed | 99,507.173 | 251,688 | 59,704,304 |
| other_heap_unchanged | 16,581.052 | 34,979 | 9,948,631 |

The ABI input range is `[4165336, 6165336)` in every block (2,000,000 bytes).
Output has five different base addresses across the run and lengths from 5,320
to 1,335,888 bytes. Per-block exact intervals are recorded in the raw JSON rows.
The ranges are the ABI lengths, not allocator capacity. About 3.872 dirty lines
per block (p99 4, total 2,323) straddle their boundaries. The estimate excludes
these whole lines too, so it is optimistic if adjacent bytes must remain state.

Source inspection also shows an internal input copy in `Engine.input`, plus
reused `Out` buffers (`fills`, `dkeys`, `dvals`, `touched`, `keys`, `buf`, `idx`,
`sorted`). The ABI range classifier does not label these internal buffers;
they remain in other heap. See `testdata/matcher/matcher/src/native.rs` and
`spec.rs`, and `testdata/matcher/wasm-native/src/lib.rs`.

Allocation sites and lifetimes were not tagged. The shipped module has stripped
function names and an optimized allocator; a separate allocator build would
need its own instrumentation and validation. That optional expansion was
skipped to keep this diagnostic small. No temporary-versus-long-lived heap
classification is claimed.

## Journal and scratch estimate

The listener records the exact first-write ranges submitted by scalar, bulk and
ABI paths. It mirrors the existing range coalescing without modifying memory.
Coverage is checked against every dirty line in every block. A PagerDB Touch
splits at 2 MiB segment boundaries. The pinned PagerDB log layout uses a 32 B
header plus old and final bytes for each range, and a 56 B block record.
Thus calculated logical bytes are `56 + 32 * records + 128 * dirty_lines`.
This excludes file allocation slack, checkpoint/rotation records and compressed
sealed history. It is not disk usage or compressed bytes.

For the scratch estimate, remove unchanged and input/output lines from each
original Touch range, split the remaining runs at holes and segment boundaries,
and apply the same formula. It preserves original Touch boundaries rather than
assuming a new global coalescer. The one-record-per-retained-line row is a looser
upper bound. Leaf counts are 1 KiB region leaves, not hash-tree internal nodes.

| Metric | Mean/block | p99/block | All-block total |
| --- | ---: | ---: | ---: |
| touch_calls | 70,511.263 | 146,711 | 42,306,758 |
| journal_records | 70,512.635 | 146,714 | 42,307,581 |
| journal_bytes | 21,345,982.667 | 41,863,320 | 12,807,589,600 |
| scratch_lines | 49,591.770 | 71,585 | 29,755,062 |
| retained_lines | 99,545.123 | 251,714 | 59,727,074 |
| retained_records | 54,873.778 | 134,121 | 32,924,267 |
| retained_journal_bytes | 14,497,792.693 | 36,349,368 | 8,698,675,616 |
| retained_line_record_upper_bytes | 15,927,275.733 | 40,274,296 | 9,556,365,440 |
| retained_leaves | 25,254.460 | 49,890 | 15,152,676 |
| saved_journal_bytes | 6,848,189.973 | 10,224,864 | 4,108,913,984 |
| saved_leaves | 5,420.532 | 7,028 | 3,252,319 |

The earlier 23.862 MB/block number was an uncoalesced upper bound. This diagnostic
accounts for existing ABI and bulk coalescing and calculates 21.346 MB/block.
It observes 70,511.263 Touch calls/block, not 149,136.893 calls; the latter is the
number of distinct lines covered by those calls.

Unchanged final bytes do not imply disposable bytes: a line may hold persistent
values that were read or restored during the call. Zeroing those values next
call could break execution. A free scratch range also needs allocation/lifetime
rules and separation from state. This run neither zeroed candidate lines nor
validated an allocator redesign. Root equivalence after removing those lines,
crash behavior of scratch, and execution/Commit speedup are unverified.

## Files and reproduction

- `scratch_diagnostic_test.go`: opt-in test; no production hook or guest rebuild.
- `round7-original.txt`: all 600 per-block metric/range rows, summaries and PASS.
- `round7-summary.csv`: mean, p99 and all-block total for every metric, including
  category changed bytes. Saved metrics are differences within each block.
- `round7-tests.txt`, `round7-vet.txt`: final standard checks.
- README: opt-in command and scope.

```sh
cd ~/slop/2026/02_pagerdb_wasm
until mkdir ~/.cache/pagerdb-bench.lock 2>/dev/null; do sleep 20; done
PAGERWASM_SCRATCH_DIAGNOSTICS=1 \
PAGERWASM_FIXTURE="$HOME/temp-1m-clob/bench/fixture.bin" \
go test -run '^TestScratchDiagnostics$' -count=1 -v -timeout=30m
rmdir ~/.cache/pagerdb-bench.lock
go vet ./...
go test -count=1 -v ./...
```

The snapshot copies the full mapped region (538,968,064 bytes), which includes
the 512 MiB guest memory, reserved bitmap and allocation padding. Only the
reported dirty lines in usable guest memory enter these metrics. The snapshot
is reused; no full per-block memory dump is retained. No process lists were
collected. The two-block generated smoke test also passed before the original run.
