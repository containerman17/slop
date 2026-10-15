# Wasm host on c8id.8xlarge

Follow-up: [anonymous store bounds, round 2](round2/REPORT.md). The root-only RAM pipeline reaches about 2.02M wall msgs/s without checkpoints; a packed log with per-block fdatasync reaches about 1.87M. The measurements below describe round 1.

2026-10-15 JST. Source `840d09dd9a2650c1a427dfdbb49e1783f8e2d080`, fetched origin/main before launch. The baseline uses unchanged production source. Only an opt-in diagnostic test and evidence are committed; box-local variants are described separately. PagerDB `af2c4c95bb9d`, wazero v1.12.0.

## Machine and protocol

One c8id.8xlarge in us-west-2a, Intel Xeon 6975P-C, 16 physical cores and 32 hardware threads, 64 GiB nominal memory. Ubuntu 24.04, kernel 7.0.0-1014-aws, Go 1.26.0 linux/amd64 and Rust 1.99.0 stable. CPUs 0-15 select one thread on each physical core. `GOMAXPROCS=16`, `taskset -c 0-15`. All DBs, fixtures and TMPDIR used local ext4 NVMe `/dev/nvme1n1`. Fresh DB per replay, no cache dropping, one benchmark at a time. Pre-run vmstat samples show 100% idle; uptime and topology are in `host.txt` and run logs. No other workload was launched on this instance.

Source and fixture transfer finished before measurements. Original fixture SHA256 matched the local source. Both fixtures have 600 blocks of 50,000 messages. Synthetic data was generated on the box with `go run ./cmd/fixture 600`. Only the source archive and original fixture were copied from the Mac. Existing checked-in Wasm artifacts were used; deployment invoked the existing Rust tool with locked dependencies. Commands and setup are in `COMMANDS.md` and `setup.txt` (launch request kept outside the repo).

Each TestMatcher invocation runs the fixture twice, checks every output against the native matcher, compares the two root sequences and checks byte/leaf counts. Timers exclude deployment, fixture reads, output hashing, dirty-line summaries and final database close. Execution includes end work; end work is not added again. Execution plus Commit is the sum of these two timers, not complete wall-clock throughput. DB RetainVersions is the existing test value 2. Raw logs retain the historical hard-coded `M3 diagnostics` label; these new logs were produced on this Linux instance.

The M3 rows below are the final state-gas implementation rows from [round 5](../round5.md). That machine had uncontrolled background load. This comparison changes CPU architecture, OS and storage, so it does not isolate a core-count effect.

## Baseline results

| Fixture / metric | M3 final run 1 / 2 | c8id 16-core run 1 / 2 |
| --- | ---: | ---: |
| Original: Execution msgs/s | 2,116,746 / 2,141,791 | 1,250,138 / 1,249,359 |
| Original: Execution + Commit msgs/s | 1,036,369 / 1,009,690 | 967,437 / 967,363 |
| Original: Execution ms/block | 23.621 / 23.345 | 39.996 / 40.021 |
| Original: Execution + Commit ms/block | 48.245 / 49.520 | 51.683 / 51.687 |
| Original: Commit mean ms | 24.624 / 26.175 | 11.687 / 11.666 |
| Original: Commit p99 ms | 320.137 / 673.444 | 57.660 / 58.776 |
| Original: End work ms/block | 0.764 / 0.762 | 1.071 / 1.047 |
| Original: Dirty lines/block | 149,136.9 / 149,136.9 | 149,136.9 / 149,136.9 |
| Original: Dirty leaves/block | 30,675.0 / 30,675.0 | 30,675.0 / 30,675.0 |
| Generated: Execution msgs/s | 1,041,899 / 1,070,423 | 680,516 / 673,981 |
| Generated: Execution + Commit msgs/s | 551,728 / 569,697 | 519,520 / 515,631 |
| Generated: Execution ms/block | 47.989 / 46.710 | 73.474 / 74.186 |
| Generated: Execution + Commit ms/block | 90.624 / 87.765 | 96.243 / 96.969 |
| Generated: Commit mean ms | 42.635 / 41.055 | 22.769 / 22.782 |
| Generated: Commit p99 ms | 553.987 / 384.821 | 98.279 / 99.721 |
| Generated: End work ms/block | 1.162 / 1.329 | 1.715 / 1.744 |
| Generated: Dirty lines/block | 243,122.9 / 243,122.9 | 243,122.9 / 243,122.9 |
| Generated: Dirty leaves/block | 71,734.7 / 71,734.7 | 71,734.7 / 71,734.7 |

Execution and combined ms/block are derived from printed integer rates and rounded to three decimals. Bytes written/block remain 9,544,761 original and 15,559,868 generated. Original logs: `original-host.txt`; generated logs: `generated-host.txt`.

| Original controls | M3 run 1 / 2 msgs/s | c8id run 1 / 2 msgs/s |
| --- | ---: | ---: |
| Plain | 7,736,832 / 8,035,341 | 5,313,678 / 5,260,836 |
| Inline gas + stack | 7,225,352 / 7,037,772 | 4,781,376 / 4,764,585 |

The single 8-core original replay measured 1,169,059 execution msgs/s and 873,611 including Commit; execution 42.769 ms/block, Commit mean 14.464 ms, p99 54.617 ms and end work 1.147 ms. Native hashes and counts passed. Selecting only subtest 0 means there is no second root comparison in that run. Compared with 16 cores, mean Commit is about 24% higher and combined throughput about 10% lower.

## Commit profile

The separate 600-block original profile passed all native hashes. Mean Commit was 11.684 ms (about 7.01 s total). The trace recorded no DB.rotate / sync.Cond.Wait backlog delay. Commit spent 439.78 ms in Msync and about 2.93 s waiting in hashDirty. Hash workers account for 19.23 sampled CPU seconds (13.96 leaf hashing plus 5.27 tree hashing); SHA-256 accounts for 18.05 sampled CPU seconds across Commit. Sealer workers account for 66.30 sampled CPU seconds, 54.73% of all 121.13 CPU seconds sampled. Their sealSection and disk sorting contributions are 45.23 and 21.06 CPU seconds.

CPU totals across workers, waits and syscall durations are different measures and must not be added. Nested CPU rows overlap. This profile shows no sealer-backlog stall like the M3 trace; background sealing still consumes substantial CPU. `commit-profile.txt`, `commit-cpu.txt`, `commit-sync.txt`, `commit-syscall.txt` and `sealer-cpu.txt` retain text evidence. Binary profiles remain only on the temporary instance.

## Execution diagnosis

The execution-labelled CPU profile recorded 14.45 s in unresolved JIT external code, 2.96 s inclusive in DB.Touch, 2.01 s in logEntry, and 0.46 s in finalization. These are sampled CPU times, not wall times or disjoint categories. Touch also waited about 2.65 s on its mutex during the 600-block trace. The native Go profile cannot split JIT guest work from the inline barrier itself.

The existing unsafe barrier controls provide a separate decomposition:

| Original control | Run 1 ms/block | Run 2 ms/block |
| --- | ---: | ---: |
| Metered guest | 10.736 | 10.852 |
| Inline tracking, no scalar callback | 17.923 | 17.987 |
| Inline tracking plus empty scalar callback | 21.446 | 21.946 |
| Host tracking without DB.Touch | 21.233 | 21.709 |
| Full host | 40.270 | 40.449 |

All control output hashes passed. The stub and empty modes retain bulk callbacks. The no-DB modes do not commit, so the full-versus-host difference also includes the effects of sealing and memory traffic on subsequent execution. It is not a standalone Touch latency measurement. These controls are unsafe for persistence and do not perform root replay checks; the separate full-host pairs do.

The diagnostic baseline records 2,367 / 2,404 minor faults and about 2 major faults per execution interval. getrusage is process-wide, so concurrent sealer and prefaulter faults are included. This counter alone does not identify faulted mappings; the address profile below does. The perf stat run recorded 960.68 billion cycles, 1,953.47 billion instructions and 4.20 billion branch misses for the complete two-replay test command, including its native oracle and setup. Its cache-misses event returned zero and is unvalidated. Raw counters: `diag64-perf.txt`; per-block data: `diag64.csv`.

The 64-byte diagnostic pair, with counters and CSV output, takes 40.611 / 40.723 ms execution versus 39.996 / 40.021 ms in the ordinary baseline. Use the diagnostic baseline for the following variants; do not mix that small pacing/instrumentation difference into a claimed optimization.

## Diagnostic variants on the original fixture

All rows use 16 physical cores. Each row contains two 600-block replays.

| Variant | Execution ms/block | Commit mean ms | Commit p99 ms | Combined msgs/s |
| --- | ---: | ---: | ---: | ---: |
| 64-byte tracking, 1 KiB leaves | 40.611 / 40.723 | 12.131 / 12.161 | 58.963 / 58.698 | 948,011 / 945,467 |
| 1 KiB tracking, 1 KiB leaves | 46.718 / 46.617 | 16.631 / 16.541 | 59.450 / 59.982 | 789,274 / 791,673 |
| 4 KiB tracking, 1 KiB leaves | 61.177 / 60.715 | 30.260 / 30.005 | 89.869 / 88.158 | 546,821 / 551,148 |
| 64-byte tracking, 4 KiB leaves | 36.916 / 36.758 | 12.930 / 12.858 | 69.846 / 71.451 | 1,003,091 / 1,007,739 |
| 64-byte tracking, no epoch rotation | 33.170 / 31.736 | 10.415 / 10.379 | 18.209 / 19.490 | 1,147,176 / 1,187,220 |
| 64-byte tracking, input bypass | 38.191 / 38.117 | 11.138 / 11.120 | 64.663 / 63.953 | 1,013,604 / 1,015,500 |

| Variant | Dirty tracking lines/block | Dirty hash leaves/block | Live journal MB/block | Minor faults/execution interval |
| --- | ---: | ---: | ---: | ---: |
| 64-byte tracking, 1 KiB leaves | 149,136.9 | 30,675.0 | 21.346 | 2367.4 / 2404.0 |
| 1 KiB tracking, 1 KiB leaves | 30,675.0 | 30,675.0 | 63.660 | 5430.4 / 5474.3 |
| 4 KiB tracking, 1 KiB leaves | 16,088.4 | 64,353.6 | 132.277 | 9250.4 / 9166.6 |
| 64-byte tracking, 4 KiB leaves | 149,136.9 | 16,088.4 | 21.346 | 1911.6 / 1857.6 |
| 64-byte tracking, no epoch rotation | 149,136.9 | 30,675.0 | 21.346 | 1765.9 / 1225.8 |
| 64-byte tracking, input bypass | 117,939.0 | 28,725.3 | 17.353 | 1990.1 / 1966.4 |

MB means decimal MB. Journal bytes are measured cursor deltas, including coalescing and the Commit record; they exclude compressed sealed files and epoch checkpoints. This differs from the uncoalesced upper bound in the ordinary TestMatcher output. Every pair passed native hashes, repeated roots and returned byte/leaf count checks. The first shared-target 4 KiB tracking attempt failed at block zero and is excluded; see COMMANDS.md for the Cargo target correction.

Larger tracking units lose on this workload. Reducing callbacks does not offset writing 3.0x or 6.2x as much live journal data. The 4 KiB leaf experiment keeps 64-byte tracking and the same journal volume. It improves execution modestly while slightly worsening Commit. It changes the state format and gas leaf count, so it is a diagnostic result, not a compatible production setting.

The no-seal experiment uses the exposed EpochBytes and EpochVersions options and verifies epoch 1 remained epoch 1 in both replays. It measures work with no background epoch sealing during these blocks. It does not eliminate eventual sealing work, and final close is excluded as in the baseline. Its combined result remains below 1.2M msgs/s.

The input experiment bypasses Touch only for the ABI input copy. Input remains in mapped state; this is not a safe separate scratch-memory design. Guest writes still use the barrier. Deterministic repeated roots do not prove complete state commitment, undo or recovery with these untracked writes. Treat the gain as a diagnostic estimate requiring a real scratch-memory contract before use.

## Ideal Commit overlap bound

Using each block's recorded execution and Commit durations in `diag64.csv`, sum `max(execution, Commit)` instead of their sum. Run 1 changes from 52.742 to 41.346 ms/block, or 1,209,310 msgs/s; run 2 changes from 52.884 to 41.502 ms/block, or 1,204,765 msgs/s. This assumes perfect overlap without resource contention or dependency costs. It is an upper bound, not an implemented pipeline. Overlap alone cannot reach 2M msgs/s; the current execution phase already exceeds the 25 ms/block budget for that target.

## Fault addresses and writeback isolation

A second minor-fault profile records data addresses and non-executable mmap events (`perf record -d --data-mmap -e minor-faults -c 100`). Its two full replays passed native hashes, repeated roots and counts. Of 29,849 sampled events, 81.56% resolve to the state file, about 7% to live log chunks, 7.81% to anonymous mappings, 2.62% to heap and 0.29% to the tree. No samples were lost. These are whole-command samples, including setup, not fault latencies. See `fault-addresses-dso.txt`. The earlier call-stack profile places most faults inside JIT guest execution, and also identifies the Go prefaulter.

To test writeback's contribution, a separate no-seal pair temporarily raises dirty-background and dirty limits to 32/48 GiB, sets dirty expiry to one hour and disables periodic writeback. [The kernel documentation](https://docs.kernel.org/admin-guide/sysctl/vm.html#dirty-background-bytes) defines these writeback thresholds and timers. The instance's original settings were saved and restored immediately afterward (`writeback-settings.txt`, `writeback-restored.txt`, `writeback-restore-verified.txt`). This does not change any production source or the baseline measurements.

| No-seal condition | Execution ms/block | Commit mean ms | Combined msgs/s | Minor faults/execution interval |
| --- | ---: | ---: | ---: | ---: |
| Normal kernel writeback | 33.170 / 31.736 | 10.415 / 10.379 | 1,147,176 / 1,187,220 | 1,765.9 / 1,225.8 |
| Delayed kernel writeback | 28.694 / 28.612 | 10.084 / 10.083 | 1,289,392 / 1,292,154 | 254.6 / 251.6 |

Both delayed-writeback replays passed every hash/root/count check and kept epoch 1. This supports state-file write faults as a material part of the cost, in addition to inline tracking, callbacks, journal copy and sealer contention. It does not measure fault latency directly or prove an isolated explanation for the M3/x86 difference. These settings defer writeback; they are a diagnostic upper bound, not a production durability recommendation. Even this configuration remains below 1.3M combined msgs/s.

## Ranked work toward 2M msgs/s

None of the tested full-host configurations reaches 2M combined msgs/s. That target requires 25 ms/block including commitment.

1. Reduce work in the first-write path while retaining fine tracking and undo semantics. The no-DB control takes about 21 ms execution; full execution takes about 40 ms. Inline tracking and scalar callbacks add about 11 ms over the metered control. A design that batches first-write handling and avoids repeated state-file write faults is the largest remaining opportunity. Its safe, end-to-end gain is unmeasured; the unsafe control is not a deployable replacement.
2. Reduce sealing interference and state writeback faults without dropping persistence work. Moving sealing beyond the run improves combined throughput to 1.15-1.19M; also delaying kernel writeback reaches 1.29M. The profile's Touch mutex wait is about 4.4 ms/block, and sealer workers consume 54.7% of sampled CPU. This identifies work to remove from the foreground path, but eventual work and durability still need to be paid for.
3. Consider Commit overlap only after lowering execution cost. Ideal overlap at the measured baseline is capped near 1.21M. It cannot compensate for 40 ms execution, and real overlap may contend for the same memory and CPU resources.
4. Treat input scratch and hash-leaf changes as smaller experiments. Each improves combined throughput to roughly 1.01M here. Scratch needs a defined persistence/undo contract; changing hash leaves changes the state format. Neither is a route to 2M alone. Do not enlarge tracking units on this workload: the extra journal volume outweighs fewer callbacks.

The profile averages about 3.8 CPUs over its 31.85-second interval; a large sealer share of sampled CPU does not mean all 16 cores are saturated. No production default was changed, and no tested gains should be added together as independent savings.

## Verification and lifecycle

`go vet ./...` and `go test -count=1 -v -timeout=45m ./...` passed on the instance before measurements and again after adding the opt-in diagnostic. Original and generated full-host pairs each passed 600 native hashes, all repeated state roots and byte/leaf counts. Every diagnostic variant pair also passed those checks. The 8-core single replay has native hash/count checks only. Unsafe barrier controls and input bypass have the explicit limitations above. No race/fuzz or crash-recovery suite was run for these diagnostic copies.

Instance `i-07b4df53980beec45` launched at **2026-10-15 14:14:46 JST** (05:14:46 UTC). Boot user data scheduled shutdown at **17:15:02 JST** (08:15:02 UTC); `host.txt` records its epoch timestamp. Instance-initiated shutdown behavior is `terminate`, verified in `shutdown-behavior.json`. Root volume delete-on-termination was set in the launch request. No SSH rule was added, and no other instance was accessed or modified.

At 14:50 JST the coordinator requested an immediate handoff, keeping the box available for follow-up work, and explicitly took over termination verification. The instance is still running at handoff (`handoff-time.txt`). **Termination is unverified and owned by the coordinator.** The SSH helper `/Users/ilia/.cache/cx/wasmcloud-ssh` and all box state remain available. The estimated Linux on-demand rate in us-west-2 is **$1.7741/hour**, corroborated by [Spare Cores](https://sparecores.com/server/aws/c8id.8xlarge) and [AWS Pricing](https://aws-pricing.com/c8id.8xlarge.html). The scheduled launch-to-shutdown duration is 3.0044 hours, giving a projected compute cost of **$5.33**. This is a scheduled-duration estimate, not observed termination cost; it excludes EBS, public IPv4 and taxes and is not an AWS bill.
