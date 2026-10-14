// Diagnostic post-pass only. These controls do not provide undo or safe persistence.
use wasm_instrument::parity_wasm::elements::*;
use Instruction::*;
fn main() {
    let a: Vec<String> = std::env::args().collect();
    let mut m: Module = deserialize_buffer(&std::fs::read(&a[1]).unwrap()).unwrap();
    if a[3] == "metered" {
        wasm_instrument::export_mutable_globals(&mut m, "__global");
    } else {
        let touch = m
            .import_section()
            .unwrap()
            .entries()
            .iter()
            .position(|i| i.field() == "touch")
            .unwrap() as u32;
        let bitmap = (m.memory_section().unwrap().entries()[0]
            .limits()
            .maximum()
            .unwrap()
            - 16)
            * 65536;
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
            let line = params[i] + body.locals().iter().map(|l| l.count()).sum::<u32>();
            body.locals_mut().push(Local::new(1, ValueType::I32));
            let mut out = Vec::new();
            for op in body.code().elements() {
                if *op == Call(touch) {
                    // Preserve first-write behavior when the host does not set the bit.
                    out.extend([
                        SetLocal(line),
                        I32Const(bitmap as i32),
                        GetLocal(line),
                        I32Const(3),
                        I32ShrU,
                        I32Add,
                        I32Const(bitmap as i32),
                        GetLocal(line),
                        I32Const(3),
                        I32ShrU,
                        I32Add,
                        I32Load8U(0, 0),
                        I32Const(1),
                        GetLocal(line),
                        I32Const(7),
                        I32And,
                        I32Shl,
                        I32Or,
                        I32Store8(0, 0),
                    ]);
                    if a[3] == "empty" {
                        out.extend([GetLocal(line), Call(touch)]);
                    }
                } else {
                    out.push(op.clone());
                }
            }
            *body.code_mut() = Instructions::new(out);
        }
    }
    std::fs::write(&a[2], serialize(m).unwrap()).unwrap();
}
