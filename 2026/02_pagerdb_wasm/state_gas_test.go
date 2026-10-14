package pagerwasm

import (
	"context"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestStateGasCounts(t *testing.T) {
	_, h := testHost(t)
	p := deployCase(t, h, "state-gas")
	run := func(dst uint32) Result {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, dst)
		r := p.Run(context.Background(), Call{Input: b, Gas: 100000})
		if r.Status != 0 {
			t.Fatal(r.Status)
		}
		return r
	}
	// Input and the first store share leaf zero. Repeated destination stores are free.
	one := run(64)
	same := run(128)
	other := run(1024)
	if !reflect.DeepEqual(one.DirtyLines, []uint32{0, 1}) || one.BytesWritten != 128 || one.LeavesTouched != 1 {
		t.Fatalf("one: %+v", one)
	}
	if !reflect.DeepEqual(same.DirtyLines, []uint32{0, 1, 2}) || same.BytesWritten != 192 || same.LeavesTouched != 1 {
		t.Fatalf("same: %+v", same)
	}
	if !reflect.DeepEqual(other.DirtyLines, []uint32{0, 1, 16}) || other.BytesWritten != 192 || other.LeavesTouched != 2 {
		t.Fatalf("other: %+v", other)
	}
	if one.GasLeft-same.GasLeft != lineSize*ByteGas+HostGas || same.GasLeft-other.GasLeft != LeafGas {
		t.Fatal("state gas does not match unique bytes and leaves")
	}
}
