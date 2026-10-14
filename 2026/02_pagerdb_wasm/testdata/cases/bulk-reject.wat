(module (memory 1 1) (data $d "x")
 (func (export "call") (result i32)
  i32.const 0 i32.const 0 i32.const 1 memory.init $d
  data.drop $d i32.const 0))
