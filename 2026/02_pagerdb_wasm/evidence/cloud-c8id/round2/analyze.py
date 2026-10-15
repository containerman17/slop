#!/usr/bin/env python3
"""Verify downloaded replays and render their measurements from raw logs."""
import csv
import hashlib
import re
from pathlib import Path

folder = Path(__file__).resolve().parent
reference = {}
line_reference = None
leaf_counts = {}
results = []
for log in sorted(folder.glob('ideal-*.txt')):
    runs = []
    for line in log.read_text().splitlines():
        if 'IDEAL run=' in line:
            row = dict(re.findall(r'(\w+)=([^\s]+)', line))
            if row['blocks'] == '600':
                runs.append(row)
    if not runs:
        continue
    assert len(runs) == 2 and [r['run'] for r in runs] == ['0', '1'], log
    assert '\nPASS\n' in log.read_text(), log
    for row in runs:
        path = folder / f'{log.stem}-{row["run"]}.csv'
        rows = list(csv.DictReader(path.open()))
        assert len(rows) == 600, path
        assert [int(r['block']) for r in rows] == list(range(600)), path
        roots = [r['root'] for r in rows]
        leaf = row['leaf']
        line_counts = [int(r['dirty_lines']) for r in rows]
        counts = [int(r['dirty_leaves']) for r in rows]
        if line_reference is None:
            line_reference = line_counts
        else:
            assert line_counts == line_reference, f'dirty lines differ: {path}'
        if leaf in leaf_counts:
            assert counts == leaf_counts[leaf], f'dirty leaves differ: {path}'
        else:
            leaf_counts[leaf] = counts
        for values, field in ((line_counts, 'dirty_lines_block'), (counts, 'dirty_leaves_block')):
            assert abs(sum(values)/600-float(row[field])) < 0.00051, (path, field)
        expected_bytes = 32 if row.get('root_only') == 'true' else sum(counts)/600*int(leaf)
        if row.get('packed_log') == 'true':
            expected_bytes += sum(counts)/600*4+48
        assert abs(expected_bytes-float(row['write_bytes_block'])) < 0.00051, (path, 'write bytes')
        if leaf in reference:
            assert roots == reference[leaf], f'root sequence differs: {path}'
        else:
            reference[leaf] = roots
        assert roots[-1] == row['final_root'], path
        for field in ('execution_ms', 'snapshot_ms', 'hash_ms', 'write_ms', 'fdatasync_ms', 'pack_ms'):
            mean = sum(float(r.get(field, 0)) for r in rows) / len(rows)
            assert abs(mean - float(row.get(field, 0))) < 0.000002, (path, field)
        total = sum(float(row.get(f, 0)) for f in ('hash_ms', 'write_ms', 'fdatasync_ms', 'pack_ms'))
        assert abs(total-float(row['commit_ms'])) < 0.000003, path
        assert abs(30_000_000/float(row['wall_s'])-float(row['wall_msgs_s'])) < 1, path
    results.append((log.stem, runs))

print('# Complete measurement matrix\n')
print('Reproduce with `python3 evidence/cloud-c8id/round2/analyze.py`. Values are run 1 / run 2. `small` disables THP; `huge` requests THP; `sync` adds fdatasync; `parallel` parallelizes snapshots. G is serial, H overlaps Commit, I uses 4 KiB leaves.\n')
print(f'Verified {len(results)} measured entries, {len(results)*2} full replays, {len(results)*1200} native-checked blocks in passing logs.')
print('Dirty-line sequences match across all configurations; dirty-leaf sequences and write-byte counts match each layout.\n')
for leaf, roots in sorted(reference.items()):
    digest = hashlib.sha256(b''.join(bytes.fromhex(r) for r in roots)).hexdigest()
    print(f'All {leaf}-byte configurations share all 600 roots. Root-sequence SHA256: `{digest}`.\n')

def pair(runs, field, divisor=1, decimals=3):
    return ' / '.join(f'{float(r.get(field, 0))/divisor:.{decimals}f}' for r in runs)

print('\n| Configuration | Execution ms | Commit ms | Snapshot ms | Wall Mmsgs/s | Minor faults/block |')
print('| --- | ---: | ---: | ---: | ---: | ---: |')
for name, runs in results:
    print(f'| {name} | {pair(runs,"execution_ms")} | {pair(runs,"commit_ms")} | {pair(runs,"snapshot_ms")} | {pair(runs,"wall_msgs_s",1e6)} | {pair(runs,"minor_faults_block",decimals=1)} |')
print('\n| Configuration | Hash + fold ms | Pwrite ms | fdatasync ms | Pack ms | Timed execution + Commit Mmsgs/s |')
print('| --- | ---: | ---: | ---: | ---: | ---: |')
for name, runs in results:
    timed = pair(runs, 'timed_combined_msgs_s', 1e6) if runs[0]['overlap']=='false' else 'use wall rate'
    print(f'| {name} | {pair(runs,"hash_ms")} | {pair(runs,"write_ms")} | {pair(runs,"fdatasync_ms")} | {pair(runs,"pack_ms")} | {timed} |')

print('\n| Configuration | Write bytes/block | Write calls/block | Buffered write MB/s |')
print('| --- | ---: | ---: | ---: |')
for name, runs in results:
    speed = ' / '.join(f'{float(r["write_bytes_block"])/(float(r["write_ms"])/1000)/1e6:.0f}' for r in runs)
    print(f'| {name} | {pair(runs,"write_bytes_block",decimals=0)} | {pair(runs,"pwrite_calls_block",decimals=3)} | {speed} |')
