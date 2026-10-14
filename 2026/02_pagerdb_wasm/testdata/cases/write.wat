(module
 (import "env" "input" (func $input (param i32)))
 (import "env" "output" (func $output (param i32 i32)))
 (import "env" "emit_log" (func $log (param i32 i32)))
 (memory (export "memory") 1 1)
 (func (export "call") (result i32)
  i32.const 0 call $input
  i32.const 4094 i64.const 123456 i64.store
  i32.const 12300 i32.const 99 i32.store8
  i32.const 4094 i32.const 8 call $output
  i32.const 4094 i32.const 8 call $log
  i32.const 0 i32.load8_u i32.const 2 i32.eq
  if unreachable end
  i32.const 0 i32.load8_u i32.const 3 i32.eq
  if loop br 0 end end
  i32.const 0 i32.load8_u))
