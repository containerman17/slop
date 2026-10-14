(module
 (import "env" "block_time" (func $time (result i64)))
 (import "env" "block_number" (func $number (result i64)))
 (import "env" "gas_left" (func $gas (result i64)))
 (import "env" "caller" (func $caller (param i32)))
 (import "env" "value" (func $value (param i32)))
 (import "env" "call_type" (func $type (result i32)))
 (import "env" "input_len" (func $len (result i32)))
 (import "env" "input" (func $input (param i32)))
 (import "env" "output" (func $output (param i32 i32)))
 (import "env" "emit_log" (func $log (param i32 i32)))
 (memory 1 1)
 (func (export "call") (result i32)
  i32.const 0 call $caller
  i32.const 20 call $value
  i32.const 56 call $time i64.store
  i32.const 64 call $number i64.store
  i32.const 72 call $type i32.store
  i32.const 76 call $len i32.store
  i32.const 80 call $gas i64.store
  i32.const 96 call $input
  i32.const 0 i32.const 52 call $log
  i32.const 0 i32.const 100 call $output
  i32.const 0))
