package pagerevm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/ava-labs/avalanchego/graft/coreth/plugin/evm/customtypes"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/state"
	"github.com/ava-labs/libevm/core/types"
	"github.com/ava-labs/libevm/ethdb"
	"github.com/ava-labs/libevm/rlp"
	"github.com/ava-labs/libevm/trie"
	"github.com/ava-labs/libevm/trie/trienode"
	"github.com/ava-labs/libevm/triedb"
	"github.com/containerman17/pagerdb"
	"github.com/holiman/uint256"
)

var unsupported = errors.New("operation is outside the linear replay adapter")
var storagePresent = common.Hash{31: 1}

// Database uses stock StateDB journaling. Flush must run after every transaction
// and atomic transfer group. CommitBlock is the only state commitment operation.
type Database struct {
	Pages           *pagerdb.DB
	accounts, codes *Table
	pending         map[common.Address]*types.StateAccount
	slots           map[common.Address]map[common.Hash]common.Hash
	code            map[common.Hash][]byte
	resets          map[common.Address]bool
}

func NewDatabase(p *pagerdb.DB) (*Database, error) {
	a, c := &Table{p, 1, 80}, &Table{p, 2, 4}
	if _, err := p.Map(1); errors.Is(err, pagerdb.ErrRegion) && p.Version() == 0 {
		var e error
		a, e = NewTable(p, 80)
		if e != nil {
			return nil, e
		}
		c, e = NewTable(p, 4)
		if e != nil {
			return nil, e
		}
		if a.Region != 1 || c.Region != 2 {
			return nil, errors.New("adapter requires an exclusive empty PagerDB")
		}
	} else {
		for _, t := range []*Table{a, c} {
			b, e := p.Map(t.Region)
			if e != nil {
				return nil, e
			}
			if len(b) < 16 || binary.LittleEndian.Uint64(b[8:16]) != uint64(t.ValueSize) || (len(b)-16)%(32+t.ValueSize) != 0 {
				return nil, errors.New("invalid adapter table")
			}
			n := t.Capacity()
			if n < 16 || n&(n-1) != 0 || t.Len() > n {
				return nil, errors.New("invalid adapter capacity")
			}
		}
	}
	return &Database{resets: make(map[common.Address]bool), Pages: p, accounts: a, codes: c, pending: make(map[common.Address]*types.StateAccount), slots: make(map[common.Address]map[common.Hash]common.Hash), code: make(map[common.Hash][]byte)}, nil
}
func addressKey(a common.Address) (k [32]byte) { copy(k[:20], a[:]); return }
func (d *Database) slotTable(a common.Address) *Table {
	b := d.accounts.Get(addressKey(a))
	r := pagerdb.Region(binary.LittleEndian.Uint32(b[72:76]))
	if r == 0 {
		return nil
	}
	return &Table{d.Pages, r, 32}
}
func (d *Database) OpenTrie(root common.Hash) (state.Trie, error) {
	if root != (common.Hash{}) {
		return nil, errors.New("linear adapter requires the zero current-state selector; historical roots are unsupported")
	}
	return &regionTrie{d: d}, nil
}
func (d *Database) OpenStorageTrie(_ common.Hash, a common.Address, root common.Hash, _ state.Trie) (state.Trie, error) {
	return &regionTrie{d: d, address: &a, empty: root == types.EmptyRootHash}, nil
}
func (d *Database) CopyTrie(state.Trie) state.Trie { panic(unsupported) }
func (d *Database) DiskDB() ethdb.KeyValueStore    { panic(unsupported) }
func (d *Database) TrieDB() *triedb.Database       { panic(unsupported) }
func (d *Database) ContractCode(_ common.Address, h common.Hash) ([]byte, error) {
	if c, ok := d.code[h]; ok {
		return bytes.Clone(c), nil
	}
	r := pagerdb.Region(binary.LittleEndian.Uint32(d.codes.Get(h)))
	if r == 0 {
		return nil, nil
	}
	b, e := d.Pages.Map(r)
	return bytes.Clone(b), e
}
func (d *Database) ContractCodeSize(a common.Address, h common.Hash) (int, error) {
	b, e := d.ContractCode(a, h)
	return len(b), e
}
func sorted[K ~[20]byte | ~[32]byte, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		for n := 0; n < len(a); n++ {
			if a[n] != b[n] {
				return a[n] < b[n]
			}
		}
		return false
	})
	return out
}
func (d *Database) Flush(s *state.StateDB) error {
	s.IntermediateRoot(true)
	if e := s.Error(); e != nil {
		return e
	}
	// Stock StateDB iterates Go maps. Sort before assigning regions or buckets.
	for _, h := range sorted(d.code) {
		if zero(d.codes.Get(h)) && len(d.code[h]) > 0 {
			r, e := d.Pages.Alloc(len(d.code[h]))
			if e != nil {
				return e
			}
			d.Pages.Write(r, 0, d.code[h])
			var v [4]byte
			binary.LittleEndian.PutUint32(v[:], uint32(r))
			if e = d.codes.Set(h, v[:]); e != nil {
				return e
			}
		}
	}
	all := make(map[common.Address]bool)
	for a := range d.pending {
		all[a] = true
	}
	for a := range d.slots {
		all[a] = true
	}
	for _, a := range sorted(all) {
		k := addressKey(a)
		record := d.accounts.Get(k)
		st := d.slotTable(a)
		if d.resets[a] || (d.pending[a] != nil && d.pending[a].Root == types.EmptyRootHash) {
			if st != nil {
				if e := d.Pages.Free(st.Region); e != nil {
					return e
				}
			}
			st = nil
			clear(record[72:76])
		}
		if acc, ok := d.pending[a]; ok {
			if acc == nil {
				if st != nil {
					if e := d.Pages.Free(st.Region); e != nil {
						return e
					}
				}
				if e := d.accounts.Set(k, make([]byte, 80)); e != nil {
					return e
				}
				continue
			}
			binary.LittleEndian.PutUint64(record[:8], acc.Nonce)
			bal := acc.Balance.Bytes32()
			copy(record[8:40], bal[:])
			copy(record[40:72], acc.CodeHash)
			record[76] = 0
			if customtypes.IsAccountMultiCoin(acc) {
				record[76] = 1
			}
			record[77] = 1
		}
		if len(d.slots[a]) > 0 {
			if st == nil {
				var e error
				st, e = NewTable(d.Pages, 32)
				if e != nil {
					return e
				}
				binary.LittleEndian.PutUint32(record[72:76], uint32(st.Region))
			}
			for _, k := range sorted(d.slots[a]) {
				v := d.slots[a][k]
				if e := st.Set(k, v[:]); e != nil {
					return e
				}
			}
		}
		if e := d.accounts.Set(k, record); e != nil {
			return e
		}
	}
	clear(d.resets)
	clear(d.pending)
	clear(d.slots)
	clear(d.code)
	return nil
}
func (d *Database) CommitBlock(s *state.StateDB) (uint64, error) {
	if e := d.Flush(s); e != nil {
		return 0, e
	}
	return d.Pages.Commit()
}

type regionTrie struct {
	d       *Database
	address *common.Address
	empty   bool
}

func (t *regionTrie) GetAccount(a common.Address) (*types.StateAccount, error) {
	if v, ok := t.d.pending[a]; ok {
		return v, nil
	}
	b := t.d.accounts.Get(addressKey(a))
	if b[77] == 0 {
		return nil, nil
	}
	root := types.EmptyRootHash
	if r := t.d.slotTable(a); r != nil && r.Len() > 0 {
		root = storagePresent
	}
	// The bool belongs to coreth's registered StateAccount payload. No storage
	// root is persisted; this marker only tells stock StateDB storage is present.
	raw, e := rlp.EncodeToBytes([]any{binary.LittleEndian.Uint64(b[:8]), new(uint256.Int).SetBytes(b[8:40]).ToBig(), root, b[40:72], b[76] != 0})
	if e != nil {
		return nil, e
	}
	var acc types.StateAccount
	e = rlp.DecodeBytes(raw, &acc)
	return &acc, e
}
func (t *regionTrie) GetStorage(a common.Address, k []byte) ([]byte, error) {
	h := common.BytesToHash(k)
	if v, ok := t.d.slots[a][h]; ok {
		return bytes.TrimLeft(v[:], "\x00"), nil
	}
	if t.empty {
		return nil, nil
	}
	st := t.d.slotTable(a)
	if st == nil {
		return nil, nil
	}
	return bytes.TrimLeft(st.Get(h), "\x00"), nil
}
func (t *regionTrie) UpdateAccount(a common.Address, v *types.StateAccount) error {
	c := *v
	c.Balance = new(uint256.Int).Set(v.Balance)
	c.CodeHash = bytes.Clone(v.CodeHash)
	t.d.pending[a] = &c
	return nil
}
func (t *regionTrie) DeleteAccount(a common.Address) error {
	t.d.pending[a] = nil
	delete(t.d.slots, a)
	return nil
}
func (t *regionTrie) UpdateStorage(a common.Address, k, v []byte) error {
	if t.empty {
		t.d.resets[a] = true
	}
	if t.d.slots[a] == nil {
		t.d.slots[a] = make(map[common.Hash]common.Hash)
	}
	t.d.slots[a][common.BytesToHash(k)] = common.BytesToHash(v)
	return nil
}
func (t *regionTrie) DeleteStorage(a common.Address, k []byte) error {
	return t.UpdateStorage(a, k, nil)
}
func (t *regionTrie) UpdateContractCode(_ common.Address, h common.Hash, b []byte) error {
	t.d.code[h] = bytes.Clone(b)
	return nil
}
func (t *regionTrie) Hash() common.Hash { t.empty = false; return storagePresent }
func (t *regionTrie) Commit(bool) (common.Hash, *trienode.NodeSet, error) {
	return common.Hash{}, nil, unsupported
}
func (t *regionTrie) GetKey([]byte) []byte                           { panic(unsupported) }
func (t *regionTrie) NodeIterator([]byte) (trie.NodeIterator, error) { return nil, unsupported }
func (t *regionTrie) Prove([]byte, ethdb.KeyValueWriter) error       { return unsupported }
