// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import {KeriLightClient} from "../src/KeriLightClient.sol";
import {Blake3Yul} from "../src/Blake3Yul.sol";

// Fixtures come from script/extract.py (keripy 1.2.12 parses keri-probe/*.cesr and testdata/fakechain).
contract KeriLightClientTest is Test {
    bytes32 constant ROOT = 0x33f5bc7730ff073833ffc58f9dfe7096819d964049df9a313692c96110f8f0b6;
    bytes32 constant EXT = 0x83661dde60ee257e5257649242b20481f4e766c8cb164d067f69abeccdad8a12;
    bytes32 constant PROV = 0x3f3c267e829d6a4d6d2633eafa7d0ce4a9247dbf0e2e4c4b05937f5977e09266;
    bytes32 constant INTERNAL = 0x572bb58cc7c75d53125c439840f1df9e739bb9c985b99d70896ec3b50c6d420b; // GLEIF's own LE AID
    string[5] ILK = ["icp", "rot", "ixn", "dip", "drt"];
    // Fake chain (testdata/fakechain): External, the three GLEIF schema SAIDs, holders.
    bytes32 constant FAKE_EXT = 0x9a38c16a50eb0a514dbc71715db18ba12e9f49eabac7ddbfa6fea22586dca415;
    bytes32 constant QVI_SCHEMA = 0x17dd96ef11dbb15bc7e7a1aeac089a72bef029bbf6ca9b010e0c9af10062a6a8;
    bytes32 constant LE_SCHEMA = 0xd3d7a75bd0cd117a27022e4be9a9d94f147f5328003a0ffc6a90eb656c0d9598;
    bytes32 constant ECR_SCHEMA = 0x4cbd3e48a415c00d57597b107ba409e5eaaf34f584d6631a5524ef0b8c3b98f0;
    bytes32 constant AVALABS = 0x591593fa8d9bdc53cfb01e591a46394c202a1851e999047837073c82246e9637;
    bytes32 constant ILIA = 0xc08a2acaaf186d1bfc2cf150b1cbee534c7201b7ba73831013ef83a165be5505;
    uint256 constant QVI_CRED = 0;
    uint256 constant LE_CRED = 1;
    uint256 constant ECR_ILIA = 2;
    uint256 constant ECR_REVOKED = 3;

    KeriLightClient lc;
    uint256[5] cnt;
    uint256[5] sum;
    uint256[5] lo;
    uint256[5] hi;

    function setUp() public {
        lc = new KeriLightClient(FAKE_EXT, QVI_SCHEMA, LE_SCHEMA, ECR_SCHEMA);
    }

    function fixture(string memory name) internal view returns (string memory) {
        return vm.readFile(string.concat(vm.projectRoot(), "/test/fixtures/", name, ".json"));
    }

    function path(uint256 i, string memory key) internal pure returns (string memory) {
        return string.concat(".events[", vm.toString(i), "].", key);
    }

    function load(string memory json, uint256 i)
        internal
        pure
        returns (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs)
    {
        return abi.decode(
            vm.parseJsonBytes(json, path(i, "args")),
            (KeriLightClient.Event, KeriLightClient.Sig[], KeriLightClient.Sig[])
        );
    }

    /// Posts every event of a fixture; asserts the rebuilt bytes and SAID equal the originals.
    function feed(string memory name) internal returns (bytes32 lastSaid) {
        string memory json = fixture(name);
        uint256 n = vm.parseJsonUint(json, ".n");
        for (uint256 i; i < n; i++) {
            (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
                load(json, i);
            (bytes memory raw, bytes32 said) = lc.serialize(e);
            assertEq(raw, vm.parseJsonBytes(json, path(i, "raw")), "rebuilt bytes != original");
            lastSaid = vm.parseJsonBytes32(json, path(i, "said"));
            assertEq(said, lastSaid, "said");
            uint256 g = gasleft();
            lc.postEvent(e, sigs, wigs);
            g -= gasleft();
            console.log(
                string.concat(
                    name, " ", ILK[e.t], " sn ", vm.toString(e.s), " bytes ", vm.toString(raw.length), " sigs ",
                    vm.toString(sigs.length), " receipts ", vm.toString(wigs.length), " gas ", vm.toString(g)
                )
            );
            cnt[e.t]++;
            sum[e.t] += g;
            if (lo[e.t] == 0 || g < lo[e.t]) lo[e.t] = g;
            if (g > hi[e.t]) hi[e.t] = g;
        }
    }

    function report() internal view {
        for (uint256 t; t < 5; t++) {
            if (cnt[t] == 0) continue;
            console.log(
                string.concat(
                    "GAS ", ILK[t], " n=", vm.toString(cnt[t]), " min=", vm.toString(lo[t]), " avg=",
                    vm.toString(sum[t] / cnt[t]), " max=", vm.toString(hi[t])
                )
            );
        }
    }

    function assertHead(bytes32 pre, uint256 seq, bytes32 said) internal view {
        (bool exists,,, uint64 s,, bytes32 last,,,) = lc.kels(pre);
        assertTrue(exists);
        assertEq(s, seq, "seq");
        assertEq(last, said, "last digest");
    }

    // ---------------- real GLEIF data ----------------

    function test_root() public {
        bytes32 last = feed("root");
        report();
        assertHead(ROOT, 4, last);
        assertTrue(lc.anchorExists(ROOT, EXT, 0, EXT), "root anchors External dip");
    }

    function test_external() public {
        feed("root");
        bytes32 last = feed("ext");
        report();
        assertHead(EXT, 27, last);
        (,,,,,, bytes32 del,,) = lc.kels(EXT);
        assertEq(del, ROOT);
        bytes32 provDrt = 0x10f73b289dde6da644eedeffb25ea0bcd07bf19913c5846e77b620ccf7a6bc1f;
        assertTrue(lc.anchorExists(EXT, PROV, 0, PROV), "ixn 1 anchors Provenant dip");
        assertTrue(lc.anchorExists(EXT, PROV, 0x18, provDrt), "ixn 8 anchors Provenant drt");
        // Real-data quirk: External ixn 7 anchored the same drt with s "24" (decimal, not hex),
        // so the highest seal s for Provenant is 0x24, a sequence number Provenant never had.
        assertTrue(lc.anchorExists(EXT, PROV, 0x24, provDrt));
        assertTrue(lc.isAnchored(EXT, PROV, 0x24));
    }

    function test_provenant() public {
        feed("root");
        feed("ext");
        bytes32 last = feed("prov");
        report();
        assertHead(PROV, 69, last);
        // Every seal of the last Provenant ixn is queryable.
        string memory json = fixture("prov");
        (KeriLightClient.Event memory e,,) = load(json, 69);
        assertEq(e.a.length, 1);
        assertTrue(lc.anchorExists(PROV, e.a[0].i, e.a[0].s, e.a[0].d));
        assertFalse(lc.anchorExists(PROV, e.a[0].i, e.a[0].s + 1, e.a[0].d));
        assertTrue(lc.isAnchored(PROV, e.a[0].i, e.a[0].s));
        assertFalse(lc.isAnchored(PROV, e.a[0].i, e.a[0].s + 1));
    }

    // ---------------- negative ----------------

    function test_rejects_flipped_signature_bit() public {
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(fixture("root"), 0);
        sigs[0].r ^= bytes32(uint256(1));
        vm.expectRevert(bytes("bad sig"));
        lc.postEvent(e, sigs, wigs);
    }

    function test_rejects_flipped_receipt_bit() public {
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(fixture("root"), 0);
        wigs[2].s ^= bytes32(uint256(1) << 100);
        vm.expectRevert(bytes("bad receipt"));
        lc.postEvent(e, sigs, wigs);
    }

    function test_rejects_wrong_prior_digest() public {
        string memory json = fixture("root");
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(json, 0);
        lc.postEvent(e, sigs, wigs);
        (e, sigs, wigs) = load(json, 1);
        e.p ^= bytes32(uint256(1));
        vm.expectRevert(bytes("prior digest"));
        lc.postEvent(e, sigs, wigs);
    }

    function test_rejects_replay() public {
        string memory json = fixture("root");
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(json, 0);
        lc.postEvent(e, sigs, wigs);
        vm.expectRevert(bytes("exists"));
        lc.postEvent(e, sigs, wigs);
        (e, sigs, wigs) = load(json, 1);
        lc.postEvent(e, sigs, wigs);
        vm.expectRevert(bytes("sn"));
        lc.postEvent(e, sigs, wigs);
    }

    function test_rejects_too_few_receipts() public {
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(fixture("root"), 0);
        assertEq(e.bt, 4);
        assembly {
            mstore(wigs, 3)
        }
        vm.expectRevert(bytes("receipts below bt"));
        lc.postEvent(e, sigs, wigs);
    }

    function test_rejects_below_signing_threshold() public {
        // Root icp kt is 7 x 1/3: two of the three signatures are not enough.
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(fixture("root"), 0);
        assembly {
            mstore(sigs, 2)
        }
        vm.expectRevert(bytes("kt not met"));
        lc.postEvent(e, sigs, wigs);
    }

    function test_rejects_dip_without_anchor() public {
        // Root icp only: the seal of External's dip sits in Root rot 1, not posted yet.
        (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) =
            load(fixture("root"), 0);
        lc.postEvent(e, sigs, wigs);
        (e, sigs, wigs) = load(fixture("ext"), 0);
        vm.expectRevert(bytes("no delegation anchor"));
        lc.postEvent(e, sigs, wigs);
    }

    // ---------------- fake chain: credentials and address links ----------------

    function cred(uint256 i)
        internal
        view
        returns (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss, bytes32 said)
    {
        return cred("fake_creds", i);
    }

    function cred(string memory set, uint256 i)
        internal
        view
        returns (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss, bytes32 said)
    {
        string memory json = fixture(set);
        string memory p = string.concat(".creds[", vm.toString(i), "].");
        (acdc, off, iss) = abi.decode(
            vm.parseJsonBytes(json, string.concat(p, "args")),
            (bytes, KeriLightClient.Offsets, KeriLightClient.IssFields)
        );
        said = vm.parseJsonBytes32(json, string.concat(p, "said"));
    }

    function register(uint256 i) internal returns (bytes32 said) {
        return register("fake_creds", i);
    }

    function register(string memory set, uint256 i) internal returns (bytes32 said) {
        (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss, bytes32 s) =
            cred(set, i);
        uint256 g = gasleft();
        lc.registerCredential(acdc, off, iss);
        g -= gasleft();
        console.log(string.concat("GAS registerCredential ", vm.parseJsonString(fixture(set), string.concat(".creds[", vm.toString(i), "].name")), " bytes ", vm.toString(acdc.length), " gas ", vm.toString(g)));
        return s;
    }

    function link(uint256 i) internal view returns (bytes32 said, address account, KeriLightClient.Sig[] memory sigs) {
        string memory json = fixture("fake_links");
        string memory p = string.concat(".links[", vm.toString(i), "].");
        said = vm.parseJsonBytes32(json, string.concat(p, "said"));
        account = vm.parseJsonAddress(json, string.concat(p, "account"));
        sigs = abi.decode(vm.parseJsonBytes(json, string.concat(p, "args")), (KeriLightClient.Sig[]));
    }

    function test_fake_chain() public {
        feed("fake_kels");
        report();
        bytes32[4] memory saids;
        for (uint256 i; i < 4; i++) saids[i] = register(i);
        assertTrue(lc.isValid(saids[QVI_CRED]));
        assertTrue(lc.isValid(saids[LE_CRED]));
        assertTrue(lc.isValid(saids[ECR_ILIA]));
        assertFalse(lc.isValid(saids[ECR_REVOKED]), "revoked");
        assertTrue(lc.isAnchored(AVALABS, saids[ECR_REVOKED], 1), "revocation anchor at Ava Labs sn 4");
        assertEq(lc.leiOf(saids[LE_CRED]), "254900FM9C1FUTT81V36");
        assertEq(lc.leiOf(saids[ECR_ILIA]), "254900FM9C1FUTT81V36");
        assertEq(lc.leiOf(saids[QVI_CRED]), "");
        (bytes32 issuer, bytes32 holder, bytes32 schema, bytes32 parent,) = lc.credentials(saids[ECR_ILIA]);
        assertEq(issuer, AVALABS);
        assertEq(holder, ILIA);
        assertEq(schema, ECR_SCHEMA);
        assertEq(parent, saids[LE_CRED]);

        (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss,) = cred(LE_CRED);
        vm.expectRevert(bytes("registered"));
        lc.registerCredential(acdc, off, iss);

        for (uint256 i; i < 2; i++) {
            (bytes32 said, address account, KeriLightClient.Sig[] memory sigs) = link(i);
            uint256 g = gasleft();
            vm.expectEmit();
            emit KeriLightClient.Linked(account, said);
            lc.linkAddress(said, account, sigs);
            g -= gasleft();
            console.log(string.concat("GAS linkAddress ", vm.toString(account), " gas ", vm.toString(g)));
            assertEq(lc.credentialOf(account), said);
            (string memory lei, bool valid) = lc.leiOfAddress(account);
            assertEq(lei, "254900FM9C1FUTT81V36");
            assertEq(valid, i == 0, "Ilia valid, Revoked Person not");
        }
        (string memory l, bool v) = lc.leiOfAddress(address(1));
        assertEq(l, "");
        assertFalse(v);
    }

    function fakeUpTo(uint256 n) internal {
        feed("fake_kels");
        for (uint256 i; i < n; i++) register(i);
    }

    function test_rejects_wrong_link_signature() public {
        fakeUpTo(4);
        (bytes32 said, address account, KeriLightClient.Sig[] memory sigs) = link(0);
        sigs[0].s ^= bytes32(uint256(1));
        vm.expectRevert(bytes("bad sig"));
        lc.linkAddress(said, account, sigs);
        // Ilia's valid signature does not cover another address.
        (,, sigs) = link(0);
        vm.expectRevert(bytes("bad sig"));
        lc.linkAddress(said, address(0xBEEF), sigs);
        // Ilia's signature does not satisfy the Revoked Person's keys.
        (bytes32 revSaid,,) = link(1);
        vm.expectRevert(bytes("bad sig"));
        lc.linkAddress(revSaid, account, sigs);
    }

    function test_rejects_wrong_offsets() public {
        fakeUpTo(1);
        (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss,) = cred(LE_CRED);
        KeriLightClient.Offsets memory bad = off;
        bad.edge += 1; // key text mismatch
        vm.expectRevert(bytes("key"));
        lc.registerCredential(acdc, bad, iss);
        (,off,,) = cred(LE_CRED);
        off.i = off.ai; // a.i has the same key text, but not the pinned top-level position
        vm.expectRevert(bytes("i offset"));
        lc.registerCredential(acdc, off, iss);
        (,off,,) = cred(LE_CRED);
        off.lei -= 2;
        vm.expectRevert(bytes("LEI offset"));
        lc.registerCredential(acdc, off, iss);
    }

    function test_rejects_tampered_credential() public {
        fakeUpTo(1);
        (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss,) = cred(LE_CRED);
        acdc[off.lei + 19] = "7"; // last LEI char 6 -> 7
        vm.expectRevert(bytes("said"));
        lc.registerCredential(acdc, off, iss);
    }

    function test_rejects_iss_wrong_registry() public {
        fakeUpTo(1);
        (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss,) = cred(LE_CRED);
        iss.registry ^= bytes32(uint256(1));
        vm.expectRevert(bytes("no iss anchor"));
        lc.registerCredential(acdc, off, iss);
    }

    function test_rejects_unregistered_parent() public {
        feed("fake_kels");
        // LE before its QVI credential.
        (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss,) = cred(LE_CRED);
        vm.expectRevert(bytes("parent"));
        lc.registerCredential(acdc, off, iss);
        // ECR before its LE credential.
        register(QVI_CRED);
        (acdc, off, iss,) = cred(ECR_ILIA);
        vm.expectRevert(bytes("parent"));
        lc.registerCredential(acdc, off, iss);
    }

    // ---------------- real production credentials (GLEIF annual report signatures) ----------------

    /// Real Root, External, Provenant and GLEIF Internal histories (testdata/realcred/fetched), real GLEIF External as trust root.
    function realChain() internal {
        lc = new KeriLightClient(EXT, QVI_SCHEMA, LE_SCHEMA, ECR_SCHEMA);
        feed("real_kels");
        report();
    }

    function test_real_credentials() public {
        realChain();
        bytes32 qvi = register("real_creds", 0); // Provenant QVI, issued by GLEIF External, anchored External sn 4
        bytes32 le = register("real_creds", 1); // GLEIF LE, issued by Provenant, anchored Provenant sn 2
        assertEq(qvi, 0xc6ad52acffbbd062cf1c0e737262b210b83325df896cd069d0e9802040e97f67); // EMatUqz_u9Bi...
        assertEq(le, 0x1f7219b5973a7fcc85342c1e5b5fb40b0dae5580c6079d30f210978ae0e82d5e); // EB9yGbWXOn_M...
        assertTrue(lc.isValid(qvi));
        assertTrue(lc.isValid(le));
        assertEq(lc.leiOf(le), "506700GE1G29325QX363");
        (bytes32 issuer, bytes32 holder, bytes32 schema, bytes32 parent,) = lc.credentials(le);
        assertEq(issuer, PROV);
        assertEq(holder, INTERNAL);
        assertEq(schema, LE_SCHEMA);
        assertEq(parent, qvi);
        (issuer, holder,,,) = lc.credentials(qvi);
        assertEq(issuer, EXT);
        assertEq(holder, PROV);
    }

    function test_real_revocations() public {
        realChain();
        (,,,,,, bytes32 del,,) = lc.kels(INTERNAL);
        assertEq(del, ROOT, "Internal delegated by Root");
        // OOR authorization of the 2024 CFO (EKiLM8f_...), revoked by GLEIF Internal at sn 0xf.
        bytes32 auth = 0xa88b33c7ff9620305526f63da50c41381efa4c7c4c1243d075d8f3268607574a;
        assertTrue(lc.isAnchored(INTERNAL, auth, 0));
        assertTrue(lc.isAnchored(INTERNAL, auth, 1));
        // The CFO OOR credential itself (EFv2NH9O...), revoked by Provenant at sn 0x30.
        bytes32 oor = 0x5bf6347f4e00450afc6bcf318dbeff2cb0db29a0467587f32e6d2de9b55cae85;
        assertTrue(lc.isAnchored(PROV, oor, 0));
        assertTrue(lc.isAnchored(PROV, oor, 1));
        // Not revoked: the QVI and LE credentials.
        assertFalse(lc.isAnchored(EXT, 0xc6ad52acffbbd062cf1c0e737262b210b83325df896cd069d0e9802040e97f67, 1));
        assertFalse(lc.isAnchored(PROV, 0x1f7219b5973a7fcc85342c1e5b5fb40b0dae5580c6079d30f210978ae0e82d5e, 1));
    }

    // ---------------- Blake3 multi-chunk ----------------

    function pat(uint256 len) internal pure returns (bytes memory out) {
        out = new bytes(len);
        for (uint256 i; i < len; i++) out[i] = bytes1(uint8(i % 251));
    }

    // Python blake3 1.0.11 over byte i = i % 251 (the official BLAKE3 test vector input).
    function test_blake3_vectors() public pure {
        uint256[13] memory lens = [uint256(0), 44, 1023, 1024, 1025, 1181, 2048, 2049, 3072, 4096, 4097, 5121, 9217];
        bytes32[13] memory want = [
            bytes32(0xaf1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262),
            0x062c1bce7948a70db0d55355f0e74f13e54a022e8fffceab06fe71041d0aad47,
            0x10108970eeda3eb932baac1428c7a2163b0e924c9a9e25b35bba72b28f70bd11,
            0x42214739f095a406f3fc83deb889744ac00df831c10daa55189b5d121c855af7,
            0xd00278ae47eb27b34faecf67b4fe263f82d5412916c1ffd97c8cb7fb814b8444,
            0x5d1685ca6008c756d638012eb28ebba8c5b09932b9c9c716a40c7dfe5dc41e24,
            0xe776b6028c7cd22a4d0ba182a8bf62205d2ef576467e838ed6f2529b85fba24a,
            0x5f4d72f40d7a5f82b15ca2b2e44b1de3c2ef86c426c95c1af0b6879522563030,
            0xb98cb0ff3623be03326b373de6b9095218513e64f1ee2edd2525c7ad1e5cffd2,
            0x015094013f57a5277b59d8475c0501042c0b642e531b0a1c8f58d2163229e969,
            0x9b4052b38f1c5fc8b1f9ff7ac7b27cd242487b3d890d15c96a1c25b8aa0fb995,
            0x628bd2cb2004694adaab7bbd778a25df25c47b9d4155a55f8fbd79f2fe154cff,
            0xd42c90aa30bee83ecb52ad31b685d566145649496764878873598cef582d4d8f
        ];
        for (uint256 i; i < lens.length; i++) {
            assertEq(Blake3Yul.hash(pat(lens[i])), want[i]);
        }
    }
}
