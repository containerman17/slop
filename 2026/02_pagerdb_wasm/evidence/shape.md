# Aggregate fixture shape

Measured once on 30,000,000 local source messages and once on 30,000,000 generated
messages (600 blocks each). Only aggregate numbers appear here. No source records,
prices, identifiers, symbol names, symbol mappings or output hashes are included.
The generator uses seed 1; commands are in `commands.md`.

| Measure | Local original | Generated |
|---|---:|---:|
| Add count | 15,386,807 | 15,389,702 |
| Cancel count | 12,825,097 | 12,822,632 |
| Replace count | 807,780 | 807,708 |
| Modify count | 980,316 | 979,958 |
| Buy / sell count | 15,027,297 / 14,972,703 | 15,029,174 / 14,970,826 |
| Symbols | 9,812 | 9,812 |
| Distinct symbols per block, mean | 2,180.393 | 8,947.035 |
| Distinct symbols per block, p50 / p90 / p99 | 2,354 / 2,714 / 3,538 | 8,980 / 9,027 / 9,052 |
| Top 1 symbol share | 1.594023% | 1.591777% |
| Top 10 symbol share | 10.251797% | 10.241290% |
| Top 100 symbol share | 36.354957% | 36.365347% |
| Distinct price levels per symbol, p50 / p90 / p99 | 47 / 432 / 1,957 | 80 / 90 / 470 |
| Active levels before a message, p50 / p90 / p99 | 50 / 1,095 / 3,048 | 54 / 1,615 / 3,389 |
| Absolute distance from midpoint, bps, p50 / p90 / p99 | 11 / 1,203 / 10,310 | 96 / 4,862 / 40,000 |
| Quantity, p50 / p90 / p99 | 100 / 1,000 / 7,800 | 80 / 1,000 / 9,900 |
| Quantity mean | 548.151 | 758.321 |
| Crossing Add share | 4.277002% | 4.401593% |
| Cancel delay in messages, p50 / p90 / p99 | 2,879 / 1,040,436 / 12,640,200 | 2,661 / 963,278 / 11,513,906 |

Definitions: price levels are distinct submitted prices per symbol across all
priced operations. Active levels and bid/ask quotes come from the native matcher's
book immediately before each message. Midpoint distance includes Add/Replace/Modify
only when both quotes exist. Crossing Adds are Adds that actually produce a fill.
Cancel delay is the number of intervening global message positions since the last
observed Add/Replace of that symbol and id; this is not wall-clock time.

The generated operation mix, symbol skew and cancel delays approximate the source.
The price and quantity distributions do not match closely. The generated workload
is a reproducible correctness fixture and a separate diagnostic workload, not a
statistical substitute for the source data. Its higher dirty-page count must not
be presented as the source workload's storage cost.

The generator preserves global symbol skew but not temporal clustering: it touches
4.1 times as many symbols per block. This is a measured limitation, not a claim of
source-like per-block locality.
