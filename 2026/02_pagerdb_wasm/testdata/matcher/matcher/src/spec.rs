//! Fixture format, output accumulator and hash. Mirrors bench/go/spec/spec.go exactly.

pub const OP_ADD: u8 = 1;
pub const OP_CANCEL: u8 = 2;
pub const OP_REPLACE: u8 = 3;
pub const OP_MODIFY: u8 = 4;
pub const BUY: u8 = 1;
pub const SELL: u8 = 2;
pub const MSG_SIZE: usize = 40;
pub const FILL_SIZE: usize = 40;
pub const DELTA_SIZE: usize = 16;
pub const CASH_ASSET: u32 = 0xFFFF;

#[derive(Clone, Copy, Default, Debug)]
pub struct Msg {
    pub op: u8,
    pub side: u8,
    pub symbol: u16,
    pub broker: u16,
    pub client: u16,
    pub qty: u32,
    pub price: u64,
    pub order_id: u64,
    pub new_id: u64,
}

#[inline(always)]
fn u16le(b: &[u8], o: usize) -> u16 {
    u16::from_le_bytes([b[o], b[o + 1]])
}
#[inline(always)]
fn u32le(b: &[u8], o: usize) -> u32 {
    u32::from_le_bytes(b[o..o + 4].try_into().unwrap())
}
#[inline(always)]
fn u64le(b: &[u8], o: usize) -> u64 {
    u64::from_le_bytes(b[o..o + 8].try_into().unwrap())
}

#[inline(always)]
pub fn decode(b: &[u8]) -> Msg {
    let b = &b[..MSG_SIZE];
    Msg {
        op: b[0],
        side: b[1],
        symbol: u16le(b, 2),
        broker: u16le(b, 4),
        client: u16le(b, 6),
        qty: u32le(b, 8),
        price: u64le(b, 16),
        order_id: u64le(b, 24),
        new_id: u64le(b, 32),
    }
}

#[inline(always)]
pub fn symbol_of(b: &[u8]) -> u16 {
    u16le(b, 2)
}

/// One block's fills and balance deltas. Deltas live in an open-addressing table with a
/// touched list so reset and merge cost O(touched).
pub struct Out {
    fills: Vec<u8>,
    dkeys: Vec<u32>, // key+1, 0 = empty; key = broker<<16 | asset
    dvals: Vec<i64>,
    touched: Vec<u32>,
    mask: u32,
    keys: Vec<u32>,
    buf: Vec<u8>,
    idx: Vec<u64>,
    sorted: Vec<u8>,
}

impl Default for Out {
    fn default() -> Self {
        Self::new()
    }
}

impl Out {
    pub fn new() -> Self {
        const CAP: usize = 1 << 15;
        Out {
            fills: Vec::with_capacity(1 << 17),
            dkeys: vec![0; CAP],
            dvals: vec![0; CAP],
            touched: Vec::with_capacity(1 << 12),
            mask: (CAP - 1) as u32,
            keys: Vec::with_capacity(1 << 12),
            buf: Vec::with_capacity(1 << 17),
            idx: Vec::with_capacity(1 << 12),
            sorted: Vec::with_capacity(1 << 17),
        }
    }

    pub fn reset(&mut self) {
        self.fills.clear();
        for &i in &self.touched {
            self.dkeys[i as usize] = 0;
            self.dvals[i as usize] = 0;
        }
        self.touched.clear();
        self.keys.clear();
        self.buf.clear();
    }

    #[inline]
    fn slot(&mut self, key: u32) -> usize {
        let k = key + 1;
        let mut i = key.wrapping_mul(2654435761) & self.mask;
        loop {
            let kk = self.dkeys[i as usize];
            if kk == k {
                return i as usize;
            }
            if kk == 0 {
                self.dkeys[i as usize] = k;
                self.touched.push(i);
                return i as usize;
            }
            i = (i + 1) & self.mask;
        }
    }

    #[inline]
    pub fn add_delta(&mut self, key: u32, v: i64) {
        let s = self.slot(key);
        self.dvals[s] += v;
    }

    #[inline]
    #[allow(clippy::too_many_arguments)]
    pub fn fill(&mut self, symbol: u16, maker_broker: u16, taker_broker: u16, taker_side: u8, qty: u32, price: u64, maker_id: u64, taker_id: u64) {
        let mut b = [0u8; FILL_SIZE];
        b[0..2].copy_from_slice(&symbol.to_le_bytes());
        b[2..4].copy_from_slice(&maker_broker.to_le_bytes());
        b[4..6].copy_from_slice(&taker_broker.to_le_bytes());
        b[6] = taker_side;
        b[8..12].copy_from_slice(&qty.to_le_bytes());
        b[16..24].copy_from_slice(&price.to_le_bytes());
        b[24..32].copy_from_slice(&maker_id.to_le_bytes());
        b[32..40].copy_from_slice(&taker_id.to_le_bytes());
        self.fills.extend_from_slice(&b);
        let (buyer, seller) = if taker_side == BUY { (taker_broker, maker_broker) } else { (maker_broker, taker_broker) };
        let cash = qty as i64 * price as i64;
        self.add_delta((buyer as u32) << 16 | symbol as u32, qty as i64);
        self.add_delta((buyer as u32) << 16 | CASH_ASSET, -cash);
        self.add_delta((seller as u32) << 16 | symbol as u32, -(qty as i64));
        self.add_delta((seller as u32) << 16 | CASH_ASSET, cash);
    }

    pub fn num_fills(&self) -> usize {
        self.fills.len() / FILL_SIZE
    }

    /// Appends another accumulator's fills and sums its deltas into self.
    pub fn merge(&mut self, p: &Out) {
        self.fills.extend_from_slice(&p.fills);
        for &i in &p.touched {
            self.add_delta(p.dkeys[i as usize] - 1, p.dvals[i as usize]);
        }
    }

    /// Serializes the block output: fills ordered by symbol then execution order, deltas sorted.
    pub fn bytes(&mut self) -> &[u8] {
        let n = self.fills.len() / FILL_SIZE;
        self.idx.clear();
        for i in 0..n {
            let sym = u16le(&self.fills, i * FILL_SIZE) as u64;
            self.idx.push(sym << 32 | i as u64);
        }
        self.idx.sort_unstable();
        self.sorted.clear();
        for &k in &self.idx {
            let i = k as u32 as usize;
            self.sorted.extend_from_slice(&self.fills[i * FILL_SIZE..(i + 1) * FILL_SIZE]);
        }
        std::mem::swap(&mut self.fills, &mut self.sorted);
        for &i in &self.touched {
            if self.dvals[i as usize] != 0 {
                self.keys.push(self.dkeys[i as usize] - 1);
            }
        }
        self.keys.sort_unstable();
        self.buf.clear();
        self.buf.extend_from_slice(&(n as u32).to_le_bytes());
        self.buf.extend_from_slice(&(self.keys.len() as u32).to_le_bytes());
        self.buf.extend_from_slice(&self.fills);
        for ki in 0..self.keys.len() {
            let k = self.keys[ki];
            let s = self.slot(k);
            let v = self.dvals[s];
            let mut d = [0u8; DELTA_SIZE];
            d[0..2].copy_from_slice(&((k >> 16) as u16).to_le_bytes());
            d[2..4].copy_from_slice(&(k as u16).to_le_bytes());
            d[8..16].copy_from_slice(&v.to_le_bytes());
            self.buf.extend_from_slice(&d);
        }
        &self.buf
    }
}

pub fn fnv1a(b: &[u8]) -> u64 {
    let mut h: u64 = 14695981039346656037;
    for &c in b {
        h ^= c as u64;
        h = h.wrapping_mul(1099511628211);
    }
    h
}

/// Output length from its 8-byte header.
pub fn out_len(hdr: &[u8]) -> usize {
    8 + u32le(hdr, 0) as usize * FILL_SIZE + u32le(hdr, 4) as usize * DELTA_SIZE
}
