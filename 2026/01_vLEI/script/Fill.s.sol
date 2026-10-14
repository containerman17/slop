// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import {KeriLightClient} from "../src/KeriLightClient.sol";

/// Posts every KEL event of test/fixtures/<SET>_kels.json, registers <SET>_creds.json and links
/// <SET>_links.json (if present), all from script/extract.py. SET is "fake" (default) or "real". Fuji only.
/// Resumable: skips events whose sn the contract already stores, registered credentials and existing links.
/// SET=real LC=<address> forge script script/Fill.s.sol --rpc-url $FUJI --broadcast --slow --private-key $PK
contract Fill is Script {
    function fixture(string memory name) internal view returns (string memory) {
        return vm.readFile(string.concat(vm.projectRoot(), "/test/fixtures/", name, ".json"));
    }

    function run() external {
        require(block.chainid == 43113, "Fuji only");
        KeriLightClient lc = KeriLightClient(vm.envAddress("LC"));
        string memory set = vm.envOr("SET", string("fake"));
        string memory kels = fixture(string.concat(set, "_kels"));
        string memory creds = fixture(string.concat(set, "_creds"));
        vm.startBroadcast();
        for (uint256 i; i < vm.parseJsonUint(kels, ".n"); i++) {
            (KeriLightClient.Event memory e, KeriLightClient.Sig[] memory sigs, KeriLightClient.Sig[] memory wigs) = abi.decode(
                vm.parseJsonBytes(kels, string.concat(".events[", vm.toString(i), "].args")),
                (KeriLightClient.Event, KeriLightClient.Sig[], KeriLightClient.Sig[])
            );
            (bool exists,,, uint64 seq,,,,,) = lc.kels(e.i);
            if (exists && seq >= e.s) continue;
            lc.postEvent(e, sigs, wigs);
        }
        for (uint256 i; i < vm.parseJsonUint(creds, ".n"); i++) {
            string memory p = string.concat(".creds[", vm.toString(i), "].");
            (, bytes32 holder,,,) = lc.credentials(vm.parseJsonBytes32(creds, string.concat(p, "said")));
            if (holder != 0) continue;
            (bytes memory acdc, KeriLightClient.Offsets memory off, KeriLightClient.IssFields memory iss) = abi.decode(
                vm.parseJsonBytes(creds, string.concat(p, "args")),
                (bytes, KeriLightClient.Offsets, KeriLightClient.IssFields)
            );
            lc.registerCredential(acdc, off, iss);
        }
        string memory linksPath = string.concat(vm.projectRoot(), "/test/fixtures/", set, "_links.json");
        string memory links = vm.exists(linksPath) ? vm.readFile(linksPath) : '{"n":0}';
        for (uint256 i; i < vm.parseJsonUint(links, ".n"); i++) {
            string memory p = string.concat(".links[", vm.toString(i), "].");
            bytes32 said = vm.parseJsonBytes32(links, string.concat(p, "said"));
            address account = vm.parseJsonAddress(links, string.concat(p, "account"));
            if (lc.credentialOf(account) == said) continue;
            lc.linkAddress(said, account, abi.decode(vm.parseJsonBytes(links, string.concat(p, "args")), (KeriLightClient.Sig[])));
        }
        vm.stopBroadcast();
    }
}
