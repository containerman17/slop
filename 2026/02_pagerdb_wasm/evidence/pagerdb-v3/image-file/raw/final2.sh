. /mnt/nvme/env.sh
R=/mnt/nvme/final
date > $R/start.txt
(cd /mnt/nvme/pagerdb; export GOWORK=off; uname -r; go version; go vet ./... && echo VET_OK; go test -count=1 -timeout=20m ./...; go test -race -count=1 -timeout=30m ./...; PAGERDB_CRASH_ITERS=1000 go test -count=1 -run "^TestCrashRecovery\$" -v -timeout=60m .) > $R/core-checks.txt 2>&1
for gib in 4 16; do for v in before after; do d=/mnt/nvme/pagerdb; [ $v = before ] && d=/mnt/nvme/pagerdb-before
 (cd $d && GOWORK=off PAGERDB_MEM_GIB=$gib taskset -c 0-15 go test -count=1 -run "^TestMemoryFootprint\$" -v -timeout=30m .) > $R/mem-$v-${gib}g.txt 2>&1; done; done
(export GOFLAGS= PAGERDB_DISK=nvme1n1p2 GOWORK=/mnt/nvme/go.work
cd /mnt/nvme/pagerdb-diag; go vet ./... > $R/diag-core-checks.txt 2>&1 && echo VET_OK >> $R/diag-core-checks.txt
PAGERDB_EVM_RETAIN=32 taskset -c 0-15 go test -run "^\$" -bench "^BenchmarkEVM\$" -benchtime=700x -count=2 -timeout=30m -v > $R/evm.txt 2>&1
cd /mnt/nvme/repo-diag; { go vet ./... && echo VET_OK; taskset -c 0-15 go test -count=1 -timeout=30m ./...; } > $R/host-checks.txt 2>&1
PAGERWASM_FIXTURE=/mnt/nvme/original.bin taskset -c 0-15 go test -run "^TestMatcher\$" -count=1 -v -timeout=30m . > $R/original.txt 2>&1
PAGERWASM_FIXTURE=/mnt/nvme/synthetic.bin taskset -c 0-15 go test -run "^TestMatcher\$" -count=1 -v -timeout=30m . > $R/synthetic.txt 2>&1)
(cd /mnt/nvme/repo-pinned; export GOWORK=off
{ go vet ./... && echo VET_OK; taskset -c 0-15 go test -count=1 -timeout=30m ./...; } > $R/pinned-host-checks.txt 2>&1
PAGERWASM_FIXTURE=/mnt/nvme/original.bin taskset -c 0-15 go test -run "^TestMatcher\$" -count=1 -v -timeout=30m . > $R/pinned-original.txt 2>&1
PAGERWASM_FIXTURE=/mnt/nvme/synthetic.bin taskset -c 0-15 go test -run "^TestMatcher\$" -count=1 -v -timeout=30m . > $R/pinned-synthetic.txt 2>&1)
/mnt/nvme/run.sh swap-half-s1-off 12800000000 "0.01:off:0:500:500" PAGERDB_DISK=nvme1n1p2
/mnt/nvme/run.sh swap-fits-s1-off none "0.01:off:0:500:500" PAGERDB_DISK=nvme1n1p2
date > $R/done.txt
