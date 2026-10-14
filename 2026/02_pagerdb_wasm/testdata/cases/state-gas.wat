(module
 (import "env" "input" (func $input (param i32)))
 (memory 1 1)
 (func (export "call") (result i32)
  i32.const 0 call $input
  i32.const 64 i32.const 7 i32.store8
  i32.const 0 i32.load i32.const 9 i32.store8
  i32.const 0 i32.load i32.const 9 i32.store8
  i32.const 0))
