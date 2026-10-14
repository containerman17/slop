(module
 (import "env" "input" (func $input (param i32)))
 (memory (export "memory") 1 1)
 (data (i32.const 256) "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!?")
 (func (export "call") (result i32)
  i32.const 0 call $input
  i32.const 0 i32.load
  if
   i32.const 4 i32.load i32.const 8 i32.load i32.const 12 i32.load memory.fill
  else
   i32.const 4 i32.load i32.const 8 i32.load i32.const 12 i32.load memory.copy
  end
  i32.const 20 i32.load if unreachable end
  i32.const 16 i32.load))
