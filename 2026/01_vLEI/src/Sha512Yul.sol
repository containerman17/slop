// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// SHA-512 in inline assembly (64-bit words in 256-bit slots, masked). Returns the 64-byte digest
/// as two big-endian halves.
library Sha512Yul {
    function hash(bytes memory data) internal pure returns (bytes32 hi, bytes32 lo) {
        assembly ("memory-safe") {
            // One compression over W (80 slots at w, K table at w + 0xa00) into H (8 slots at w - 0x100).
            function compress(w) {
                let M := 0xffffffffffffffff
                let hp := sub(w, 0x100)
                let a := mload(hp)
                let b := mload(add(hp, 0x20))
                let c := mload(add(hp, 0x40))
                let d := mload(add(hp, 0x60))
                let e := mload(add(hp, 0x80))
                let f := mload(add(hp, 0xa0))
                let g := mload(add(hp, 0xc0))
                let h := mload(add(hp, 0xe0))
                for { let q := w } lt(q, add(w, 0xa00)) { q := add(q, 0x100) } {
                    // round t+0
                    let t10 := and(add(add(add(h, and(xor(xor(or(shr(14, e), shl(50, e)), or(shr(18, e), shl(46, e))), or(shr(41, e), shl(23, e))), M)), xor(g, and(e, xor(f, g)))), add(mload(add(q, 0x000)), mload(add(q, 0xa00)))), M)
                    d := and(add(d, t10), M)
                    h := and(add(t10, add(and(xor(xor(or(shr(28, a), shl(36, a)), or(shr(34, a), shl(30, a))), or(shr(39, a), shl(25, a))), M), or(and(a, b), and(c, or(a, b))))), M)
                    // round t+1
                    let t11 := and(add(add(add(g, and(xor(xor(or(shr(14, d), shl(50, d)), or(shr(18, d), shl(46, d))), or(shr(41, d), shl(23, d))), M)), xor(f, and(d, xor(e, f)))), add(mload(add(q, 0x020)), mload(add(q, 0xa20)))), M)
                    c := and(add(c, t11), M)
                    g := and(add(t11, add(and(xor(xor(or(shr(28, h), shl(36, h)), or(shr(34, h), shl(30, h))), or(shr(39, h), shl(25, h))), M), or(and(h, a), and(b, or(h, a))))), M)
                    // round t+2
                    let t12 := and(add(add(add(f, and(xor(xor(or(shr(14, c), shl(50, c)), or(shr(18, c), shl(46, c))), or(shr(41, c), shl(23, c))), M)), xor(e, and(c, xor(d, e)))), add(mload(add(q, 0x040)), mload(add(q, 0xa40)))), M)
                    b := and(add(b, t12), M)
                    f := and(add(t12, add(and(xor(xor(or(shr(28, g), shl(36, g)), or(shr(34, g), shl(30, g))), or(shr(39, g), shl(25, g))), M), or(and(g, h), and(a, or(g, h))))), M)
                    // round t+3
                    let t13 := and(add(add(add(e, and(xor(xor(or(shr(14, b), shl(50, b)), or(shr(18, b), shl(46, b))), or(shr(41, b), shl(23, b))), M)), xor(d, and(b, xor(c, d)))), add(mload(add(q, 0x060)), mload(add(q, 0xa60)))), M)
                    a := and(add(a, t13), M)
                    e := and(add(t13, add(and(xor(xor(or(shr(28, f), shl(36, f)), or(shr(34, f), shl(30, f))), or(shr(39, f), shl(25, f))), M), or(and(f, g), and(h, or(f, g))))), M)
                    // round t+4
                    let t14 := and(add(add(add(d, and(xor(xor(or(shr(14, a), shl(50, a)), or(shr(18, a), shl(46, a))), or(shr(41, a), shl(23, a))), M)), xor(c, and(a, xor(b, c)))), add(mload(add(q, 0x080)), mload(add(q, 0xa80)))), M)
                    h := and(add(h, t14), M)
                    d := and(add(t14, add(and(xor(xor(or(shr(28, e), shl(36, e)), or(shr(34, e), shl(30, e))), or(shr(39, e), shl(25, e))), M), or(and(e, f), and(g, or(e, f))))), M)
                    // round t+5
                    let t15 := and(add(add(add(c, and(xor(xor(or(shr(14, h), shl(50, h)), or(shr(18, h), shl(46, h))), or(shr(41, h), shl(23, h))), M)), xor(b, and(h, xor(a, b)))), add(mload(add(q, 0x0a0)), mload(add(q, 0xaa0)))), M)
                    g := and(add(g, t15), M)
                    c := and(add(t15, add(and(xor(xor(or(shr(28, d), shl(36, d)), or(shr(34, d), shl(30, d))), or(shr(39, d), shl(25, d))), M), or(and(d, e), and(f, or(d, e))))), M)
                    // round t+6
                    let t16 := and(add(add(add(b, and(xor(xor(or(shr(14, g), shl(50, g)), or(shr(18, g), shl(46, g))), or(shr(41, g), shl(23, g))), M)), xor(a, and(g, xor(h, a)))), add(mload(add(q, 0x0c0)), mload(add(q, 0xac0)))), M)
                    f := and(add(f, t16), M)
                    b := and(add(t16, add(and(xor(xor(or(shr(28, c), shl(36, c)), or(shr(34, c), shl(30, c))), or(shr(39, c), shl(25, c))), M), or(and(c, d), and(e, or(c, d))))), M)
                    // round t+7
                    let t17 := and(add(add(add(a, and(xor(xor(or(shr(14, f), shl(50, f)), or(shr(18, f), shl(46, f))), or(shr(41, f), shl(23, f))), M)), xor(h, and(f, xor(g, h)))), add(mload(add(q, 0x0e0)), mload(add(q, 0xae0)))), M)
                    e := and(add(e, t17), M)
                    a := and(add(t17, add(and(xor(xor(or(shr(28, b), shl(36, b)), or(shr(34, b), shl(30, b))), or(shr(39, b), shl(25, b))), M), or(and(b, c), and(d, or(b, c))))), M)
                }
                mstore(hp, and(add(mload(hp), a), M))
                mstore(add(hp, 0x20), and(add(mload(add(hp, 0x20)), b), M))
                mstore(add(hp, 0x40), and(add(mload(add(hp, 0x40)), c), M))
                mstore(add(hp, 0x60), and(add(mload(add(hp, 0x60)), d), M))
                mstore(add(hp, 0x80), and(add(mload(add(hp, 0x80)), e), M))
                mstore(add(hp, 0xa0), and(add(mload(add(hp, 0xa0)), f), M))
                mstore(add(hp, 0xc0), and(add(mload(add(hp, 0xc0)), g), M))
                mstore(add(hp, 0xe0), and(add(mload(add(hp, 0xe0)), h), M))
            }

            let M := 0xffffffffffffffff
            // Scratch past the free memory pointer: H (8 slots), W (80), K (80), block (128 B).
            let hp := mload(0x40)
            let w := add(hp, 0x100)
            let kp := add(w, 0xa00)
            let buf := add(kp, 0xa00)

            mstore(hp, 0x6a09e667f3bcc908)
            mstore(add(hp, 0x20), 0xbb67ae8584caa73b)
            mstore(add(hp, 0x40), 0x3c6ef372fe94f82b)
            mstore(add(hp, 0x60), 0xa54ff53a5f1d36f1)
            mstore(add(hp, 0x80), 0x510e527fade682d1)
            mstore(add(hp, 0xa0), 0x9b05688c2b3e6c1f)
            mstore(add(hp, 0xc0), 0x1f83d9abfb41bd6b)
            mstore(add(hp, 0xe0), 0x5be0cd19137e2179)
            mstore(add(kp, 0x000), 0x428a2f98d728ae22)
            mstore(add(kp, 0x020), 0x7137449123ef65cd)
            mstore(add(kp, 0x040), 0xb5c0fbcfec4d3b2f)
            mstore(add(kp, 0x060), 0xe9b5dba58189dbbc)
            mstore(add(kp, 0x080), 0x3956c25bf348b538)
            mstore(add(kp, 0x0a0), 0x59f111f1b605d019)
            mstore(add(kp, 0x0c0), 0x923f82a4af194f9b)
            mstore(add(kp, 0x0e0), 0xab1c5ed5da6d8118)
            mstore(add(kp, 0x100), 0xd807aa98a3030242)
            mstore(add(kp, 0x120), 0x12835b0145706fbe)
            mstore(add(kp, 0x140), 0x243185be4ee4b28c)
            mstore(add(kp, 0x160), 0x550c7dc3d5ffb4e2)
            mstore(add(kp, 0x180), 0x72be5d74f27b896f)
            mstore(add(kp, 0x1a0), 0x80deb1fe3b1696b1)
            mstore(add(kp, 0x1c0), 0x9bdc06a725c71235)
            mstore(add(kp, 0x1e0), 0xc19bf174cf692694)
            mstore(add(kp, 0x200), 0xe49b69c19ef14ad2)
            mstore(add(kp, 0x220), 0xefbe4786384f25e3)
            mstore(add(kp, 0x240), 0x0fc19dc68b8cd5b5)
            mstore(add(kp, 0x260), 0x240ca1cc77ac9c65)
            mstore(add(kp, 0x280), 0x2de92c6f592b0275)
            mstore(add(kp, 0x2a0), 0x4a7484aa6ea6e483)
            mstore(add(kp, 0x2c0), 0x5cb0a9dcbd41fbd4)
            mstore(add(kp, 0x2e0), 0x76f988da831153b5)
            mstore(add(kp, 0x300), 0x983e5152ee66dfab)
            mstore(add(kp, 0x320), 0xa831c66d2db43210)
            mstore(add(kp, 0x340), 0xb00327c898fb213f)
            mstore(add(kp, 0x360), 0xbf597fc7beef0ee4)
            mstore(add(kp, 0x380), 0xc6e00bf33da88fc2)
            mstore(add(kp, 0x3a0), 0xd5a79147930aa725)
            mstore(add(kp, 0x3c0), 0x06ca6351e003826f)
            mstore(add(kp, 0x3e0), 0x142929670a0e6e70)
            mstore(add(kp, 0x400), 0x27b70a8546d22ffc)
            mstore(add(kp, 0x420), 0x2e1b21385c26c926)
            mstore(add(kp, 0x440), 0x4d2c6dfc5ac42aed)
            mstore(add(kp, 0x460), 0x53380d139d95b3df)
            mstore(add(kp, 0x480), 0x650a73548baf63de)
            mstore(add(kp, 0x4a0), 0x766a0abb3c77b2a8)
            mstore(add(kp, 0x4c0), 0x81c2c92e47edaee6)
            mstore(add(kp, 0x4e0), 0x92722c851482353b)
            mstore(add(kp, 0x500), 0xa2bfe8a14cf10364)
            mstore(add(kp, 0x520), 0xa81a664bbc423001)
            mstore(add(kp, 0x540), 0xc24b8b70d0f89791)
            mstore(add(kp, 0x560), 0xc76c51a30654be30)
            mstore(add(kp, 0x580), 0xd192e819d6ef5218)
            mstore(add(kp, 0x5a0), 0xd69906245565a910)
            mstore(add(kp, 0x5c0), 0xf40e35855771202a)
            mstore(add(kp, 0x5e0), 0x106aa07032bbd1b8)
            mstore(add(kp, 0x600), 0x19a4c116b8d2d0c8)
            mstore(add(kp, 0x620), 0x1e376c085141ab53)
            mstore(add(kp, 0x640), 0x2748774cdf8eeb99)
            mstore(add(kp, 0x660), 0x34b0bcb5e19b48a8)
            mstore(add(kp, 0x680), 0x391c0cb3c5c95a63)
            mstore(add(kp, 0x6a0), 0x4ed8aa4ae3418acb)
            mstore(add(kp, 0x6c0), 0x5b9cca4f7763e373)
            mstore(add(kp, 0x6e0), 0x682e6ff3d6b2b8a3)
            mstore(add(kp, 0x700), 0x748f82ee5defb2fc)
            mstore(add(kp, 0x720), 0x78a5636f43172f60)
            mstore(add(kp, 0x740), 0x84c87814a1f0ab72)
            mstore(add(kp, 0x760), 0x8cc702081a6439ec)
            mstore(add(kp, 0x780), 0x90befffa23631e28)
            mstore(add(kp, 0x7a0), 0xa4506cebde82bde9)
            mstore(add(kp, 0x7c0), 0xbef9a3f7b2c67915)
            mstore(add(kp, 0x7e0), 0xc67178f2e372532b)
            mstore(add(kp, 0x800), 0xca273eceea26619c)
            mstore(add(kp, 0x820), 0xd186b8c721c0c207)
            mstore(add(kp, 0x840), 0xeada7dd6cde0eb1e)
            mstore(add(kp, 0x860), 0xf57d4f7fee6ed178)
            mstore(add(kp, 0x880), 0x06f067aa72176fba)
            mstore(add(kp, 0x8a0), 0x0a637dc5a2c898a6)
            mstore(add(kp, 0x8c0), 0x113f9804bef90dae)
            mstore(add(kp, 0x8e0), 0x1b710b35131c471b)
            mstore(add(kp, 0x900), 0x28db77f523047d84)
            mstore(add(kp, 0x920), 0x32caab7b40c72493)
            mstore(add(kp, 0x940), 0x3c9ebe0a15c9bebc)
            mstore(add(kp, 0x960), 0x431d67c49c100d4c)
            mstore(add(kp, 0x980), 0x4cc5d4becb3e42b6)
            mstore(add(kp, 0x9a0), 0x597f299cfc657e2a)
            mstore(add(kp, 0x9c0), 0x5fcb6fab3ad6faec)
            mstore(add(kp, 0x9e0), 0x6c44198c4a475817)

            let len := mload(data)
            let nb := div(add(len, 144), 128) // ceil((len + 1 + 16) / 128)
            for { let i := 0 } lt(i, nb) { i := add(i, 1) } {
                let off := mul(i, 128)
                mstore(buf, 0)
                mstore(add(buf, 0x20), 0)
                mstore(add(buf, 0x40), 0)
                mstore(add(buf, 0x60), 0)
                if lt(off, len) {
                    let n := sub(len, off)
                    if gt(n, 128) { n := 128 }
                    mcopy(buf, add(add(data, 0x20), off), n)
                }
                if iszero(lt(len, off)) { if lt(len, add(off, 128)) { mstore8(add(buf, sub(len, off)), 0x80) } }
                if eq(i, sub(nb, 1)) { mstore(add(buf, 0x60), or(mload(add(buf, 0x60)), shl(3, len))) }

                for { let j := 0 } lt(j, 4) { j := add(j, 1) } {
                    let x := mload(add(buf, mul(j, 0x20)))
                    let p := add(w, mul(j, 0x80))
                    mstore(p, shr(192, x))
                    mstore(add(p, 0x20), and(shr(128, x), M))
                    mstore(add(p, 0x40), and(shr(64, x), M))
                    mstore(add(p, 0x60), and(x, M))
                }
                for { let t := add(w, 0x200) } lt(t, add(w, 0xa00)) { t := add(t, 0x20) } {
                    let x2 := mload(sub(t, 0x40))
                    let x15 := mload(sub(t, 0x1e0))
                    let s1 := xor(xor(or(shr(19, x2), shl(45, x2)), or(shr(61, x2), shl(3, x2))), shr(6, x2))
                    let s0 := xor(xor(or(shr(1, x15), shl(63, x15)), or(shr(8, x15), shl(56, x15))), shr(7, x15))
                    mstore(t, and(add(add(and(s1, M), mload(sub(t, 0xe0))), add(and(s0, M), mload(sub(t, 0x200)))), M))
                }
                compress(w)
            }
            hi := or(or(shl(192, mload(hp)), shl(128, mload(add(hp, 0x20)))), or(shl(64, mload(add(hp, 0x40))), mload(add(hp, 0x60))))
            lo := or(or(shl(192, mload(add(hp, 0x80))), shl(128, mload(add(hp, 0xa0)))), or(shl(64, mload(add(hp, 0xc0))), mload(add(hp, 0xe0))))
        }
    }
}
