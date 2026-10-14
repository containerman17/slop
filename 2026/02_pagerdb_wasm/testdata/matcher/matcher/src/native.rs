//! Reference matcher in plain structs: one book per symbol, slab orders, sorted price
//! arrays (best at the end), open-addressing order index. Same algorithm as bench/go/native.
use crate::spec::{self, Msg, Out};

#[derive(Clone, Copy, Default)]
struct Order {
    price: u64,
    id: u64,
    qty: u32,
    next: i32,
    prev: i32,
    broker: u16,
    side: u8,
}

/// A price level, stored inline in the side's sorted array (no separate level slab).
#[derive(Clone, Copy, Default)]
struct Level {
    price: u64,
    head: i32,
    tail: i32,
}

#[derive(Default)]
struct Side {
    levels: Vec<Level>, // sorted by price, best at the end (asks descending, bids ascending)
}

impl Side {
    /// Position of price or its insertion point.
    #[inline]
    fn find(&self, price: u64, is_bid: bool) -> (usize, bool) {
        let (mut lo, mut hi) = (0usize, self.levels.len());
        while lo < hi {
            let mid = (lo + hi) >> 1;
            let p = self.levels[mid].price;
            if p == price {
                return (mid, true);
            }
            let less = if is_bid { p < price } else { p > price };
            if less {
                lo = mid + 1;
            } else {
                hi = mid;
            }
        }
        (lo, false)
    }
}

/// Open-addressing u64 -> i32 with linear probing and backward-shift deletion. Keys are never 0.
/// Key and value share one 16-byte entry so a lookup costs one cache miss.
#[derive(Clone, Copy, Default)]
struct Ent {
    key: u64,
    val: i32,
}
struct Table {
    ents: Vec<Ent>,
    mask: u64,
    n: usize,
}

#[inline(always)]
fn mix(mut k: u64) -> u64 {
    k ^= k >> 32;
    k = k.wrapping_mul(0x9E3779B97F4A7C15);
    k ^ (k >> 29)
}

impl Table {
    fn new(cap: usize) -> Self {
        Table { ents: vec![Ent::default(); cap], mask: (cap - 1) as u64, n: 0 }
    }
    #[inline]
    fn get(&self, k: u64) -> Option<i32> {
        let mut i = mix(k) & self.mask;
        loop {
            let e = &self.ents[i as usize];
            if e.key == k {
                return Some(e.val);
            }
            if e.key == 0 {
                return None;
            }
            i = (i + 1) & self.mask;
        }
    }
    #[inline]
    fn put(&mut self, k: u64, v: i32) {
        if self.n * 4 >= self.ents.len() * 3 {
            self.grow();
        }
        let mut i = mix(k) & self.mask;
        loop {
            let e = &mut self.ents[i as usize];
            if e.key == k {
                e.val = v;
                return;
            }
            if e.key == 0 {
                *e = Ent { key: k, val: v };
                self.n += 1;
                return;
            }
            i = (i + 1) & self.mask;
        }
    }
    #[inline]
    fn del(&mut self, k: u64) {
        let mut i = mix(k) & self.mask;
        loop {
            let kk = self.ents[i as usize].key;
            if kk == 0 {
                return;
            }
            if kk == k {
                break;
            }
            i = (i + 1) & self.mask;
        }
        let mut j = i;
        loop {
            j = (j + 1) & self.mask;
            let ej = self.ents[j as usize];
            if ej.key == 0 {
                break;
            }
            let h = mix(ej.key) & self.mask;
            if (i <= j && (h <= i || h > j)) || (i > j && (h <= i && h > j)) {
                self.ents[i as usize] = ej;
                i = j;
            }
        }
        self.ents[i as usize] = Ent::default();
        self.n -= 1;
    }
    fn grow(&mut self) {
        let mut t = Table::new(self.ents.len() * 2);
        for e in &self.ents {
            if e.key != 0 {
                t.put(e.key, e.val);
            }
        }
        *self = t;
    }
}

/// One symbol's complete state.
pub struct Book {
    bid: Side,
    ask: Side,
    orders: Vec<Order>,
    free: Vec<i32>,
    index: Table,
}

impl Default for Book {
    fn default() -> Self {
        Self::new()
    }
}

impl Book {
    pub fn new() -> Self {
        Book {
            bid: Side::default(),
            ask: Side::default(),
            orders: Vec::with_capacity(256),
            free: Vec::new(),
            index: Table::new(512),
        }
    }

    #[inline]
    fn alloc_order(&mut self) -> i32 {
        if let Some(i) = self.free.pop() {
            return i;
        }
        self.orders.push(Order::default());
        (self.orders.len() - 1) as i32
    }

    fn rest(&mut self, o: i32) {
        let ord = self.orders[o as usize];
        let is_bid = ord.side == spec::BUY;
        let s = if is_bid { &mut self.bid } else { &mut self.ask };
        let (pos, found) = s.find(ord.price, is_bid);
        if !found {
            s.levels.insert(pos, Level { price: ord.price, head: -1, tail: -1 });
        }
        let lv = &mut s.levels[pos];
        let tail = lv.tail;
        if tail >= 0 {
            self.orders[tail as usize].next = o;
        } else {
            lv.head = o;
        }
        lv.tail = o;
        let ord = &mut self.orders[o as usize];
        ord.next = -1;
        ord.prev = tail;
        self.index.put(ord.id, o);
    }

    fn unlink(&mut self, o: i32) {
        let ord = self.orders[o as usize];
        let is_bid = ord.side == spec::BUY;
        let s = if is_bid { &mut self.bid } else { &mut self.ask };
        let (pos, _) = s.find(ord.price, is_bid);
        let lv = &mut s.levels[pos];
        if ord.prev >= 0 {
            self.orders[ord.prev as usize].next = ord.next;
        } else {
            lv.head = ord.next;
        }
        if ord.next >= 0 {
            self.orders[ord.next as usize].prev = ord.prev;
        } else {
            lv.tail = ord.prev;
        }
        if lv.head < 0 {
            s.levels.remove(pos);
        }
        self.index.del(ord.id);
        self.free.push(o);
    }

    fn add(&mut self, out: &mut Out, m: &Msg, id: u64) {
        let mut qty = m.qty;
        let is_buy = m.side == spec::BUY;
        while qty > 0 {
            let opp = if is_buy { &self.ask } else { &self.bid };
            let Some(&lv) = opp.levels.last() else { break };
            if (is_buy && lv.price > m.price) || (!is_buy && lv.price < m.price) {
                break;
            }
            // levels in the array are never empty, so head is valid
            let h = lv.head;
            let r = &mut self.orders[h as usize];
            let f = qty.min(r.qty);
            out.fill(m.symbol, r.broker, m.broker, m.side, f, r.price, r.id, id);
            qty -= f;
            r.qty -= f;
            if r.qty == 0 {
                self.unlink(h);
            }
        }
        if qty == 0 {
            return;
        }
        let o = self.alloc_order();
        self.orders[o as usize] = Order { price: m.price, id, qty, broker: m.broker, side: m.side, ..Default::default() };
        self.rest(o);
    }

    /// Applies one message.
    #[inline]
    pub fn step(&mut self, out: &mut Out, m: &Msg) {
        match m.op {
            spec::OP_ADD => self.add(out, m, m.order_id),
            spec::OP_CANCEL => {
                if let Some(o) = self.index.get(m.order_id) {
                    self.unlink(o);
                }
            }
            spec::OP_REPLACE => {
                if let Some(o) = self.index.get(m.order_id) {
                    self.unlink(o);
                }
                self.add(out, m, m.new_id);
            }
            spec::OP_MODIFY => {
                let Some(o) = self.index.get(m.order_id) else { return };
                let ord = &mut self.orders[o as usize];
                if ord.price == m.price && m.qty <= ord.qty {
                    ord.qty = m.qty;
                    return;
                }
                self.unlink(o);
                self.add(out, m, m.order_id);
            }
            _ => {}
        }
    }

    pub fn live(&self) -> usize {
        self.index.n
    }
}

/// Single-threaded matcher over all symbols.
pub struct Engine {
    pub books: Vec<Option<Box<Book>>>,
    out: Out,
    input: Vec<u8>,
}

impl Default for Engine {
    fn default() -> Self {
        Self::new()
    }
}

impl Engine {
    pub fn new() -> Self {
        let mut books = Vec::with_capacity(1 << 16);
        books.resize_with(1 << 16, || None);
        Engine { books, out: Out::new(), input: Vec::with_capacity(50_000 * spec::MSG_SIZE) }
    }
    #[inline]
    pub fn book(&mut self, sym: u16) -> &mut Book {
        self.books[sym as usize].get_or_insert_with(|| Box::new(Book::new()))
    }
}

#[inline(always)]
pub fn prefetch<T>(p: *const T) {
    #[cfg(target_arch = "x86_64")]
    unsafe {
        core::arch::x86_64::_mm_prefetch(p as *const i8, core::arch::x86_64::_MM_HINT_T0);
    }
    #[cfg(not(target_arch = "x86_64"))]
    let _ = p;
}

impl Book {
    /// Prefetches the lines a message on this book will touch first: its index slot and the
    /// best opposite level's price and the level's head order.
    #[inline(always)]
    pub fn prefetch_for(&self, id: u64, side: u8) {
        let i = mix(id) & self.index.mask;
        prefetch(unsafe { self.index.ents.as_ptr().add(i as usize) });
        let opp = if side == spec::BUY { &self.ask } else { &self.bid };
        if !opp.levels.is_empty() {
            prefetch(unsafe { opp.levels.as_ptr().add(opp.levels.len() - 1) });
        }
    }
    /// Last stage: the index slot is cached now; prefetch the resting order it points to
    /// (cancel, modify, replace). Prefetching the head order for adds measured no gain.
    #[inline(always)]
    pub fn prefetch_order(&self, op: u8, id: u64) {
        if op != spec::OP_ADD {
            if let Some(o) = self.index.get(id) {
                prefetch(unsafe { self.orders.as_ptr().add(o as usize) });
            }
        }
    }
}

/// Look-ahead distances of the software prefetch pipeline in `apply`.
const LA_BOOK: usize = 16;
const LA_SLOT: usize = 8;
const LA_ORD: usize = 3;

impl crate::Engine for Engine {
    fn apply(&mut self, block: &[u8]) -> &[u8] {
        self.input.clear();
        self.input.extend_from_slice(block);
        self.out.reset();
        let n = self.input.len() / spec::MSG_SIZE;
        for i in 0..n {
            if i + LA_BOOK < n {
                let sym = spec::symbol_of(&self.input[(i + LA_BOOK) * spec::MSG_SIZE..]);
                if let Some(b) = &self.books[sym as usize] {
                    prefetch(&**b as *const Book);
                }
            }
            if i + LA_SLOT < n {
                let rec = &self.input[(i + LA_SLOT) * spec::MSG_SIZE..];
                let sym = spec::symbol_of(rec);
                if let Some(b) = &self.books[sym as usize] {
                    let id = u64::from_le_bytes(rec[24..32].try_into().unwrap());
                    b.prefetch_for(id, rec[1]);
                }
            }
            if i + LA_ORD < n {
                let rec = &self.input[(i + LA_ORD) * spec::MSG_SIZE..];
                let sym = spec::symbol_of(rec);
                if let Some(b) = &self.books[sym as usize] {
                    b.prefetch_order(rec[0], u64::from_le_bytes(rec[24..32].try_into().unwrap()));
                }
            }
            let m = spec::decode(&self.input[i * spec::MSG_SIZE..]);
            let b = self.books[m.symbol as usize].get_or_insert_with(|| Box::new(Book::new()));
            b.step(&mut self.out, &m);
        }
        self.out.bytes()
    }
    fn live(&mut self) -> usize {
        self.books.iter().flatten().map(|b| b.live()).sum()
    }
}

/// Harness floor: copy in, empty output.
pub struct Noop {
    input: Vec<u8>,
    out: [u8; 8],
}
impl Default for Noop {
    fn default() -> Self {
        Noop { input: Vec::with_capacity(50_000 * spec::MSG_SIZE), out: [0; 8] }
    }
}
impl crate::Engine for Noop {
    fn apply(&mut self, block: &[u8]) -> &[u8] {
        self.input.clear();
        self.input.extend_from_slice(block);
        &self.out
    }
    fn live(&mut self) -> usize {
        0
    }
}

// Native-only observation for aggregate fixture diagnostics. Matching is unchanged.
#[cfg(not(target_arch = "wasm32"))]
impl Book {
    pub fn shape_quote(&self) -> (Option<u64>, Option<u64>, usize) {
        (self.bid.levels.last().map(|x| x.price), self.ask.levels.last().map(|x| x.price), self.bid.levels.len()+self.ask.levels.len())
    }
}
