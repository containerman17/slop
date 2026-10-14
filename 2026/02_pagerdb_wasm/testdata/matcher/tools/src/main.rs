use matcher::{spec, Engine};
use std::{
    collections::{BTreeMap, HashMap, HashSet},
    io::Read,
};
fn quantile(h: &BTreeMap<u64, u64>, q: u64) -> u64 {
    let total: u64 = h.values().sum();
    let target = (total * q + 99) / 100;
    let mut n = 0;
    for (v, c) in h {
        n += c;
        if n >= target {
            return *v;
        }
    }
    0
}
fn hist(name: &str, h: &BTreeMap<u64, u64>) {
    let n: u64 = h.values().sum();
    let sum: f64 = h.iter().map(|(v, c)| *v as f64 * *c as f64).sum();
    println!(
        "{name}: n={n} mean={:.3} p50={} p90={} p99={}",
        sum / n.max(1) as f64,
        quantile(h, 50),
        quantile(h, 90),
        quantile(h, 99)
    );
}
fn main() {
    let a: Vec<String> = std::env::args().collect();
    let shape = a[1] == "shape";
    let mut f = std::fs::File::open(&a[2]).unwrap();
    let mut header = [0; 16];
    f.read_exact(&mut header).unwrap();
    assert_eq!(&header[..5], b"CLOB1");
    let per = u32::from_le_bytes(header[8..12].try_into().unwrap()) as usize;
    let blocks = u32::from_le_bytes(header[12..16].try_into().unwrap()) as usize;
    let mut block = vec![0; per * 40];
    let mut engine = matcher::native::Engine::new();
    let mut ops = [0u64; 5];
    let mut sides = [0u64; 3];
    let mut symbols = vec![0u64; 65536];
    let mut prices: Vec<HashSet<u64>> = (0..65536).map(|_| HashSet::new()).collect();
    let mut qty = BTreeMap::new();
    let mut distance = BTreeMap::new();
    let mut levels = BTreeMap::new();
    let mut delays = BTreeMap::new();
    let mut added = HashMap::new();
    let mut cross = 0u64;
    let mut seq = 0u64;
    let mut out = spec::Out::new();
    for i in 0..blocks {
        f.read_exact(&mut block).unwrap();
        if !shape {
            let output = engine.apply(&block);
            let mut h = 14695981039346656037u64;
            for x in output {
                h ^= *x as u64;
                h = h.wrapping_mul(1099511628211);
            }
            println!("{i} {h:016x}");
            continue;
        }
        for b in block.chunks_exact(40) {
            let m = spec::decode(b);
            seq += 1;
            ops[m.op as usize] += 1;
            sides[m.side as usize] += 1;
            symbols[m.symbol as usize] += 1;
            let book = engine.book(m.symbol);
            let (bid, ask, depth) = book.shape_quote();
            *levels.entry(depth as u64).or_insert(0) += 1;
            if m.op == 1 || m.op == 3 || m.op == 4 {
                prices[m.symbol as usize].insert(m.price);
                *qty.entry(m.qty as u64).or_insert(0) += 1;
                if let (Some(b), Some(a)) = (bid, ask) {
                    let mid = (b as u128 + a as u128) / 2;
                    if mid > 0 {
                        let d = (m.price as u128).abs_diff(mid) * 10000 / mid;
                        *distance.entry(d as u64).or_insert(0) += 1;
                    }
                }
            }
            let key = (m.symbol, m.order_id);
            if m.op == 1 {
                added.insert(key, seq);
            } else if m.op == 2 {
                if let Some(start) = added.remove(&key) {
                    *delays.entry(seq - start).or_insert(0) += 1;
                }
            } else if m.op == 3 {
                added.remove(&key);
                added.insert((m.symbol, m.new_id), seq);
            }
            out.reset();
            book.step(&mut out, &m);
            if m.op == 1 && u32::from_le_bytes(out.bytes()[..4].try_into().unwrap()) > 0 {
                cross += 1;
            }
        }
    }
    if shape {
        println!("messages={seq} blocks={blocks} ops={ops:?} sides={sides:?}");
        let mut frequencies: Vec<u64> = symbols.into_iter().filter(|x| *x > 0).collect();
        frequencies.sort_unstable_by(|a, b| b.cmp(a));
        println!(
            "symbols={} top1_pct={:.6} top10_pct={:.6} top100_pct={:.6}",
            frequencies.len(),
            frequencies[0] as f64 * 100. / seq as f64,
            frequencies.iter().take(10).sum::<u64>() as f64 * 100. / seq as f64,
            frequencies.iter().take(100).sum::<u64>() as f64 * 100. / seq as f64
        );
        let mut unique = BTreeMap::new();
        for p in prices {
            if !p.is_empty() {
                *unique.entry(p.len() as u64).or_insert(0) += 1;
            }
        }
        hist("distinct price levels per symbol", &unique);
        hist("active levels at message", &levels);
        hist(
            "absolute distance from pre-message midpoint bps (priced messages with two quotes)",
            &distance,
        );
        hist("qty (priced messages)", &qty);
        hist(
            "cancel delay in messages since observed Add/Replace",
            &delays,
        );
        println!(
            "crossing Adds={cross} crossing_Add_pct={:.6}",
            cross as f64 * 100. / ops[1] as f64
        );
        println!(
            "qty buckets <=1,<=10,<=100,<=1000,>1000: {:?}",
            [
                qty.range(..=1).map(|(_, n)| *n).sum::<u64>(),
                qty.range(2..=10).map(|(_, n)| *n).sum(),
                qty.range(11..=100).map(|(_, n)| *n).sum(),
                qty.range(101..=1000).map(|(_, n)| *n).sum(),
                qty.range(1001..).map(|(_, n)| *n).sum()
            ]
        );
    }
}
