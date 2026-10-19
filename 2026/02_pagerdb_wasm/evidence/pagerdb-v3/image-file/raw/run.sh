#!/bin/bash
export TZ=Asia/Tokyo
# usage: run.sh NAME CAP_BYTES|none RUNS [extra VAR=value...]
name=$1 cap=$2 runs=$3; shift 3
out=/mnt/nvme/final/$name
inner=(bash -c ". /mnt/nvme/env.sh; cd /mnt/nvme/pagerdb; export GOWORK=off; exec env \"\$@\" taskset -c 0-15 go test -run \"^TestSwapEVM\\\$\" -count=1 -v -timeout=0 ." _ PAGERDB_SWAP_RUNS="$runs" PAGERDB_SWAP_CSV=$out.csv "$@")
{ echo "# start $(date) name=$name cap=$cap runs=$runs extra=$*"; grep -E "pswpin|pswpout" /proc/vmstat; } > $out.txt
if [ "$cap" = none ]; then "${inner[@]}"; else sudo systemd-run --scope --quiet -p MemoryMax=$cap -p MemorySwapMax=infinity --uid=ubuntu --gid=ubuntu "${inner[@]}"; fi >> $out.txt 2>&1
rc=$?
{ echo "# end exit=$rc $(date)"; grep -E "pswpin|pswpout" /proc/vmstat; free -m; } >> $out.txt
