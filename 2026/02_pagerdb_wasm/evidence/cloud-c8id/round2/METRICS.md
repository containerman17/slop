# Complete measurement matrix

Reproduce with `python3 evidence/cloud-c8id/round2/analyze.py`. Values are run 1 / run 2. `small` disables THP; `huge` requests THP; `sync` adds fdatasync; `parallel` parallelizes snapshots. G is serial, H overlaps Commit, I uses 4 KiB leaves.

Verified 25 measured entries, 50 full replays, 30000 native-checked blocks in passing logs.
Dirty-line sequences match across all configurations; dirty-leaf sequences and write-byte counts match each layout.

All 1024-byte configurations share all 600 roots. Root-sequence SHA256: `675960bbfcda30bd77c59e6061d5c76f929d4dd13982bcb76d58453fb07e4861`.

All 4096-byte configurations share all 600 roots. Root-sequence SHA256: `e5f4ead306d0904e4926ffa4c4a85a9b980303db334b277ee64af8e35b037719`.

All 512-byte configurations share all 600 roots. Root-sequence SHA256: `d89a687c791ecf4c7819519e5778f86877d2106c88ad7b6d658ece572e623ed4`.


| Configuration | Execution ms | Commit ms | Snapshot ms | Wall Mmsgs/s | Minor faults/block |
| --- | ---: | ---: | ---: | ---: | ---: |
| ideal-g-1k-huge-sync | 22.114 / 22.096 | 103.329 / 102.892 | 0.000 / 0.000 | 0.395 / 0.397 | 244.5 / 208.8 |
| ideal-g-1k-huge | 22.020 / 21.897 | 75.946 / 75.749 | 0.000 / 0.000 | 0.505 / 0.506 | 254.3 / 242.5 |
| ideal-g-1k-small-sync | 23.920 / 23.947 | 103.424 / 103.359 | 0.000 / 0.000 | 0.389 / 0.389 | 378.3 / 365.5 |
| ideal-g-1k-small | 24.612 / 24.175 | 78.254 / 77.077 | 0.000 / 0.000 | 0.481 / 0.489 | 378.0 / 366.0 |
| ideal-h-1k-huge-sync | 24.117 / 23.699 | 103.550 / 103.414 | 3.417 / 3.338 | 0.466 / 0.467 | 832.4 / 819.5 |
| ideal-h-1k-huge | 23.600 / 23.643 | 76.706 / 76.936 | 3.216 / 3.229 | 0.623 / 0.621 | 832.1 / 787.3 |
| ideal-h-1k-small-sync | 25.852 / 25.919 | 103.055 / 102.980 | 3.349 / 3.362 | 0.468 / 0.469 | 955.8 / 934.3 |
| ideal-h-1k-small | 25.770 / 25.804 | 76.629 / 76.486 | 3.225 / 3.205 | 0.622 / 0.624 | 955.8 / 910.5 |
| ideal-i-4k-huge-sync | 22.123 / 21.917 | 78.433 / 78.037 | 0.000 / 0.000 | 0.492 / 0.495 | 244.3 / 235.9 |
| ideal-i-4k-huge | 21.827 / 21.872 | 50.938 / 50.975 | 0.000 / 0.000 | 0.678 / 0.677 | 244.4 / 225.6 |
| ideal-i-4k-small-sync | 24.035 / 24.056 | 78.624 / 78.710 | 0.000 / 0.000 | 0.482 / 0.482 | 367.9 / 349.0 |
| ideal-i-4k-small | 23.734 / 23.780 | 51.432 / 51.584 | 0.000 / 0.000 | 0.657 / 0.655 | 367.9 / 349.0 |
| ideal-log-g-huge-sync | 24.056 / 24.159 | 20.274 / 20.371 | 0.000 / 0.000 | 1.101 / 1.094 | 497.9 / 446.9 |
| ideal-log-g-huge | 23.899 / 23.936 | 15.246 / 15.239 | 0.000 / 0.000 | 1.243 / 1.239 | 498.1 / 447.3 |
| ideal-log-g-small | 25.856 / 25.837 | 15.247 / 15.269 | 0.000 / 0.000 | 1.186 / 1.182 | 621.7 / 570.4 |
| ideal-log-h-huge-parallel-sync | 24.902 / 24.610 | 17.072 / 17.061 | 1.025 / 1.005 | 1.849 / 1.866 | 834.0 / 615.0 |
| ideal-log-h-huge-parallel | 24.742 / 24.590 | 12.035 / 12.032 | 1.009 / 0.997 | 1.863 / 1.869 | 833.8 / 624.5 |
| ideal-log-h-huge | 24.208 / 24.129 | 12.191 / 12.203 | 3.574 / 3.601 | 1.731 / 1.730 | 834.2 / 614.6 |
| ideal-log-h-small | 26.429 / 26.207 | 12.169 / 12.170 | 3.585 / 3.614 | 1.607 / 1.613 | 958.0 / 748.8 |
| ideal-root-g-huge | 22.044 / 22.065 | 5.086 / 5.076 | 0.000 / 0.000 | 1.774 / 1.773 | 256.0 / 245.2 |
| ideal-root-h-huge-parallel-1024-repeat | 22.838 / 22.813 | 5.174 / 5.170 | 0.903 / 0.906 | 2.016 / 2.017 | 833.8 / 820.9 |
| ideal-root-h-huge-parallel-4096 | 23.017 / 22.906 | 7.299 / 7.315 | 1.442 / 1.432 | 1.966 / 1.976 | 835.1 / 695.8 |
| ideal-root-h-huge-parallel-512 | 23.160 / 22.600 | 5.277 / 5.279 | 0.743 / 0.751 | 2.002 / 2.040 | 852.9 / 810.8 |
| ideal-root-h-huge-parallel | 22.748 / 22.672 | 5.137 / 5.147 | 0.896 / 0.897 | 2.024 / 2.022 | 826.0 / 764.2 |
| ideal-root-h-huge | 22.576 / 22.426 | 5.259 / 5.261 | 3.063 / 3.032 | 1.872 / 1.882 | 834.2 / 821.9 |

| Configuration | Hash + fold ms | Pwrite ms | fdatasync ms | Pack ms | Timed execution + Commit Mmsgs/s |
| --- | ---: | ---: | ---: | ---: | ---: |
| ideal-g-1k-huge-sync | 5.229 / 5.240 | 70.659 / 70.245 | 27.441 / 27.407 | 0.000 / 0.000 | 0.399 / 0.400 |
| ideal-g-1k-huge | 5.193 / 5.165 | 70.753 / 70.584 | 0.000 / 0.000 | 0.000 / 0.000 | 0.510 / 0.512 |
| ideal-g-1k-small-sync | 5.254 / 5.221 | 70.735 / 70.722 | 27.436 / 27.416 | 0.000 / 0.000 | 0.393 / 0.393 |
| ideal-g-1k-small | 5.184 / 5.230 | 73.069 / 71.848 | 0.000 / 0.000 | 0.000 / 0.000 | 0.486 / 0.494 |
| ideal-h-1k-huge-sync | 5.292 / 5.250 | 70.477 / 70.519 | 27.781 / 27.645 | 0.000 / 0.000 | use wall rate |
| ideal-h-1k-huge | 5.291 / 5.307 | 71.415 / 71.629 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |
| ideal-h-1k-small-sync | 5.217 / 5.249 | 70.237 / 70.112 | 27.601 / 27.619 | 0.000 / 0.000 | use wall rate |
| ideal-h-1k-small | 5.268 / 5.287 | 71.362 / 71.199 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |
| ideal-i-4k-huge-sync | 5.660 / 5.593 | 45.099 / 44.851 | 27.674 / 27.593 | 0.000 / 0.000 | 0.497 / 0.500 |
| ideal-i-4k-huge | 5.545 / 5.564 | 45.394 / 45.411 | 0.000 / 0.000 | 0.000 / 0.000 | 0.687 / 0.686 |
| ideal-i-4k-small-sync | 5.672 / 5.664 | 45.270 / 45.330 | 27.682 / 27.717 | 0.000 / 0.000 | 0.487 / 0.487 |
| ideal-i-4k-small | 5.616 / 5.640 | 45.816 / 45.944 | 0.000 / 0.000 | 0.000 / 0.000 | 0.665 / 0.663 |
| ideal-log-g-huge-sync | 5.351 / 5.328 | 6.587 / 6.643 | 5.023 / 5.029 | 3.313 / 3.371 | 1.128 / 1.123 |
| ideal-log-g-huge | 5.367 / 5.354 | 6.596 / 6.586 | 0.000 / 0.000 | 3.283 / 3.299 | 1.277 / 1.276 |
| ideal-log-g-small | 5.370 / 5.367 | 6.593 / 6.606 | 0.000 / 0.000 | 3.283 / 3.296 | 1.216 / 1.216 |
| ideal-log-h-huge-parallel-sync | 5.263 / 5.246 | 6.717 / 6.723 | 5.028 / 5.028 | 0.064 / 0.064 | use wall rate |
| ideal-log-h-huge-parallel | 5.243 / 5.231 | 6.727 / 6.736 | 0.000 / 0.000 | 0.065 / 0.065 | use wall rate |
| ideal-log-h-huge | 5.366 / 5.353 | 6.762 / 6.784 | 0.000 / 0.000 | 0.064 / 0.065 | use wall rate |
| ideal-log-h-small | 5.371 / 5.355 | 6.735 / 6.751 | 0.000 / 0.000 | 0.063 / 0.064 | use wall rate |
| ideal-root-g-huge | 5.078 / 5.067 | 0.009 / 0.008 | 0.000 / 0.000 | 0.000 / 0.000 | 1.843 / 1.842 |
| ideal-root-h-huge-parallel-1024-repeat | 5.165 / 5.161 | 0.009 / 0.009 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |
| ideal-root-h-huge-parallel-4096 | 7.289 / 7.305 | 0.010 / 0.010 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |
| ideal-root-h-huge-parallel-512 | 5.269 / 5.270 | 0.009 / 0.009 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |
| ideal-root-h-huge-parallel | 5.128 / 5.138 | 0.009 / 0.009 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |
| ideal-root-h-huge | 5.249 / 5.252 | 0.009 / 0.009 | 0.000 / 0.000 | 0.000 / 0.000 | use wall rate |

| Configuration | Write bytes/block | Write calls/block | Buffered write MB/s |
| --- | ---: | ---: | ---: |
| ideal-g-1k-huge-sync | 31411191 / 31411191 | 13608.878 / 13608.878 | 445 / 447 |
| ideal-g-1k-huge | 31411191 / 31411191 | 13608.878 / 13608.878 | 444 / 445 |
| ideal-g-1k-small-sync | 31411191 / 31411191 | 13608.878 / 13608.878 | 444 / 444 |
| ideal-g-1k-small | 31411191 / 31411191 | 13608.878 / 13608.878 | 430 / 437 |
| ideal-h-1k-huge-sync | 31411191 / 31411191 | 13608.878 / 13608.878 | 446 / 445 |
| ideal-h-1k-huge | 31411191 / 31411191 | 13608.878 / 13608.878 | 440 / 439 |
| ideal-h-1k-small-sync | 31411191 / 31411191 | 13608.878 / 13608.878 | 447 / 448 |
| ideal-h-1k-small | 31411191 / 31411191 | 13608.878 / 13608.878 | 440 / 441 |
| ideal-i-4k-huge-sync | 65898114 / 65898114 | 7356.203 / 7356.203 | 1461 / 1469 |
| ideal-i-4k-huge | 65898114 / 65898114 | 7356.203 / 7356.203 | 1452 / 1451 |
| ideal-i-4k-small-sync | 65898114 / 65898114 | 7356.203 / 7356.203 | 1456 / 1454 |
| ideal-i-4k-small | 65898114 / 65898114 | 7356.203 / 7356.203 | 1438 / 1434 |
| ideal-log-g-huge-sync | 31533939 / 31533939 | 1.000 / 1.000 | 4788 / 4747 |
| ideal-log-g-huge | 31533939 / 31533939 | 1.000 / 1.000 | 4781 / 4788 |
| ideal-log-g-small | 31533939 / 31533939 | 1.000 / 1.000 | 4783 / 4773 |
| ideal-log-h-huge-parallel-sync | 31533939 / 31533939 | 1.000 / 1.000 | 4695 / 4691 |
| ideal-log-h-huge-parallel | 31533939 / 31533939 | 1.000 / 1.000 | 4688 / 4682 |
| ideal-log-h-huge | 31533939 / 31533939 | 1.000 / 1.000 | 4664 / 4648 |
| ideal-log-h-small | 31533939 / 31533939 | 1.000 / 1.000 | 4682 / 4671 |
| ideal-root-g-huge | 32 / 32 | 1.000 / 1.000 | 4 / 4 |
| ideal-root-h-huge-parallel-1024-repeat | 32 / 32 | 1.000 / 1.000 | 4 / 4 |
| ideal-root-h-huge-parallel-4096 | 32 / 32 | 1.000 / 1.000 | 3 / 3 |
| ideal-root-h-huge-parallel-512 | 32 / 32 | 1.000 / 1.000 | 4 / 4 |
| ideal-root-h-huge-parallel | 32 / 32 | 1.000 / 1.000 | 4 / 4 |
| ideal-root-h-huge | 32 / 32 | 1.000 / 1.000 | 4 / 4 |
