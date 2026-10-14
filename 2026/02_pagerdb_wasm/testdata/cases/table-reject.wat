(module (memory 1 1) (table 1 funcref)
 (func (export "call") (result i32)
  i32.const 0 i32.const 0 i32.const 1 table.copy i32.const 0))
