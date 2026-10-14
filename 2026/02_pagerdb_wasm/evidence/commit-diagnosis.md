# Commit diagnosis

Single-process M3 diagnostics under background load. No PagerDB code was changed.
CPU and execution-trace profiles cover blocks 580 through 599 after replaying the
first 580 blocks. They are diagnostic runs with profiler overhead, not replacements
for the unprofiled 600-block measurements. All replayed output hashes passed.

| Per profiled block | Local original | Generated |
|---|---:|---:|
| Mean dirty pages | 18,174.3 | 45,317.8 |
| Mean Commit elapsed, ms | 355.528 | 497.620 |
| Index construction, sampled CPU ms | 4.0 | 2.5 |
| Final-byte copy, sampled CPU ms | 11.0 | 27.0 |
| Leaf/path hashing and setup, sampled CPU ms | 32.5 | 84.0 |
| Block CRC, sampled CPU ms | 15.0 | 33.5 |
| Rotation, sampled CPU ms | 1.0 | 2.0 |
| Other sampled Commit CPU ms | 1.0 | 1.0 |
| Sealer backpressure, elapsed wait ms | 238.5 | 263.0 |
| Rotation Msync, syscall elapsed ms | 55.0 | 80.5 |
| Rotation clone/setup, syscall elapsed ms | about 0.5 | about 8.5 |
| Hash worker waiting, elapsed ms | 29.5 | 69.5 |
| Final-byte worker waiting, elapsed ms | 8.0 | 19.5 |

CPU entries sum CPU across worker goroutines. They are **not** elapsed phase
measurements and must not be added to the wait/syscall rows. CPU sampling has
10 ms granularity over 20 blocks. The residual includes scheduling and runtime
work. The native library has no public phase timers; exact wall time separately
for index, copy and hash is unverified. Trace wait and syscall times are measured,
not inferred from CPU samples.

The dominating wait is `DB.rotate -> sync.Cond.Wait`: rotation waits when two
seals are pending. The trace records 4.77 s original and 5.26 s generated across
20 commits. Rotation also performs `Msync(MS_SYNC)` before cloning the state:
1.10 s original and 1.61 s generated. Slow commits line up with rotation cadence.
For example, generated blocks 583/586/592 took 1.246/1.636/1.406 s, while their
neighbors generally took 159 to 218 ms; original block 580 took 4.064 s and its
next three blocks took 95/61/59 ms. These tails depend on background load and I/O.

The 12 ms `BenchmarkCommit50k` figure does not scale by leaf count alone. In pinned
PagerDB `bench_test.go:73`, that benchmark writes **64 bytes** at each of 50,000
leaves and sets `EpochBytes` and `EpochVersions` to `1<<40`, avoiding rotation and
sealing. Its final-byte copy is 3.2 MB and its log is about 8 MB per block. Here
each `Touch` records 4,096 old bytes and 4,096 final bytes plus a 32-byte header.
At the full-run averages, generated blocks copy 142.3 MB and checksum about
285.7 MB of log; original blocks copy 65.9 MB and checksum about 132.3 MB. Default
1 GiB epochs rotate about every 4 generated or 8 original blocks at those average
sizes, more often at the end of the generated run. The microbenchmark excludes
this persistent sealing workload.

## Why the page count is large

The barrier touches an entire 4 KiB page for every store, including temporary
stores and stores whose final value is unchanged. All Rust stack, allocator,
book slabs/indexes/price-level arrays, input buffers, fills and output scratch are
inside the mapped region. The matcher copies input twice (host input and its own
input buffer) and updates separate allocations for each symbol. Counting only
persistent changed bytes therefore underestimates the work of this prototype.

Measured on block 599:

| Measure | Local original | Generated |
|---|---:|---:|
| Touched pages | 17,237 | 45,849 |
| Pages with final byte changes | 15,803 | 42,006 |
| Touched pages with no final changes | 1,434 | 3,843 |
| Final changed bytes | 1,687,676 | 2,667,872 |
| Whole-page bytes tracked | 70,602,752 | 187,797,504 |
| Page bytes / final changed bytes | 41.8x | 70.4x |
| Dirty pages in successive 64 MiB address bands | 4930, 4298, 2744, 2416, 2326, 523 | 11984, 11888, 12726, 9251 |

A separate full-fixture scan found 8,947 distinct symbols per generated block
on average, versus 2,180 in the original (block 599: 8,992 versus 2,491). The
generator samples the global symbol distribution without source temporal
clustering. More per-symbol book allocations therefore participate in each block.
This directly confirms broad address spread and whole-page amplification. It does
not assign each dirty page to an individual Rust allocation; allocator-versus-book
versus-output attribution remains unverified. No allocator or barrier optimization
was attempted.

Reproduction and raw CPU/trace summaries are in `commands.md` and the
`*-mature-{profile,cpu,sync,syscall}.txt` files. Raw binary profiles and traces are
outside the repository under `~/.cache/cx/wasmhost.*`.
