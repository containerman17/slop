"""Extract the two checked host runs and the six EVM rows from raw output."""
from pathlib import Path
import json,re
base=Path(__file__).parent
out={}
for name in ['original','synthetic']:
    text=(base/(name+'.txt')).read_text()
    assert '\nPASS\n' in text and '--- FAIL' not in text
    rows=[]
    for line in text.splitlines():
        if 'V3_METRICS ' in line:
            fields=dict(part.split('=',1) for part in line.split('V3_METRICS ',1)[1].split())
            rows.append({k:v if k=='final_root' else float(v) for k,v in fields.items()})
    assert len(rows)==2 and rows[0]['final_root']==rows[1]['final_root']
    for r in rows:
        assert r['blocks']==600
        assert abs(r['wall_msgs_s']-30_000_000/r['wall_s'])<1
    out[name]=rows
text=(base/'evm.txt').read_text()
assert '\nPASS\n' in text and '--- FAIL' not in text
rows=[]
for line in text.splitlines():
    if re.match(r'BenchmarkEVM/\S+\s+700\s',line):
        parts=line.split(); row={'name':parts[0], 'blocks':int(parts[1])}
        for i in range(2,len(parts),2): row[parts[i+1]]=float(parts[i])
        rows.append(row)
assert len(rows)==6
for name in ['uniform','hot_sigma_20000','hot_sigma_2000']:
    assert f'EVM_ROOTS_CHECKED distribution={name} blocks=700' in text
out['evm']=rows
print(json.dumps(out,indent=2))
