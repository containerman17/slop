(module (memory 1 1) (func $recur (param i32) (result i32) local.get 0 i32.const 1 i32.add call $recur) (func (export "call") (result i32) i32.const 0 call $recur))
