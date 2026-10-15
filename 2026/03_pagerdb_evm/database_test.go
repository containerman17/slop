package pagerevm

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/graft/coreth/core/extstate"
	"github.com/ava-labs/avalanchego/graft/coreth/plugin/evm/customtypes"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/rawdb"
	"github.com/ava-labs/libevm/core/state"
	"github.com/ava-labs/libevm/core/types"
	"github.com/containerman17/pagerdb"
	"github.com/holiman/uint256"
)

func TestMain(m *testing.M) { customtypes.Register(); extstate.RegisterExtras(); os.Exit(m.Run()) }
func TestStateDifferential(t *testing.T) {
	p := pages(t)
	db, e := NewDatabase(p)
	if e != nil {
		t.Fatal(e)
	}
	a, e := state.New(common.Hash{}, db, nil)
	if e != nil {
		t.Fatal(e)
	}
	mem := rawdb.NewMemoryDatabase()
	defer mem.Close()
	b, e := state.New(types.EmptyRootHash, state.NewDatabase(mem), nil)
	if e != nil {
		t.Fatal(e)
	}
	rng := rand.New(rand.NewSource(42))
	addresses := make([]common.Address, 20)
	keys := make([]common.Hash, 32)
	for i := range addresses {
		rng.Read(addresses[i][:])
	}
	for i := range keys {
		rng.Read(keys[i][:])
	}
	check := func() {
		t.Helper()
		for _, addr := range addresses {
			if a.GetBalance(addr).Cmp(b.GetBalance(addr)) != 0 || a.GetNonce(addr) != b.GetNonce(addr) || a.Exist(addr) != b.Exist(addr) || !bytes.Equal(a.GetCode(addr), b.GetCode(addr)) || a.GetCodeHash(addr) != b.GetCodeHash(addr) || customtypes.IsMultiCoin(a, addr) != customtypes.IsMultiCoin(b, addr) {
				t.Fatal("account mismatch")
			}
			for _, k := range keys {
				if a.GetState(addr, k) != b.GetState(addr, k) || a.GetCommittedState(addr, k) != b.GetCommittedState(addr, k) {
					t.Fatal("storage mismatch")
				}
			}
		}
	}
	for block := 0; block < 30; block++ {
		for tx := 0; tx < 8; tx++ {
			sa, sb := a.Snapshot(), b.Snapshot()
			for op := 0; op < 25; op++ {
				addr := addresses[rng.Intn(len(addresses))]
				key := keys[rng.Intn(len(keys))]
				v := common.BigToHash(new(uint256.Int).SetUint64(rng.Uint64()).ToBig())
				switch rng.Intn(9) {
				case 0:
					a.SetNonce(addr, uint64(op))
					b.SetNonce(addr, uint64(op))
				case 1:
					amount := uint256.NewInt(uint64(op))
					a.SetBalance(addr, amount)
					b.SetBalance(addr, amount)
				case 2:
					code := []byte{0x60, byte(op), 0}
					a.SetCode(addr, code)
					b.SetCode(addr, code)
				case 3:
					a.SetState(addr, key, common.Hash{})
					b.SetState(addr, key, common.Hash{})
				case 4:
					a.SelfDestruct(addr)
					b.SelfDestruct(addr)
				case 5:
					a.CreateAccount(addr)
					b.CreateAccount(addr)
				case 6:
					customtypes.SetMultiCoin(a, addr, true)
					customtypes.SetMultiCoin(b, addr, true)
				default:
					a.SetState(addr, key, v)
					b.SetState(addr, key, v)
				}
			}
			if rng.Intn(2) == 0 {
				a.RevertToSnapshot(sa)
				b.RevertToSnapshot(sb)
			}
			check()
			if e = db.Flush(a); e != nil {
				t.Fatal(e)
			}
			b.IntermediateRoot(true)
			check()
		}
		if _, e = db.CommitBlock(a); e != nil {
			t.Fatal(e)
		}
		root, e := b.Commit(uint64(block), true)
		if e != nil {
			t.Fatal(e)
		}
		got, err := db.StateRoot()
		if err != nil || got != root {
			t.Fatalf("block %d state root: got %s want %s error %v", block, got, root, err)
		}
		b, e = state.New(root, b.Database(), nil)
		if e != nil {
			t.Fatal(e)
		}
		a, e = state.New(common.Hash{}, db, nil)
		if e != nil {
			t.Fatal(e)
		}
		check()
	}
	if e = p.Verify(); e != nil {
		t.Fatal(e)
	}
}

func TestDeterministicRootAndReopen(t *testing.T) {
	var want [32]byte
	for run := 0; run < 3; run++ {
		dir := t.TempDir()
		p, e := pagerdb.Open(dir, pagerdb.Options{MaxLeaves: 1 << 12, Workers: 2})
		if e != nil {
			t.Fatal(e)
		}
		db, e := NewDatabase(p)
		if e != nil {
			t.Fatal(e)
		}
		s, e := state.New(common.Hash{}, db, nil)
		if e != nil {
			t.Fatal(e)
		}
		order := rand.New(rand.NewSource(int64(run))).Perm(200)
		for _, i := range order {
			a := common.Address{18: byte(i >> 8), 19: byte(i)}
			s.SetBalance(a, uint256.NewInt(uint64(i+1)))
			s.SetCode(a, []byte{0x60, byte(i)})
			s.SetState(a, common.Hash{31: 1}, common.Hash{30: 1, 31: byte(i)})
		}
		if _, e = db.CommitBlock(s); e != nil {
			t.Fatal(e)
		}
		mptRoot, e := db.StateRoot()
		if e != nil {
			t.Fatal(e)
		}
		root := p.Root()
		if run == 0 {
			want = root
		} else if root != want {
			t.Fatal("root depends on operation order")
		}
		if e = p.Checkpoint(); e != nil {
			t.Fatal(e)
		}
		if e = p.Close(); e != nil {
			t.Fatal(e)
		}
		p, e = pagerdb.Open(dir, pagerdb.Options{MaxLeaves: 1 << 12, Workers: 2})
		if e != nil {
			t.Fatal(e)
		}
		db, e = NewDatabase(p)
		if e != nil {
			t.Fatal(e)
		}
		s, e = state.New(common.Hash{}, db, nil)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 200; i++ {
			a := common.Address{19: byte(i)}
			if s.GetBalance(a).Uint64() != uint64(i+1) || !bytes.Equal(s.GetCode(a), []byte{0x60, byte(i)}) || s.GetState(a, common.Hash{31: 1}) != (common.Hash{30: 1, 31: byte(i)}) {
				t.Fatal("reopen lost account")
			}
		}
		if got, err := db.StateRoot(); err != nil || got != mptRoot {
			t.Fatalf("reopen state root: got %s want %s error %v", got, mptRoot, err)
		}
		if p.Root() != root {
			t.Fatal("reopen changed root")
		}
		if e = p.Close(); e != nil {
			t.Fatal(e)
		}
	}
}
