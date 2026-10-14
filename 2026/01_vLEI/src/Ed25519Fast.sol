// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {p, a, n, gx, gy} from "@solidity/fields/SCL_wei25519.sol";
import "@solidity/elliptic/SCL_mulmuladdX_fullgenW.sol";
import "@solidity/lib/libSCL_EIP6565.sol";
import {Sha512Yul} from "./Sha512Yul.sol";

/// SCL Verify_LE (get-smooth/crypto-lib libSCL_EIP6565.sol) with its Solidity SHA-512 swapped for Sha512Yul.
/// The elliptic-curve part is unchanged SCL code.
library Ed25519Fast {
    function verifyLE(bytes memory m, uint256 r, uint256 s, uint256[5] memory extKpub) internal returns (bool) {
        s = SCL_sha512.Swap256(s);
        if (s == 0 || s >= n) return false;
        uint256[6] memory Q = [extKpub[0], extKpub[1], p, a, gx, gy];
        (uint256 x, uint256 y) = ecGenMulmuladdB4W(Q, s, n - challenge(r, extKpub[4], m));
        (x, y) = SCL_EIP6565.WeierStrass2Edwards(x, y);
        return SCL_sha512.Swap256(y + ((x & 1) << 255)) == r;
    }

    /// k = SHA-512(R || A || M) mod L, inputs in RFC 8032 byte order.
    function challenge(uint256 r, uint256 kc, bytes memory m) private pure returns (uint256) {
        (bytes32 hi, bytes32 lo) = Sha512Yul.hash(abi.encodePacked(r, kc, m));
        return SCL_EIP6565.Red512Modq(SCL_sha512.Swap512([uint256(hi), uint256(lo)]));
    }
}
