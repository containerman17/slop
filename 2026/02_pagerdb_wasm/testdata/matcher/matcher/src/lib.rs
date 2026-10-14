//! Matcher engines shared by the host harness and the wasm modules. No I/O, no threads.
pub mod native;
pub mod spec;

pub trait Engine {
    /// Copies the block into the engine's own memory, applies it and returns the serialized output.
    fn apply(&mut self, block: &[u8]) -> &[u8];
    fn live(&mut self) -> usize;
    /// Called after the lock is taken and before the first block (parallel engines start workers here).
    fn start(&mut self) {}
    /// Called after the last block and before the lock is released (workers stop spinning here).
    fn stop(&mut self) {}
}
