// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Blake3Yul} from "./Blake3Yul.sol";
import {Ed25519Fast} from "gasbench/Ed25519Fast.sol";
import {SclEdKey} from "gasbench/SclEdKey.sol";

/// On-chain KERI light client. Rebuilds each key event byte-exact from structured fields
/// (keripy 1.2.12 field order, KERI10JSON), checks SAID, chain, signatures, witness receipts
/// and delegation, then keeps only the latest key state. On top: vLEI credential registration
/// (QVI -> LE -> ECR, checked against issuance anchors) and holder-signed EVM address links.
/// Unaudited prototype.
contract KeriLightClient {
    uint8 constant ICP = 0;
    uint8 constant ROT = 1;
    uint8 constant IXN = 2;
    uint8 constant DIP = 3;
    uint8 constant DRT = 4;
    bytes15 constant ILKS = "icprotixndipdrt";
    uint256 constant MAX = 32; // keys, next digests, witnesses
    bytes constant DUMMY = "############################################";

    struct Seal {
        bytes32 i;
        uint256 s;
        bytes32 d;
    }

    /// Raw 32-byte values; the CESR codes are implied: i, p, n, a.i, a.d, di 'E' (Blake3-256),
    /// k 'D' (Ed25519), b, br, ba 'B' (non-transferable Ed25519). kt and nt are the JSON
    /// fragments as signed, e.g. "2" (with quotes) or ["1/3","1/3","1/3"].
    struct Event {
        uint8 t;
        bytes32 i;
        uint256 s;
        bytes32 p;
        bytes kt;
        bytes32[] k;
        bytes nt;
        bytes32[] n;
        uint256 bt;
        bytes32[] b;
        bytes32[] br;
        bytes32[] ba;
        string[] c;
        Seal[] a;
        bytes32 di;
    }

    /// Indexed signature. ondex 255 = none (CESR current-only codes).
    struct Sig {
        uint8 index;
        uint8 ondex;
        bytes32 r;
        bytes32 s;
    }

    struct Kel {
        bool exists;
        bool estOnly;
        bool doNotDelegate;
        uint64 seq;
        uint64 bt;
        bytes32 lastDigest;
        bytes32 delegator;
        bytes32[] keys;
        bytes kt;
        bytes32[] ndigs;
        bytes nt;
        bytes32[] wits;
    }

    Ed25519Verifier public immutable ed = new Ed25519Verifier();
    mapping(bytes32 => Kel) public kels;
    mapping(bytes32 => uint256[4]) extKeys; // raw Ed25519 key -> SCL extended form (5th word is the key)
    mapping(bytes32 => bool) anchors; // keccak(prefix, seal i, s, d)
    mapping(bytes32 => bool) anchoredIS; // keccak(prefix, seal i, s), any d

    /// Byte offsets into the credential JSON of the 44-char values of i, s, a.i, the edge n
    /// (e.qvi.n for LE, e.le.n for ECR, else 0), and of the 20-char a.LEI (LE only, else 0).
    struct Offsets {
        uint256 i;
        uint256 s;
        uint256 ai;
        uint256 lei;
        uint256 edge;
    }

    /// The TEL iss event fields that are not in the credential: registry and issuance datetime.
    struct IssFields {
        bytes32 registry;
        string dt;
    }

    struct Credential {
        bytes32 issuer;
        bytes32 holder;
        bytes32 schema;
        bytes32 parent; // edge credential SAID (0 for QVI)
        bytes20 lei; // LE only
    }

    bytes32 public immutable trustedExternal; // issuer of QVI credentials (GLEIF External)
    bytes32 public immutable qviSchema;
    bytes32 public immutable leSchema;
    bytes32 public immutable ecrSchema;
    mapping(bytes32 => Credential) public credentials;
    mapping(address => bytes32) public credentialOf; // linked EVM address -> credential SAID

    event EventAccepted(bytes32 prefix, uint256 seq);
    event Linked(address account, bytes32 credSaid);

    constructor(bytes32 external_, bytes32 qvi, bytes32 le, bytes32 ecr) {
        trustedExternal = external_;
        qviSchema = qvi;
        leSchema = le;
        ecrSchema = ecr;
    }

    function postEvent(Event calldata e, Sig[] calldata sigs, Sig[] calldata wigs) external {
        (bytes memory raw, bytes32 said) = serialize(e);
        Kel storage kel = kels[e.i];
        bytes32[] memory wits = checkChain(e, kel, said);
        uint256 bt = e.bt;
        if (e.t == IXN) {
            bt = kel.bt;
            checkSigs(raw, sigs, kel.keys, kel.kt, new bytes32[](0), "");
        } else {
            require(e.k.length <= MAX && e.n.length <= MAX && wits.length <= MAX, "too many");
            require(wits.length == 0 ? bt == 0 : bt >= 1 && bt <= wits.length, "bt");
            parseThreshold(e.nt, e.n.length); // well-formed next threshold
            // Rotation: signed by the new keys, which must also satisfy the prior next commitment.
            bool rot = e.t == ROT || e.t == DRT;
            checkSigs(raw, sigs, e.k, e.kt, rot ? kel.ndigs : new bytes32[](0), rot ? kel.nt : bytes(""));
        }
        checkReceipts(raw, wigs, wits, bt);
        if (e.t == DIP || e.t == DRT) {
            bytes32 del = e.t == DIP ? e.di : kel.delegator;
            require(!kels[del].doNotDelegate, "delegator DND");
            require(anchors[keccak256(abi.encode(del, e.i, e.s, said))], "no delegation anchor");
        }
        store(e, kel, said, wits, bt);
        emit EventAccepted(e.i, e.s);
    }

    /// Prefix, sequence and prior digest rules; returns the witness list after this event.
    function checkChain(Event calldata e, Kel storage kel, bytes32 said) internal view returns (bytes32[] memory) {
        uint8 t = e.t;
        if (t == ICP || t == DIP) {
            require(said == e.i, "prefix != said"); // only self-addressing prefixes
            require(!kel.exists, "exists");
            require(e.s == 0, "sn");
            return deriveWits(new bytes32[](0), new bytes32[](0), e.b);
        }
        require(kel.exists, "unknown prefix");
        require(e.s == kel.seq + 1, "sn");
        require(e.p == kel.lastDigest, "prior digest");
        if (t == IXN) {
            require(!kel.estOnly, "establishment only");
            return kel.wits;
        }
        require((t == DRT) == (kel.delegator != 0), "rot/drt");
        require(kel.ndigs.length > 0, "non-transferable");
        return deriveWits(kel.wits, e.br, e.ba);
    }

    function store(Event calldata e, Kel storage kel, bytes32 said, bytes32[] memory wits, uint256 bt) internal {
        if (e.t == ICP || e.t == DIP) {
            kel.exists = true;
            if (e.t == DIP) kel.delegator = e.di; // icp ignores di (not serialized)
            for (uint256 j; j < e.c.length; j++) {
                bytes32 h = keccak256(bytes(e.c[j]));
                if (h == keccak256("EO")) kel.estOnly = true;
                if (h == keccak256("DND")) kel.doNotDelegate = true;
            }
        }
        kel.seq = uint64(e.s);
        kel.lastDigest = said;
        if (e.t != IXN) {
            kel.keys = e.k;
            kel.kt = e.kt;
            kel.ndigs = e.n;
            kel.nt = e.nt;
            kel.wits = wits;
            kel.bt = uint64(bt);
        }
        for (uint256 j; j < e.a.length; j++) {
            Seal calldata a = e.a[j];
            anchors[keccak256(abi.encode(e.i, a.i, a.s, a.d))] = true;
            anchoredIS[keccak256(abi.encode(e.i, a.i, a.s))] = true;
        }
    }

    function anchorExists(bytes32 prefix, bytes32 i, uint256 s, bytes32 d) external view returns (bool) {
        return anchors[keccak256(abi.encode(prefix, i, s, d))];
    }

    /// Any seal {i, s} in the KEL of `prefix`. A credential is revoked when its issuer anchored {credSaid, 1}.
    function isAnchored(bytes32 prefix, bytes32 i, uint256 s) public view returns (bool) {
        return anchoredIS[keccak256(abi.encode(prefix, i, s))];
    }

    // ---------------- vLEI credentials ----------------

    /// Registers a QVI, LE or ECR credential. `acdc` is the compact JSON exactly as issued. Every value
    /// is read at a pinned position of the keripy layout {v, d, u?, i, ri, s, a{d, u?, i, ..}, e{d, edge}},
    /// so a caller cannot point i, s or a.i at a nested field with the same key.
    function registerCredential(bytes calldata acdc, Offsets calldata off, IssFields calldata iss) external {
        bytes32 said = acdcSaid(acdc);
        require(credentials[said].holder == 0, "registered");
        bytes32 issuer = field(acdc, off.i, '","i":"');
        require(off.i == 81 || salt(acdc, 74, off.i - 7), "i offset"); // after d (ends at 74), optional u
        require(off.s == off.i + 103 && keyAt(acdc, off.i + 52, '","ri":"'), "s offset");
        bytes32 schema = field(acdc, off.s, '","s":"');
        require(keyAt(acdc, off.s + 56, '","a":{"d":"'), "a offset");
        require(off.ai == off.s + 107 || salt(acdc, off.s + 100, off.ai - 7), "a.i offset");
        Credential memory c = Credential(issuer, field(acdc, off.ai, '","i":"'), schema, 0, 0);

        bytes memory body = abi.encodePacked(
            '"t":"iss","d":"', DUMMY, '","i":"', qb64("E", said), '","s":"0","ri":"', qb64("E", iss.registry),
            '","dt":"', iss.dt, '"'
        );
        bytes memory raw = abi.encodePacked('{"v":"KERI10JSON', hex6(26 + body.length), '_",', body, "}");
        require(anchors[keccak256(abi.encode(issuer, said, uint256(0), Blake3Yul.hash(raw)))], "no iss anchor");

        if (schema == qviSchema) {
            require(issuer == trustedExternal, "issuer");
        } else {
            bool le = schema == leSchema;
            require(le || schema == ecrSchema, "schema");
            bytes memory key = le ? bytes('","qvi":{"n":"') : bytes('","le":{"n":"');
            c.parent = field(acdc, off.edge, key);
            uint256 e = off.edge - key.length - 44; // start of the e.d value
            require(off.edge > off.ai && keyAt(acdc, e, '},"e":{"d":"'), "edge offset");
            Credential storage p = credentials[c.parent];
            require(p.schema == (le ? qviSchema : leSchema) && p.holder == issuer, "parent");
            if (le) {
                require(off.lei > off.ai && off.lei < e && keyAt(acdc, off.lei, '","LEI":"'), "LEI offset");
                require(acdc[off.lei + 20] == '"', "LEI");
                c.lei = bytes20(acdc[off.lei:off.lei + 20]);
            }
        }
        credentials[said] = c;
    }

    /// Registered, and no credential on the way up to the QVI credential is revoked.
    function isValid(bytes32 said) public view returns (bool) {
        for (uint256 j; j < 3; j++) {
            Credential storage c = credentials[said];
            if (c.holder == 0 || isAnchored(c.issuer, said, 1)) return false;
            if (c.schema == qviSchema) return true;
            said = c.parent;
        }
        return false;
    }

    /// LEI of an LE credential, or of the LE parent of an ECR credential; "" otherwise.
    function leiOf(bytes32 said) public view returns (string memory) {
        Credential storage c = credentials[said];
        if (c.schema == ecrSchema) c = credentials[c.parent];
        return c.lei == 0 ? "" : string(abi.encodePacked(c.lei));
    }

    /// The credential holder's current keys sign the lowercase 0x address text (as `kli sign --text`).
    function linkAddress(bytes32 said, address account, Sig[] calldata sigs) external {
        Kel storage kel = kels[credentials[said].holder];
        require(kel.exists, "holder KEL"); // also an unregistered credential (holder 0)
        checkSigs(abi.encodePacked("0x", hexN(uint160(account), 40)), sigs, kel.keys, kel.kt, new bytes32[](0), "");
        credentialOf[account] = said;
        emit Linked(account, said);
    }

    function leiOfAddress(address account) external view returns (string memory lei, bool valid) {
        bytes32 said = credentialOf[account];
        return (leiOf(said), isValid(said));
    }

    /// SAID of a compact ACDC: Blake3 with the top-level d value as 44 '#', must equal d.
    function acdcSaid(bytes calldata acdc) internal pure returns (bytes32 said) {
        require(keyAt(acdc, 16, '{"v":"ACDC10JSON') && keyAt(acdc, 30, '_","d":"'), "acdc");
        require(keccak256(acdc[16:22]) == keccak256(hex6(acdc.length)), "acdc size");
        said = unq(acdc[30:74]);
        bytes memory m = acdc;
        bytes memory dummy = DUMMY;
        assembly ("memory-safe") {
            mcopy(add(m, 62), add(dummy, 32), 44)
        }
        require(Blake3Yul.hash(m) == said, "said");
    }

    /// The 44-char qb64 value at `off`, which must follow `key` and end with a quote.
    function field(bytes calldata b, uint256 off, bytes memory key) internal pure returns (bytes32) {
        require(keyAt(b, off, key) && b[off + 44] == '"', "key");
        return unq(b[off:off + 44]);
    }

    /// `b` holds `key` right before `end`.
    function keyAt(bytes calldata b, uint256 end, bytes memory key) internal pure returns (bool) {
        return end >= key.length && end <= b.length && keccak256(b[end - key.length:end]) == keccak256(key);
    }

    /// b[from:to] is exactly one string field ,"u":"<no quote>" (the salt of a private ACDC).
    function salt(bytes calldata b, uint256 from, uint256 to) internal pure returns (bool) {
        if (to < from + 7 || !keyAt(b, from + 7, '","u":"')) return false;
        for (uint256 j = from + 7; j < to; j++) if (b[j] == '"') return false;
        return true;
    }

    /// Raw 32 bytes of a qb64 'E' (Blake3-256) value; canonical encoding only.
    function unq(bytes calldata q) internal pure returns (bytes32 r) {
        uint256 v;
        for (uint256 j = 1; j < 44; j++) {
            uint256 c = uint8(q[j]);
            v = (v << 6) | (c == 95 ? 63 : c >= 97 ? c - 71 : c >= 65 ? c - 65 : c >= 48 ? c + 4 : 62);
        }
        r = bytes32(v);
        require(keccak256(qb64("E", r)) == keccak256(q), "qb64");
    }

    // ---------------- signatures ----------------

    /// keys/kt: the signing key list and threshold. priorNext/priorNt: set only for rot/drt.
    function checkSigs(
        bytes memory raw,
        Sig[] calldata sigs,
        bytes32[] memory keys,
        bytes memory kt,
        bytes32[] memory priorNext,
        bytes memory priorNt
    ) internal {
        uint256 seen;
        uint256 exposed;
        for (uint256 j; j < sigs.length; j++) {
            Sig calldata g = sigs[j];
            // Repeated index allowed (GLEIF Root rot 3 signs index 1 twice), counted once as in keripy.
            require(g.index < keys.length, "sig index");
            seen |= 1 << g.index;
            require(verify(raw, g, keys[g.index]), "bad sig");
            // Rotation: a new key exposes the prior next digest at ondex (keripy Kever.exposeds).
            if (g.ondex < priorNext.length && Blake3Yul.hash(qb64("D", keys[g.index])) == priorNext[g.ondex]) {
                exposed |= 1 << g.ondex;
            }
        }
        require(satisfies(kt, keys.length, seen), "kt not met");
        // Rotation must also satisfy the prior next threshold over the exposed digests.
        if (priorNext.length != 0) require(satisfies(priorNt, priorNext.length, exposed), "prior nt not met");
    }

    function checkReceipts(bytes memory raw, Sig[] calldata wigs, bytes32[] memory wits, uint256 bt) internal {
        uint256 seen;
        for (uint256 j; j < wigs.length; j++) {
            Sig calldata g = wigs[j];
            require(g.index < wits.length && (seen >> g.index) & 1 == 0, "receipt index");
            seen |= 1 << g.index;
            require(verify(raw, g, wits[g.index]), "bad receipt");
        }
        require(wigs.length >= bt, "receipts below bt");
    }

    function verify(bytes memory m, Sig calldata g, bytes32 pk) internal returns (bool) {
        uint256[4] storage x = extKeys[pk];
        if (x[0] == 0) extKeys[pk] = ed.expandKey(pk); // first use of this key: expand once and cache
        return ed.verify(m, g.r, g.s, [x[0], x[1], x[2], x[3], uint256(pk)]);
    }

    // ---------------- thresholds ----------------

    /// keripy Tholder.satisfy. Numeric "M": at least M (M > 0) signers. Weighted: sum of
    /// signer weights >= 1; an index past the weight list fails (keripy IndexError).
    function satisfies(bytes memory thold, uint256 n, uint256 bits) internal pure returns (bool) {
        (uint256[] memory w, uint256 req) = parseThreshold(thold, n);
        uint256 sum;
        for (uint256 i; bits >> i != 0; i++) {
            if ((bits >> i) & 1 == 0) continue;
            if (i >= w.length) return false;
            sum += w[i];
        }
        return req > 0 && sum >= req;
    }

    /// Returns integer weights over a common denominator and the required sum.
    /// shortcut: single weighted clause only (no [[..],[..]] or {"w":[..]} forms); add when a KEL uses them.
    function parseThreshold(bytes memory t, uint256 n) internal pure returns (uint256[] memory w, uint256 req) {
        require(t.length >= 3, "threshold");
        uint256 j;
        if (t[0] == '"') {
            (req, j) = parseNum(t, 1, 16);
            require(j == t.length - 1 && t[j] == '"', "threshold");
            require(n >= req, "threshold size");
            w = new uint256[](n);
            for (uint256 i; i < n; i++) w[i] = 1;
            return (w, req);
        }
        require(t[0] == "[", "threshold");
        uint256[] memory num = new uint256[](MAX);
        uint256[] memory den = new uint256[](MAX);
        uint256 cnt;
        req = 1; // lcm of denominators
        j = 1;
        while (true) {
            require(cnt < MAX && t[j] == '"', "threshold");
            (num[cnt], j) = parseNum(t, j + 1, 10);
            den[cnt] = 1;
            if (t[j] == "/") (den[cnt], j) = parseNum(t, j + 1, 10);
            require(den[cnt] > 0 && num[cnt] <= den[cnt] && t[j] == '"', "weight"); // 0 <= w <= 1
            req = (req * den[cnt]) / gcd(req, den[cnt]);
            require(req < 1 << 64, "weight");
            cnt++;
            j++;
            if (t[j] == "]") break;
            require(t[j] == ",", "threshold");
            j++;
        }
        require(j == t.length - 1 && n >= cnt, "threshold size");
        w = new uint256[](cnt);
        uint256 sum;
        for (uint256 i; i < cnt; i++) {
            w[i] = (num[i] * req) / den[i];
            sum += w[i];
        }
        require(sum >= req, "weights sum < 1");
    }

    /// Digits (base 10, or lowercase hex for 16) from j; returns value and the first non-digit index.
    function parseNum(bytes memory t, uint256 j, uint256 base) internal pure returns (uint256 v, uint256 k) {
        for (k = j; k < t.length; k++) {
            uint8 c = uint8(t[k]);
            uint256 d;
            if (c >= 48 && c <= 57) d = c - 48;
            else if (base == 16 && c >= 97 && c <= 102) d = c - 87;
            else break;
            v = v * base + d;
            require(v < 1 << 32, "number");
        }
        require(k > j && k < t.length, "number");
    }

    function gcd(uint256 a, uint256 b) internal pure returns (uint256) {
        while (b != 0) (a, b) = (b, a % b);
        return a;
    }

    /// keripy Kever.deriveBacks: remove cuts (all must be present), append adds (new, unique).
    function deriveWits(bytes32[] memory old, bytes32[] memory cuts, bytes32[] memory adds)
        internal
        pure
        returns (bytes32[] memory out)
    {
        out = new bytes32[](old.length + adds.length);
        uint256 m;
        for (uint256 i; i < old.length; i++) {
            if (!contains(cuts, cuts.length, old[i])) out[m++] = old[i];
        }
        require(old.length - m == cuts.length, "bad cuts"); // also rejects duplicate cuts
        for (uint256 i; i < adds.length; i++) {
            require(!contains(out, m, adds[i]) && !contains(cuts, cuts.length, adds[i]), "bad adds");
            out[m++] = adds[i];
        }
        assembly ("memory-safe") {
            mstore(out, m)
        }
    }

    function contains(bytes32[] memory xs, uint256 len, bytes32 x) internal pure returns (bool) {
        for (uint256 i; i < len; i++) if (xs[i] == x) return true;
        return false;
    }

    // ---------------- serialization ----------------

    /// The exact KERI10JSON bytes keripy 1.2.12 signs, and their SAID (Blake3-256 over the
    /// bytes with d, and i for inception, set to 44 '#').
    function serialize(Event calldata e) public pure returns (bytes memory raw, bytes32 said) {
        uint8 t = e.t;
        require(t <= DRT, "ilk");
        bool incept = t == ICP || t == DIP;
        bytes memory body = abi.encodePacked(
            '"t":"', bytes3(ILKS << (24 * t)), '","d":"', DUMMY, '","i":"', incept ? DUMMY : qb64("E", e.i),
            '","s":"', hexStr(e.s), '"'
        );
        if (!incept) body = abi.encodePacked(body, ',"p":"', qb64("E", e.p), '"');
        if (t != IXN) {
            body = abi.encodePacked(
                body, ',"kt":', e.kt, ',"k":', qlist("D", e.k), ',"nt":', e.nt, ',"n":', qlist("E", e.n),
                ',"bt":"', hexStr(e.bt), '"'
            );
            body = incept
                ? abi.encodePacked(body, ',"b":', qlist("B", e.b), ',"c":', strList(e.c))
                : abi.encodePacked(body, ',"br":', qlist("B", e.br), ',"ba":', qlist("B", e.ba));
        }
        body = abi.encodePacked(body, ',"a":', seals(e.a));
        if (t == DIP) body = abi.encodePacked(body, ',"di":"', qb64("E", e.di), '"');
        raw = abi.encodePacked('{"v":"KERI10JSON', hex6(26 + body.length), '_",', body, "}");
        said = Blake3Yul.hash(raw);
        bytes memory q = qb64("E", said);
        assembly ("memory-safe") {
            // d starts at byte 40, i (inception) at 91: {"v":"KERI10JSONxxxxxx_","t":"icp","d":"...","i":"
            mcopy(add(raw, 72), add(q, 32), 44)
            if incept { mcopy(add(raw, 123), add(q, 32), 44) }
        }
    }

    /// CESR qb64 of a 32-byte raw value with a one-char code: base64url of 0x00||raw, first char = code.
    function qb64(bytes1 code, bytes32 raw) internal pure returns (bytes memory out) {
        out = new bytes(44);
        assembly ("memory-safe") {
            let p := add(out, 32)
            mstore8(p, shr(248, code))
            let v := raw
            for { let i := 43 } gt(i, 0) { i := sub(i, 1) } {
                let x := and(v, 63)
                let ch := add(x, 65) // A-Z
                if gt(x, 25) { ch := add(x, 71) } // a-z
                if gt(x, 51) { ch := sub(x, 4) } // 0-9
                if eq(x, 62) { ch := 45 } // -
                if eq(x, 63) { ch := 95 } // _
                mstore8(add(p, i), ch)
                v := shr(6, v)
            }
        }
    }

    function qlist(bytes1 code, bytes32[] calldata xs) internal pure returns (bytes memory out) {
        out = "[";
        for (uint256 i; i < xs.length; i++) {
            out = abi.encodePacked(out, i == 0 ? "" : ",", '"', qb64(code, xs[i]), '"');
        }
        out = abi.encodePacked(out, "]");
    }

    function seals(Seal[] calldata a) internal pure returns (bytes memory out) {
        out = "[";
        for (uint256 i; i < a.length; i++) {
            out = abi.encodePacked(
                out, i == 0 ? "" : ",", '{"i":"', qb64("E", a[i].i), '","s":"', hexStr(a[i].s), '","d":"',
                qb64("E", a[i].d), '"}'
            );
        }
        out = abi.encodePacked(out, "]");
    }

    /// Config traits, e.g. ["EO"]. Alphanumeric only so no JSON escaping can differ.
    function strList(string[] calldata c) internal pure returns (bytes memory out) {
        out = "[";
        for (uint256 i; i < c.length; i++) {
            bytes calldata s = bytes(c[i]);
            for (uint256 k; k < s.length; k++) {
                bytes1 ch = s[k];
                require((ch >= "0" && ch <= "9") || (ch >= "A" && ch <= "Z") || (ch >= "a" && ch <= "z"), "trait");
            }
            out = abi.encodePacked(out, i == 0 ? "" : ",", '"', s, '"');
        }
        out = abi.encodePacked(out, "]");
    }

    /// Lowercase hex, no leading zeros ("0" for zero), as KERI writes s and bt.
    function hexStr(uint256 v) internal pure returns (bytes memory) {
        uint256 len = 1;
        for (uint256 x = v >> 4; x != 0; x >>= 4) len++;
        return hexN(v, len);
    }

    function hex6(uint256 v) internal pure returns (bytes memory) {
        require(v < 1 << 24, "too big");
        return hexN(v, 6);
    }

    function hexN(uint256 v, uint256 len) internal pure returns (bytes memory out) {
        out = new bytes(len);
        for (uint256 i = len; i > 0; i--) {
            uint256 d = v & 15;
            out[i - 1] = bytes1(uint8(d < 10 ? 48 + d : 87 + d));
            v >>= 4;
        }
    }
}

/// Ed25519 from gasbench (SCL curve code + Yul SHA-512). Separate contract because SCL moves the
/// free memory pointer to 0x880 (needs its own call frame) and to keep KeriLightClient under EIP-170.
contract Ed25519Verifier {
    function expandKey(bytes32 pk) external returns (uint256[4] memory y) {
        uint256[5] memory ext = SclEdKey.extKey(pk);
        y = [ext[0], ext[1], ext[2], ext[3]];
    }

    function verify(bytes calldata m, bytes32 r, bytes32 s, uint256[5] calldata ext) external returns (bool) {
        return Ed25519Fast.verifyLE(m, uint256(r), uint256(s), ext);
    }
}
