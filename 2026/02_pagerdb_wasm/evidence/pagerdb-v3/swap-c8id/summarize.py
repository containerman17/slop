# Usage: python3 summarize.py raw/*.txt  -> one Markdown row per SWAPRESULT.
import json, re, sys
print("| file | cap | run | access | prefetch | warmup+blocks | writes/s | write p50/p99 ms | commit p50/p99 ms | prefetch wait p50 ms | majflt/block | pswpin/block | pswpout/block | AnonHugePages GiB | swap GiB | fill s |")
print("|" + "---|" * 16)
for path in sys.argv[1:]:
    text = open(path).read()
    cap = re.search(r"cap=(\S+)", text).group(1)
    cap = cap if cap == "none" else f"{int(cap) / 1e9:g} GB"
    fill = re.search(r"SWAPFILL .*?seconds=([0-9.]+)", text)
    for line in text.splitlines():
        if "SWAPRESULT " not in line:
            continue
        d = json.loads(line.split("SWAPRESULT ", 1)[1])
        m = d["mem"]
        access = "uniform" if d["sigma_fraction"] == 0 else f"sigma {d['sigma_fraction'] * 100:g}%"
        mode = d["mode"] if d["mode"] == "off" else f"{d['mode']} {d['goroutines']}"
        print(f"| {path.split('/')[-1][:-4]} | {cap} | {d['run']} | {access} | {mode} | {d['warmup']}+{d['blocks']} | {d['writes_per_s']:,.0f} | "
              f"{d['p50_write_ms']:.1f}/{d['p99_write_ms']:.1f} | {d['p50_commit_ms']:.1f}/{d['p99_commit_ms']:.1f} | {d['p50_wait_ms']:.1f} | "
              f"{d['majflt_per_block']:,.0f} | {d['pswpin_per_block']:,.0f} | {d['pswpout_per_block']:,.0f} | "
              f"{m.get('AnonHugePagesKiB', 0) / 2**20:.1f} | {m.get('SwapKiB', 0) / 2**20:.1f} | {fill.group(1) if fill and d['run'] == 0 else ''} |")
