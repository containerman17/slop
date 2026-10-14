(module
 (import "env" "input" (func $input (param i32)))
 (memory 1 1)
 (global $g (mut i32) (i32.const 7))
 (func (export "call") (result i32)
  i32.const 0 call $input
  global.get $g i32.const 1 i32.add global.set $g
  i32.const 0 i32.load8_u))
