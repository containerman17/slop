//! ABI adapter for the existing matcher. All state stays in linear memory.
use matcher::Engine;
struct State {
    engine: matcher::native::Engine,
    input: Vec<u8>,
}
static mut STATE: Option<State> = None;
#[link(wasm_import_module = "env")]
extern "C" {
    fn input_len() -> u32;
    fn input(ptr: u32);
    fn output(ptr: u32, len: u32);
}
#[no_mangle]
pub extern "C" fn call() -> u32 {
    unsafe {
        let state = &mut *(&raw mut STATE);
        let s = state.get_or_insert_with(|| State {
            engine: matcher::native::Engine::new(),
            input: Vec::new(),
        });
        s.input.resize(input_len() as usize, 0);
        input(s.input.as_mut_ptr() as u32);
        let out = s.engine.apply(&s.input);
        output(out.as_ptr() as u32, out.len() as u32);
    }
    0
}
