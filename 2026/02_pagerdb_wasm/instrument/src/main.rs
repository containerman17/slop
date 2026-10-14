// Deploy pipeline reuses wasm-instrument 0.4.0 gas metering and stack limiting.
use wasm_instrument::parity_wasm::elements::*;
use Instruction::*;

fn add_section(m: &mut Module, s: Section) {
    m.insert_section(s).unwrap();
}
fn leaf_barrier(out: &mut Vec<Instruction>, leaf: u32, bitmap: u32, touch: u32) {
    // The bitmap itself is private instrumentation state. Only the host sets bits.
    out.extend([
        I32Const(bitmap as i32),
        GetLocal(leaf),
        I32Const(3),
        I32ShrU,
        I32Add,
        I32Load8U(0, 0),
        I32Const(1),
        GetLocal(leaf),
        I32Const(7),
        I32And,
        I32Shl,
        I32And,
        I32Eqz,
        If(BlockType::NoResult),
        GetLocal(leaf),
        Call(touch),
        End,
    ]);
}
fn instrument(data: &[u8], instruction_cost: u32, stack_limit: u32) -> (Vec<u8>, String) {
    // MVP plus sign extension; floats, SIMD, threads, reference mutation and bulk memory fail closed.
    let mut features = wasmparser::WasmFeatures::WASM1;
    features.set(wasmparser::WasmFeatures::FLOATS, false);
    features.set(wasmparser::WasmFeatures::SIGN_EXTENSION, true);
    wasmparser::Validator::new_with_features(features)
        .validate_all(data)
        .expect("unsupported Wasm feature");
    let mut m: Module = deserialize_buffer(data).expect("parse module");
    m.sections_mut()
        .retain(|s| !matches!(s, Section::Custom(_) | Section::Name(_) | Section::Reloc(_)));
    assert!(m.start_section().is_none(), "start function forbidden");
    for e in m.export_section().into_iter().flat_map(|s| s.entries()) {
        assert!(!e.field().starts_with("__"), "reserved export");
    }
    let mut nimp = 0;
    for im in m.import_section().into_iter().flat_map(|s| s.entries()) {
        assert_eq!(im.module(), "env", "unknown import module");
        let ty = match im.external() {
            External::Function(i) => *i,
            _ => panic!("only function imports allowed"),
        };
        let (args, results): (Vec<ValueType>, Vec<ValueType>) = match im.field() {
            "block_time" | "block_number" | "gas_left" => (vec![], vec![ValueType::I64]),
            "caller" | "value" | "input" => (vec![ValueType::I32], vec![]),
            "call_type" | "input_len" => (vec![], vec![ValueType::I32]),
            "emit_log" | "output" => (vec![ValueType::I32, ValueType::I32], vec![]),
            _ => panic!("unknown or reserved import"),
        };
        let Type::Function(ft) = &m.type_section().unwrap().types()[ty as usize];
        assert_eq!(ft.params(), args);
        assert_eq!(ft.results(), results);
        nimp += 1;
    }
    let mems = m.memory_section().expect("memory required").entries();
    assert_eq!(mems.len(), 1);
    let pages = mems[0]
        .limits()
        .maximum()
        .expect("fixed maximum memory required");
    assert!(pages > 0 && pages <= 8192, "maximum memory exceeds 512 MiB");
    let limit = pages * 65536;
    let mut metadata = format!("{}\n", limit);
    for d in m.data_section().into_iter().flat_map(|s| s.entries()) {
        let ins = d.offset().as_ref().expect("active data only").code();
        let off = match ins {
            [I32Const(x), End] => *x as u32 as u64,
            _ => panic!("constant data offset required"),
        };
        assert!(
            off + d.value().len() as u64 <= limit as u64,
            "data overlaps bitmap"
        );
        metadata.push_str(&format!("{} {}\n", off, d.value().len()));
    }
    // One extra Wasm leaf holds up to 16 KiB of bits, outside the guest's usable range.
    m.memory_section_mut().unwrap().entries_mut()[0] = MemoryType::new(pages + 1, Some(pages + 1));
    // Append touch to imports and shift every reference to a defined function.
    for b in m
        .code_section_mut()
        .into_iter()
        .flat_map(|s| s.bodies_mut())
    {
        for op in b.code_mut().elements_mut() {
            if let Call(i) = op {
                if *i >= nimp {
                    *i += 1;
                }
            }
        }
    }
    for e in m
        .export_section_mut()
        .into_iter()
        .flat_map(|s| s.entries_mut())
    {
        if let Internal::Function(i) = e.internal_mut() {
            if *i >= nimp {
                *i += 1;
            }
        }
    }
    for e in m
        .elements_section_mut()
        .into_iter()
        .flat_map(|s| s.entries_mut())
    {
        for i in e.members_mut() {
            if *i >= nimp {
                *i += 1;
            }
        }
    }
    let nt = m.type_section().unwrap().types().len() as u32;
    m.type_section_mut()
        .unwrap()
        .types_mut()
        .push(Type::Function(FunctionType::new(
            vec![ValueType::I32],
            vec![],
        )));
    if m.import_section().is_none() {
        add_section(&mut m, Section::Import(ImportSection::default()));
    }
    m.import_section_mut()
        .unwrap()
        .entries_mut()
        .push(ImportEntry::new(
            "env".into(),
            "touch".into(),
            External::Function(nt),
        ));
    if m.export_section().is_none() {
        add_section(&mut m, Section::Export(ExportSection::default()));
    }
    m.export_section_mut()
        .unwrap()
        .entries_mut()
        .push(ExportEntry::new("__memory".into(), Internal::Memory(0)));
    // 1 gas per original instruction and local, using the existing metered-block algorithm.
    m = rust_instrument::meter(m, instruction_cost);
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
    for (idx, body) in m
        .code_section_mut()
        .unwrap()
        .bodies_mut()
        .iter_mut()
        .enumerate()
    {
        let base = params[idx] + body.locals().iter().map(|l| l.count()).sum::<u32>();
        let (addr, v32, leaf, last, v64, eff) =
            (base, base + 1, base + 2, base + 3, base + 4, base + 5);
        body.locals_mut()
            .extend([Local::new(4, ValueType::I32), Local::new(2, ValueType::I64)]);
        let mut out = Vec::new();
        for op in body.code().elements() {
            let store = match *op {
                I32Store(_, o) => Some((o, 4, false)),
                I64Store(_, o) => Some((o, 8, true)),
                I32Store8(_, o) => Some((o, 1, false)),
                I32Store16(_, o) => Some((o, 2, false)),
                I64Store8(_, o) => Some((o, 1, true)),
                I64Store16(_, o) => Some((o, 2, true)),
                I64Store32(_, o) => Some((o, 4, true)),
                _ => None,
            };
            if let Some((offset, width, is64)) = store {
                let val = if is64 { v64 } else { v32 };
                out.extend([
                    SetLocal(val),
                    TeeLocal(addr),
                    I64ExtendUI32,
                    I64Const(offset as i64),
                    I64Add,
                    TeeLocal(eff),
                    I64Const(width),
                    I64Add,
                    I64Const(limit as i64),
                    I64GtU,
                    If(BlockType::NoResult),
                    Unreachable,
                    End,
                    GetLocal(eff),
                    I64Const(10),
                    I64ShrU,
                    I32WrapI64,
                    SetLocal(leaf),
                ]);
                leaf_barrier(&mut out, leaf, limit, nimp);
                if width > 1 {
                    out.extend([
                        GetLocal(eff),
                        I64Const(width - 1),
                        I64Add,
                        I64Const(10),
                        I64ShrU,
                        I32WrapI64,
                        TeeLocal(last),
                        GetLocal(leaf),
                        I32Ne,
                        If(BlockType::NoResult),
                    ]);
                    leaf_barrier(&mut out, last, limit, nimp);
                    out.push(End);
                }
                out.extend([GetLocal(addr), GetLocal(val), op.clone()]);
            } else {
                match op {
                    CurrentMemory(_) => out.push(I32Const(pages as i32)),
                    GrowMemory(_) => out.extend([Drop, I32Const(-1)]),
                    _ => out.push(op.clone()),
                }
            }
        }
        *body.code_mut() = Instructions::new(out);
    }
    m = wasm_instrument::inject_stack_limiter(m, stack_limit).expect("stack injection");
    // Exports let the host restore guest globals and the injected stack counter after a trap.
    wasm_instrument::export_mutable_globals(&mut m, "__global");
    let out = serialize(m).unwrap();
    wasmparser::Validator::new_with_features(features)
        .validate_all(&out)
        .expect("validate instrumented module");
    (out, metadata)
}
fn main() {
    let args: Vec<String> = std::env::args().collect();
    let bytes = std::fs::read(&args[1]).unwrap();
    let (out, metadata) = instrument(&bytes, args[3].parse().unwrap(), args[4].parse().unwrap());
    std::fs::write(&args[2], out).unwrap();
    std::fs::write(format!("{}.meta", args[2]), metadata).unwrap();
}
