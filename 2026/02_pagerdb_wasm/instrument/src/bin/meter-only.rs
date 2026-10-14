// Diagnostic control only: the same gas and stack passes, without the write barrier.
use wasm_instrument::{
    gas_metering::{self, mutable_global, ConstantCostRules},
    parity_wasm::elements::*,
};
fn main() {
    let a: Vec<String> = std::env::args().collect();
    let mut m: Module = deserialize_buffer(&std::fs::read(&a[1]).unwrap()).unwrap();
    m.sections_mut()
        .retain(|s| !matches!(s, Section::Custom(_) | Section::Name(_) | Section::Reloc(_)));
    let cost = a[3].parse().unwrap();
    m = gas_metering::inject(
        m,
        mutable_global::Injector::new("__gas"),
        &ConstantCostRules::new(cost, 0, cost),
    )
    .unwrap();
    m = wasm_instrument::inject_stack_limiter(m, a[4].parse().unwrap()).unwrap();
    std::fs::write(&a[2], serialize(m).unwrap()).unwrap();
}
