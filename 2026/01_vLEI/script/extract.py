"""Turns the GLEIF CESR streams into postEvent() fixtures, with keripy 1.2.12 as the parser.

Usage (venv with keri==1.2.12 and eth-abi): python -I script/extract.py [dir]
Writes test/fixtures/{root,ext,prov}.json (keri-probe snapshot). Each event carries the original bytes (raw),
its SAID, and abi.encode(Event, Sig[] sigs, Sig[] wigs) for KeriLightClient.postEvent.
Also writes the fake chain (testdata/fakechain) fixtures: fake_kels.json (all 6 KELs in posting
order), fake_creds.json (registerCredential args) and fake_links.json (linkAddress args).
Real chain: real_kels.json (Root, External, Provenant, GLEIF Internal from <dir>/{root,ext,prov,internal}.cesr
as written by script/fetch_real.sh, default testdata/realcred/fetched) and real_creds.json (the Provenant QVI
and GLEIF LE credentials from testdata/realcred/extracted).
"""
import json, os, re, sys
from eth_abi import encode
from keri.core import parsing
from keri.core.coring import Matter, Verfer
from keri.core.indexing import Siger

HERE = os.path.dirname(os.path.abspath(__file__))
PROBE = os.path.join(HERE, '..', '..', 'keri-probe')
OUT = os.path.join(HERE, '..', 'test', 'fixtures')
FAKE = os.path.join(HERE, '..', '..', 'testdata', 'fakechain')
REAL = os.path.join(HERE, '..', '..', 'testdata', 'realcred')

ILK = {'icp': 0, 'rot': 1, 'ixn': 2, 'dip': 3, 'drt': 4}
NO_ONDEX = 255
EVENT = '(uint8,bytes32,uint256,bytes32,bytes,bytes32[],bytes,bytes32[],uint256,bytes32[],bytes32[],bytes32[],string[],(bytes32,uint256,bytes32)[],bytes32)'
SIGS = '(uint8,uint8,bytes32,bytes32)[]'
Z = b'\x00' * 32


def raw(qb64, code):
    m = Matter(qb64=qb64)
    assert m.code == code, (qb64, code)
    return m.raw


class Capture:
    """Duck-typed Kevery: records what keripy's parser attaches to each event."""
    def __init__(self):
        self.events = []

    def processEvent(self, serder, sigers, wigers=None, **kw):
        self.events.append((serder, sigers, wigers or []))

    def __getattr__(self, name):  # rpy and other messages: ignored
        return lambda *a, **k: None


def sig(s):
    return (s.index, NO_ONDEX if s.ondex is None else s.ondex, s.raw[:32], s.raw[32:])


def event(serder, sigers, wigers):
    k = serder.ked
    t = k['t']
    est = t in ('icp', 'rot', 'dip', 'drt')
    frag = lambda v: json.dumps(v, separators=(',', ':')).encode()
    seals = []
    for a in k.get('a', []):
        assert sorted(a) == ['d', 'i', 's'], a  # only event seals occur in this data
        seals.append((raw(a['i'], 'E'), int(a['s'], 16), raw(a['d'], 'E')))
    ev = (
        ILK[t], raw(k['i'], 'E'), int(k['s'], 16),
        raw(k['p'], 'E') if 'p' in k else Z,
        frag(k['kt']) if est else b'', [raw(x, 'D') for x in k.get('k', [])],
        frag(k['nt']) if est else b'', [raw(x, 'E') for x in k.get('n', [])],
        int(k['bt'], 16) if est else 0,
        [raw(x, 'B') for x in k.get('b', [])],
        [raw(x, 'B') for x in k.get('br', [])],
        [raw(x, 'B') for x in k.get('ba', [])],
        k.get('c', []), seals,
        raw(k['di'], 'E') if 'di' in k else Z,
    )
    args = encode([EVENT, SIGS, SIGS], [ev, [sig(s) for s in sigers], [sig(s) for s in wigers]])
    return dict(t=t, s=int(k['s'], 16), said='0x' + raw(k['d'], 'E').hex(),
                raw='0x' + bytes(serder.raw).hex(), args='0x' + args.hex())


def events(path, prefix):
    cap = Capture()
    parsing.Parser(kvy=cap, framed=True).parse(ims=bytearray(open(path, 'rb').read()))
    return [event(*e) for e in cap.events if e[0].pre == prefix]


def write(out, **obj):
    with open(os.path.join(OUT, out), 'w') as f:
        json.dump(obj, f, indent=1)


def extract(cesr, prefix, out):
    evs = events(os.path.join(PROBE, cesr), prefix)
    write(out, n=len(evs), events=evs)
    print(out, len(evs), 'events, last sn', evs[-1]['s'])


extract('root_gleif.cesr', 'EDP1vHcw_wc4M__Fj53-cJaBnZZASd-aMTaSyWEQ-PC2', 'root.json')
extract('ext_witness.cesr', 'EINmHd5g7iV-UldkkkKyBIH052bIyxZNBn9pq-zNrYoS', 'ext.json')
extract('prov.cesr', 'ED88Jn6CnWpNbSYz6vp9DOSpJH2_Di5MSwWTf1l34JJm', 'prov.json')


# ---------------- fake chain (testdata/fakechain) ----------------

IDS = dict(l.split() for l in open(os.path.join(FAKE, 'ids.txt')))
# Delegators before delegates; External ixn 4 (anchors the QVI drt) lands before QVI, which is fine.
KELS = ['root', 'external', 'qvi', 'avalabs', 'ilia', 'revoked']
OFFSETS = '(uint256,uint256,uint256,uint256,uint256)'
ISS = '(bytes32,string)'
# HOME=<scratch>/fakechain/home kli sign --name <n> --alias <n> --text <address>, 2026-10-09.
# Addresses: cast wallet address --private-key $(cast keccak "vlei-fuji-throwaway-<n>")
LINKS = [
    ('ilia', 'cred_ecr_ilia', '0x46911307c190503d009230833b108eef18cdfc57',
     'AAB97D11iyXfU-0XcC1Hdplw1_B9vJhec1hvQmZ9v7lATESjD1wZRVsPfTvn0O4JZzJnvF6Hf3qIJLvJWHs60cwJ'),
    ('revoked', 'cred_ecr_revoked', '0x36a5e787690cc14fa6a4ca438da238bb98cd5a1f',
     'AACjTSk-YW1PyCdaxMbHWGvwUv0WyhxfQPle3hfhc6GT0Bo-nImWqrP5yyA6jA2BB5rDHMkqirmhZSdAX61splMP'),
]


def tel_events():
    """Every TEL message of the fake registries, as dicts (they are plain JSON in the .cesr)."""
    out = []
    for f in sorted(os.listdir(FAKE)):
        if f.startswith('tel_'):
            txt = open(os.path.join(FAKE, f)).read()
            for m in re.finditer(r'\{"v":"KERI10JSON', txt):
                out.append(json.JSONDecoder().raw_decode(txt, m.start())[0])
    return out


def after(b, key, start=0):
    return b.index(key, start) + len(key)


def cred_args(name, b, iss):
    """registerCredential args for the exact compact ACDC bytes b and its TEL iss event."""
    k = json.loads(b)
    e = k.get('e', {})
    i = after(b, b'","i":"')  # first i is the top-level issuer
    s = after(b, b'","s":"')  # first s is the top-level schema
    ai = after(b, b'","i":"', after(b, b'"a":{'))
    lei = after(b, b'","LEI":"') if 'qvi' in e else 0  # LE only
    edge = after(b, b'","qvi":{"n":"') if 'qvi' in e else after(b, b'","le":{"n":"') if 'le' in e else 0
    assert b[i:i + 44].decode() == k['i'] and b[s:s + 44].decode() == k['s'] and b[ai:ai + 44].decode() == k['a']['i']
    assert iss['t'] == 'iss' and iss['i'] == k['d'] and iss['ri'] == k['ri']
    args = encode(['bytes', OFFSETS, ISS], [b, (i, s, ai, lei, edge), (raw(iss['ri'], 'E'), iss['dt'])])
    return dict(name=name, said='0x' + raw(k['d'], 'E').hex(), schema='0x' + raw(k['s'], 'E').hex(),
                issSaid='0x' + raw(iss['d'], 'E').hex(), off=dict(i=i, s=s, ai=ai, lei=lei, edge=edge),
                args='0x' + args.hex())


def fake():
    evs = []
    for name in KELS:
        e = events(os.path.join(FAKE, f'kel_{name}.cesr'), IDS[name])
        for x in e:
            x['name'] = name
        evs += e
    write('fake_kels.json', n=len(evs), events=evs)
    print('fake_kels.json', len(evs), 'events')

    tels = tel_events()
    creds = []
    for name in ['cred_qvi', 'cred_le', 'cred_ecr_ilia', 'cred_ecr_revoked']:
        b = open(os.path.join(FAKE, f'{name}.json'), 'rb').read()
        assert json.loads(b)['d'] == IDS[name]
        iss = [t for t in tels if t['t'] == 'iss' and t['i'] == IDS[name]]
        assert len(iss) == 1
        creds.append(cred_args(name, b, iss[0]))
    write('fake_creds.json', n=len(creds), creds=creds)
    print('fake_creds.json', [(c['name'], c['off']) for c in creds])

    links = []
    for holder, cred, account, sigq in LINKS:
        # kli sign signs the raw UTF-8 text bytes, no framing: check with the holder's current key.
        k = json.loads(open(os.path.join(FAKE, f'kelexport_{holder}.cesr')).read().split('}-')[0] + '}')
        siger = Siger(qb64=sigq)
        assert Verfer(qb64=k['k'][siger.index]).verify(siger.raw, account.encode()), holder
        args = encode([SIGS], [[sig(siger)]])
        links.append(dict(name=holder, account=account, said='0x' + raw(IDS[cred], 'E').hex(), args='0x' + args.hex()))
    write('fake_links.json', n=len(links), links=links)
    print('fake_links.json ok, kli signatures verify over the raw address text')


fake()


# ---------------- real chain (fetched from the production witnesses) ----------------

# Posting order: delegators first (Root delegates External and Internal, External delegates Provenant).
REAL_KELS = [('root', 'EDP1vHcw_wc4M__Fj53-cJaBnZZASd-aMTaSyWEQ-PC2'), ('ext', 'EINmHd5g7iV-UldkkkKyBIH052bIyxZNBn9pq-zNrYoS'),
             ('prov', 'ED88Jn6CnWpNbSYz6vp9DOSpJH2_Di5MSwWTf1l34JJm'), ('internal', 'EFcrtYzHx11TElxDmEDx355zm7nJhbmdcIluw7UMbUIL')]
# Provenant QVI credential (issued by GLEIF External), GLEIF LE credential (issued by Provenant).
REAL_CREDS = [('real_qvi', 'EMatUqz_u9BizxwOc3JishC4MyXfiWzQadDpgCBA6X9n'), ('real_le', 'EB9yGbWXOn_MhTQsHltftAsNrlWAxgedMPIQl4rg6C1e')]


def real(folder):
    evs = []
    for name, pre in REAL_KELS:
        e = events(os.path.join(folder, f'{name}.cesr'), pre)
        assert [x['s'] for x in e] == list(range(len(e))), name  # complete history, no gaps
        for x in e:
            x['name'] = name
        evs += e
        print(name, len(e), 'events, last sn', hex(e[-1]['s']))
    write('real_kels.json', n=len(evs), events=evs)
    ext = os.path.join(REAL, 'extracted')
    creds = []
    for name, said in REAL_CREDS:
        b = open(os.path.join(ext, f'{said}.acdc.json'), 'rb').read()
        creds.append(cred_args(name, b, json.load(open(os.path.join(ext, f'{said}.iss.json')))))
    write('real_creds.json', n=len(creds), creds=creds)
    print('real_creds.json', [(c['name'], c['off']) for c in creds])


real(sys.argv[1] if len(sys.argv) > 1 else os.path.join(REAL, 'fetched'))
