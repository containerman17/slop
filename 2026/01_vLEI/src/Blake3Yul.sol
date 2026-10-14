// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// Minimal BLAKE3-256 (unkeyed, 32-byte output), any input length.
/// Copied from gasbench/src/Blake3Yul.sol (single chunk) and extended with the chunk tree:
/// each 1 KiB chunk gets its counter, chunk CVs merge into parent nodes, the last node gets ROOT.
/// Each state row (4 x 32-bit words) is packed into one 256-bit word as 4 x 64-bit lanes,
/// so one pass of the code below runs 4 G functions at once (SIMD style).
library Blake3Yul {
    function hash(bytes memory data) internal pure returns (bytes32 r) {
        assembly ("memory-safe") {
            // Byte-swap every 32-bit lane of a 256-bit word (little-endian word load).
            function swap32(w) -> v {
                v := or(shr(8, and(w, 0xff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00)),
                        shl(8, and(w, 0x00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff00ff)))
                v := or(shr(16, and(v, 0xffff0000ffff0000ffff0000ffff0000ffff0000ffff0000ffff0000ffff0000)),
                        shl(16, and(v, 0x0000ffff0000ffff0000ffff0000ffff0000ffff0000ffff0000ffff0000ffff)))
            }
            // One compression. cvp holds two packed rows (v0..v3, v4..v7); m holds 16 message words,
            // one per 32-byte slot; d0 is the packed 4th row (counter lo, counter hi, block len, flags).
            // Returns the packed new chaining value (row0 ^ row2, row1 ^ row3).
            function compress(cvp, m, d0) -> a, b {
                let L := 0x00000000ffffffff00000000ffffffff00000000ffffffff00000000ffffffff
                a := mload(cvp)
                b := mload(add(cvp, 0x20))
                // IV[0..3], lane i at bits 64*i
                let c := 0x00000000a54ff53a000000003c6ef37200000000bb67ae85000000006a09e667
                let d := d0
                let x
                let y
                // round 0: columns
                x := or(or(mload(add(m, 0x000)), shl(64, mload(add(m, 0x040)))), or(shl(128, mload(add(m, 0x080))), shl(192, mload(add(m, 0x0c0)))))
                y := or(or(mload(add(m, 0x020)), shl(64, mload(add(m, 0x060)))), or(shl(128, mload(add(m, 0x0a0))), shl(192, mload(add(m, 0x0e0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x100)), shl(64, mload(add(m, 0x140)))), or(shl(128, mload(add(m, 0x180))), shl(192, mload(add(m, 0x1c0)))))
                y := or(or(mload(add(m, 0x120)), shl(64, mload(add(m, 0x160)))), or(shl(128, mload(add(m, 0x1a0))), shl(192, mload(add(m, 0x1e0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                // round 1: columns
                x := or(or(mload(add(m, 0x040)), shl(64, mload(add(m, 0x060)))), or(shl(128, mload(add(m, 0x0e0))), shl(192, mload(add(m, 0x080)))))
                y := or(or(mload(add(m, 0x0c0)), shl(64, mload(add(m, 0x140)))), or(shl(128, mload(add(m, 0x000))), shl(192, mload(add(m, 0x1a0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x020)), shl(64, mload(add(m, 0x180)))), or(shl(128, mload(add(m, 0x120))), shl(192, mload(add(m, 0x1e0)))))
                y := or(or(mload(add(m, 0x160)), shl(64, mload(add(m, 0x0a0)))), or(shl(128, mload(add(m, 0x1c0))), shl(192, mload(add(m, 0x100)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                // round 2: columns
                x := or(or(mload(add(m, 0x060)), shl(64, mload(add(m, 0x140)))), or(shl(128, mload(add(m, 0x1a0))), shl(192, mload(add(m, 0x0e0)))))
                y := or(or(mload(add(m, 0x080)), shl(64, mload(add(m, 0x180)))), or(shl(128, mload(add(m, 0x040))), shl(192, mload(add(m, 0x1c0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x0c0)), shl(64, mload(add(m, 0x120)))), or(shl(128, mload(add(m, 0x160))), shl(192, mload(add(m, 0x100)))))
                y := or(or(mload(add(m, 0x0a0)), shl(64, mload(add(m, 0x000)))), or(shl(128, mload(add(m, 0x1e0))), shl(192, mload(add(m, 0x020)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                // round 3: columns
                x := or(or(mload(add(m, 0x140)), shl(64, mload(add(m, 0x180)))), or(shl(128, mload(add(m, 0x1c0))), shl(192, mload(add(m, 0x1a0)))))
                y := or(or(mload(add(m, 0x0e0)), shl(64, mload(add(m, 0x120)))), or(shl(128, mload(add(m, 0x060))), shl(192, mload(add(m, 0x1e0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x080)), shl(64, mload(add(m, 0x160)))), or(shl(128, mload(add(m, 0x0a0))), shl(192, mload(add(m, 0x020)))))
                y := or(or(mload(add(m, 0x000)), shl(64, mload(add(m, 0x040)))), or(shl(128, mload(add(m, 0x100))), shl(192, mload(add(m, 0x0c0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                // round 4: columns
                x := or(or(mload(add(m, 0x180)), shl(64, mload(add(m, 0x120)))), or(shl(128, mload(add(m, 0x1e0))), shl(192, mload(add(m, 0x1c0)))))
                y := or(or(mload(add(m, 0x1a0)), shl(64, mload(add(m, 0x160)))), or(shl(128, mload(add(m, 0x140))), shl(192, mload(add(m, 0x100)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x0e0)), shl(64, mload(add(m, 0x0a0)))), or(shl(128, mload(add(m, 0x000))), shl(192, mload(add(m, 0x0c0)))))
                y := or(or(mload(add(m, 0x040)), shl(64, mload(add(m, 0x060)))), or(shl(128, mload(add(m, 0x020))), shl(192, mload(add(m, 0x080)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                // round 5: columns
                x := or(or(mload(add(m, 0x120)), shl(64, mload(add(m, 0x160)))), or(shl(128, mload(add(m, 0x100))), shl(192, mload(add(m, 0x1e0)))))
                y := or(or(mload(add(m, 0x1c0)), shl(64, mload(add(m, 0x0a0)))), or(shl(128, mload(add(m, 0x180))), shl(192, mload(add(m, 0x020)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x1a0)), shl(64, mload(add(m, 0x000)))), or(shl(128, mload(add(m, 0x040))), shl(192, mload(add(m, 0x080)))))
                y := or(or(mload(add(m, 0x060)), shl(64, mload(add(m, 0x140)))), or(shl(128, mload(add(m, 0x0c0))), shl(192, mload(add(m, 0x0e0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                // round 6: columns
                x := or(or(mload(add(m, 0x160)), shl(64, mload(add(m, 0x0a0)))), or(shl(128, mload(add(m, 0x020))), shl(192, mload(add(m, 0x100)))))
                y := or(or(mload(add(m, 0x1e0)), shl(64, mload(add(m, 0x000)))), or(shl(128, mload(add(m, 0x120))), shl(192, mload(add(m, 0x0c0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                // diagonals: lane i takes b[i+1], c[i+2], d[i+3]
                b := or(shr(64, b), shl(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shl(64, d), shr(192, d))
                x := or(or(mload(add(m, 0x1c0)), shl(64, mload(add(m, 0x040)))), or(shl(128, mload(add(m, 0x060))), shl(192, mload(add(m, 0x0e0)))))
                y := or(or(mload(add(m, 0x140)), shl(64, mload(add(m, 0x180)))), or(shl(128, mload(add(m, 0x080))), shl(192, mload(add(m, 0x1a0)))))
                a := and(add(add(a, b), x), L)
                d := xor(d, a)
                d := and(or(shr(16, d), shl(16, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(12, b), shl(20, b)), L)
                a := and(add(add(a, b), y), L)
                d := xor(d, a)
                d := and(or(shr(8, d), shl(24, d)), L)
                c := and(add(c, d), L)
                b := xor(b, c)
                b := and(or(shr(7, b), shl(25, b)), L)
                b := or(shl(64, b), shr(192, b))
                c := or(shr(128, c), shl(128, c))
                d := or(shr(64, d), shl(192, d))
                a := xor(a, c)
                b := xor(b, d)
            }

            // Parent node over the cvs at memory lp and rp: message = left || right, key = IV
            // (at cvp + 0x280), counter 0, block len 64. Result goes to cvp.
            function parent(cvp, lp, rp, flags) {
                let m := add(cvp, 0x40)
                let la := mload(lp)
                let lb := mload(add(lp, 0x20))
                let ra := mload(rp)
                let rb := mload(add(rp, 0x20))
                for { let k := 0 } lt(k, 4) { k := add(k, 1) } {
                    let s := mul(k, 64)
                    mstore(add(m, mul(k, 0x20)), and(shr(s, la), 0xffffffff))
                    mstore(add(m, mul(add(k, 4), 0x20)), and(shr(s, lb), 0xffffffff))
                    mstore(add(m, mul(add(k, 8), 0x20)), and(shr(s, ra), 0xffffffff))
                    mstore(add(m, mul(add(k, 12), 0x20)), and(shr(s, rb), 0xffffffff))
                }
                let a, b := compress(add(cvp, 0x280), m, or(shl(128, 64), shl(192, flags)))
                mstore(cvp, a)
                mstore(add(cvp, 0x20), b)
            }

            // Scratch memory past the free memory pointer: cv (2 slots), message (16 slots),
            // block (64 B) at +0x240, packed IV (2 slots) at +0x280, cv stack (2 slots per entry) from +0x2c0.
            let cvp := mload(0x40)
            let sp := add(cvp, 0x2c0)

            // Packed IV, lane i = IV[i] / IV[4 + i].
            mstore(add(cvp, 0x280), 0x00000000a54ff53a000000003c6ef37200000000bb67ae85000000006a09e667)
            mstore(add(cvp, 0x2a0), 0x000000005be0cd19000000001f83d9ab000000009b05688c00000000510e527f)

            let len := mload(data)
            let nchunks := div(add(len, 1023), 1024)
            if iszero(nchunks) { nchunks := 1 }

            for { let c := 0 } lt(c, nchunks) { c := add(c, 1) } {
                mstore(cvp, mload(add(cvp, 0x280)))
                mstore(add(cvp, 0x20), mload(add(cvp, 0x2a0)))
                let clen := sub(len, mul(c, 1024))
                if gt(clen, 1024) { clen := 1024 }
                let nblocks := div(add(clen, 63), 64)
                if iszero(nblocks) { nblocks := 1 }

                for { let i := 0 } lt(i, nblocks) { i := add(i, 1) } {
                    let buf := add(cvp, 0x240)
                    let blen := sub(clen, mul(i, 64))
                    if gt(blen, 64) { blen := 64 }
                    mstore(buf, 0)
                    mstore(add(buf, 0x20), 0)
                    mcopy(buf, add(add(data, 0x20), add(mul(c, 1024), mul(i, 64))), blen)

                    for { let h := 0 } lt(h, 2) { h := add(h, 1) } {
                        let w := swap32(mload(add(buf, mul(h, 0x20))))
                        let mp := add(cvp, add(0x40, mul(h, 0x100)))
                        for { let j := 0 } lt(j, 8) { j := add(j, 1) } {
                            mstore(add(mp, mul(j, 0x20)), and(shr(sub(224, mul(j, 32)), w), 0xffffffff))
                        }
                    }

                    let flags := 0
                    if iszero(i) { flags := 1 } // CHUNK_START
                    if eq(i, sub(nblocks, 1)) {
                        flags := or(flags, 2) // CHUNK_END
                        if eq(nchunks, 1) { flags := or(flags, 8) } // ROOT
                    }

                    let n0, n1 := compress(cvp, add(cvp, 0x40), or(c, or(shl(128, blen), shl(192, flags))))
                    mstore(cvp, n0)
                    mstore(add(cvp, 0x20), n1)
                }

                // Not the last chunk: push its cv, merging completed subtrees (BLAKE3 reference add_chunk_chaining_value).
                if lt(add(c, 1), nchunks) {
                    for { let total := add(c, 1) } iszero(and(total, 1)) { total := shr(1, total) } {
                        sp := sub(sp, 0x40)
                        parent(cvp, sp, cvp, 4) // PARENT
                    }
                    mstore(sp, mload(cvp))
                    mstore(add(sp, 0x20), mload(add(cvp, 0x20)))
                    sp := add(sp, 0x40)
                }
            }

            // Fold the stack into the last chunk's cv; the final parent is the ROOT node.
            for {} gt(sp, add(cvp, 0x2c0)) {} {
                sp := sub(sp, 0x40)
                let flags := 4 // PARENT
                if eq(sp, add(cvp, 0x2c0)) { flags := 12 } // PARENT | ROOT
                parent(cvp, sp, cvp, flags)
            }

            // Output bytes are the little-endian encoding of v0..v7 (lanes of the two cv words).
            let a := mload(cvp)
            let b := mload(add(cvp, 0x20))
            for { let k := 0 } lt(k, 4) { k := add(k, 1) } {
                r := or(r, shl(sub(224, mul(k, 32)), and(shr(mul(k, 64), a), 0xffffffff)))
                r := or(r, shl(sub(96, mul(k, 32)), and(shr(mul(k, 64), b), 0xffffffff)))
            }
            r := swap32(r)
        }
    }
}
