# Local bulk stack-effect patch

Source: wasm-instrument 0.4.0 from the crates.io package, with its original licenses.
The source is unchanged except `src/stack_limiter/max_height.rs`: memory.copy and
memory.fill consume three stack operands; other bulk instructions return an error.
Cargo.toml adds `bulk = ["parity-wasm/bulk"]`. The parent deploy validator admits
only copy/fill. This avoids replacing the existing gas and stack algorithms.
