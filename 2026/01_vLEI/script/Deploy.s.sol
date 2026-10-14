// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import {KeriLightClient} from "../src/KeriLightClient.sol";

/// Deploys KeriLightClient. Trust root: the fake chain External (testdata/fakechain) by default, or EXTERNAL
/// (bytes32), e.g. the real GLEIF External 0x83661dde60ee257e5257649242b20481f4e766c8cb164d067f69abeccdad8a12. Fuji only.
/// [EXTERNAL=0x..] forge script script/Deploy.s.sol --rpc-url $FUJI --broadcast --private-key $PK
contract Deploy is Script {
    function run() external returns (KeriLightClient lc) {
        require(block.chainid == 43113, "Fuji only");
        vm.startBroadcast();
        lc = new KeriLightClient(
            vm.envOr("EXTERNAL", bytes32(0x9a38c16a50eb0a514dbc71715db18ba12e9f49eabac7ddbfa6fea22586dca415)), // fake External EJo4wWpQ6wpRTbxxcV2xi6Eun0nqusfdv6b-oiWG3KQV
            0x17dd96ef11dbb15bc7e7a1aeac089a72bef029bbf6ca9b010e0c9af10062a6a8, // QVI schema EBfdlu8R27Fbx-ehrqwImnK-8Cm79sqbAQ4MmvEAYqao
            0xd3d7a75bd0cd117a27022e4be9a9d94f147f5328003a0ffc6a90eb656c0d9598, // LE schema ENPXp1vQzRF6JwIuS-mp2U8Uf1MoADoP_GqQ62VsDZWY
            0x4cbd3e48a415c00d57597b107ba409e5eaaf34f584d6631a5524ef0b8c3b98f0 // ECR schema EEy9PkikFcANV1l7EHukCeXqrzT1hNZjGlUk7wuMO5jw
        );
        vm.stopBroadcast();
    }
}
