(module
 (type $f (func (param i32) (result i32)))
 (memory 1 1)
 (table 1 1 funcref)
 (elem (i32.const 0) $step)
 (func $step (type $f) (param i32) (result i32)
   local.get 0 i32.const 1 i32.add)
 (func (export "call") (result i32) (local $n i32)
   loop $again
     local.get $n i32.const 0 call_indirect (type $f) local.tee $n
     i32.const 5 i32.lt_u br_if $again
   end
   local.get $n i32.const 5 i32.eq
   if (result i32) i32.const 0 else i32.const 1 end))
