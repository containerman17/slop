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

// Dynamic bulk cost is separate from the existing static instruction meter.
pub fn meter_bulk(mut m: Module, cost: u32) -> Module {
    use Instruction::*;
    let gas = m
        .export_section()
        .unwrap()
        .entries()
        .iter()
        .find_map(|e| {
            if e.field() == "__gas" {
                if let Internal::Global(g) = e.internal() {
                    return Some(*g);
                }
            }
            None
        })
        .expect("gas global");
    let params: Vec<u32> = m
        .function_section()
        .unwrap()
        .entries()
        .iter()
        .map(|f| {
            let Type::Function(t) = &m.type_section().unwrap().types()[f.type_ref() as usize];
            t.params().len() as u32
        })
        .collect();
    for (i, body) in m
        .code_section_mut()
        .unwrap()
        .bodies_mut()
        .iter_mut()
        .enumerate()
    {
        if !body.code().elements().iter().any(|op| {
            matches!(
                op,
                Bulk(BulkInstruction::MemoryCopy | BulkInstruction::MemoryFill)
            )
        }) {
            continue;
        }
        let len = params[i] + body.locals().iter().map(|l| l.count()).sum::<u32>();
        body.locals_mut().push(Local::new(1, ValueType::I32));
        let mut out = Vec::new();
        for op in body.code().elements() {
            if matches!(
                op,
                Bulk(BulkInstruction::MemoryCopy | BulkInstruction::MemoryFill)
            ) {
                out.extend([
                    TeeLocal(len),
                    GetGlobal(gas),
                    GetLocal(len),
                    I64ExtendUI32,
                    I64Const(cost as i64),
                    I64Mul,
                    I64GeU,
                    If(BlockType::NoResult),
                    GetGlobal(gas),
                    GetLocal(len),
                    I64ExtendUI32,
                    I64Const(cost as i64),
                    I64Mul,
                    I64Sub,
                    SetGlobal(gas),
                    Else,
                    I64Const(-1),
                    SetGlobal(gas),
                    Unreachable,
                    End,
                ]);
            }
            out.push(op.clone());
        }
        *body.code_mut() = Instructions::new(out);
    }
    m
}
