// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {p, d, pMINUS_1} from "@solidity/fields/SCL_wei25519.sol";
import {ModInv} from "@solidity/modular/SCL_modular.sol";
import "@solidity/modular/SCL_sqrtMod_5mod8.sol";
import "@solidity/lib/libSCL_EIP6565.sol";

/// Builds the SCL "extended public key" from a raw 32-byte RFC 8032 Ed25519 public key, on chain.
/// SCL Verify_LE needs it; SCL only ships an off-chain SetKey(secret). Decompression copied from
/// SCL test/libSCL_eip6565.t.sol edDecompressX.
library SclEdKey {
    function extKey(bytes32 pk) internal returns (uint256[5] memory ext) {
        uint256 kc = SCL_sha512.Swap256(uint256(pk)); // RFC 8032 bytes are little-endian
        uint256 sign = kc >> 255;
        uint256 y = kc & ((1 << 255) - 1);
        require(y < p, "y >= p");
        uint256 y2 = mulmod(y, y, p);
        uint256 x2 = mulmod(addmod(y2, pMINUS_1, p), ModInv(addmod(mulmod(d, y2, p), 1, p), p), p);
        uint256 x = SqrtMod(x2);
        require(mulmod(x, x, p) == x2, "not on curve");
        if ((x & 1) != sign) x = p - x;
        (ext[0], ext[1]) = SCL_EIP6565.Edwards2WeierStrass(x, y);
        (ext[2], ext[3]) = SCL_EIP6565.ecPow128(ext[0], ext[1], 1, 1);
        ext[4] = uint256(pk);
    }
}
