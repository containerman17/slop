(module (import "wasi_snapshot_preview1" "clock_time_get" (func (param i32 i64 i32) (result i32))) (memory 1 1) (func (export "call") (result i32) i32.const 0))
