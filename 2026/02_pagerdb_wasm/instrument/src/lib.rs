use wasm_instrument::{
    gas_metering::{self, mutable_global, ConstantCostRules},
    parity_wasm::elements::*,
};

// Keep the existing metered-block algorithm and gas charges, but inline its helper.
pub fn meter(m: Module, cost: u32) -> Module {
    use Instruction::*;
    let helper = m.functions_space() as u32;
    let global = m.globals_space() as u32;
    let mut m = gas_metering::inject(
        m,
        mutable_global::Injector::new("__gas"),
        &ConstantCostRules::new(cost, 0, cost),
    )
    .expect("gas injection");
    let bodies = m.code_section_mut().unwrap().bodies_mut();
    bodies.pop().expect("gas helper");
    for body in bodies {
        let old = std::mem::take(body.code_mut().elements_mut());
        let mut out = Vec::new();
        for op in old {
            if op == Call(helper) {
                let Some(I64Const(charge)) = out.pop() else {
                    panic!("gas charge must be constant")
                };
                out.extend([
                    GetGlobal(global),
                    I64Const(charge),
                    I64GeU,
                    If(BlockType::NoResult),
                    GetGlobal(global),
                    I64Const(charge),
                    I64Sub,
                    SetGlobal(global),
                    Else,
                    I64Const(-1),
                    SetGlobal(global),
                    Unreachable,
                    End,
                ]);
            } else {
                out.push(op);
            }
        }
        *body.code_mut().elements_mut() = out;
    }
    m.function_section_mut()
        .unwrap()
        .entries_mut()
        .pop()
        .expect("gas helper signature");
    m
}
