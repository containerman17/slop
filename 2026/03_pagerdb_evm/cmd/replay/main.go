// Command replay checks local C-Chain block dumps with a final state MPT comparison.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/graft/coreth/consensus"
	"github.com/ava-labs/avalanchego/graft/coreth/consensus/dummy"
	core "github.com/ava-labs/avalanchego/graft/coreth/core"
	"github.com/ava-labs/avalanchego/graft/coreth/core/extstate"
	cp "github.com/ava-labs/avalanchego/graft/coreth/params"
	"github.com/ava-labs/avalanchego/graft/coreth/params/extras"
	"github.com/ava-labs/avalanchego/graft/coreth/plugin/evm/atomic"
	"github.com/ava-labs/avalanchego/graft/coreth/plugin/evm/customtypes"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/upgrade"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/state"
	"github.com/ava-labs/libevm/core/types"
	"github.com/ava-labs/libevm/core/vm"
	"github.com/ava-labs/libevm/rlp"
	"github.com/ava-labs/libevm/trie"
	"github.com/containerman17/pagerdb"
	adapter "github.com/containerman17/slop/2026/03_pagerdb_evm"
	"github.com/holiman/uint256"
	"github.com/klauspost/compress/zstd"
)

type chain struct{ headers map[uint64]*types.Header }

func (c chain) Engine() consensus.Engine { return dummy.NewFullFaker() }
func (c chain) GetHeader(h common.Hash, n uint64) *types.Header {
	v := c.headers[n]
	if v != nil && v.Hash() == h {
		return v
	}
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() (result error) {
	data := flag.String("data", "", "local mainnet-C dump directory")
	dir := flag.String("state", "", "new PagerDB directory (default: temporary)")
	end := flag.Uint64("to", 100000, "last block")
	rootEvery := flag.Uint64("root-every", 0, "also rebuild and check state root every N blocks (0: final only)")
	flag.Parse()
	if *end == 0 {
		return fmt.Errorf("-to must be positive")
	}
	if *data == "" {
		return fmt.Errorf("-data is required")
	}
	core.RegisterExtras()
	customtypes.Register()
	extstate.RegisterExtras()
	cp.RegisterExtras()
	var index struct {
		Genesis        common.Hash
		Network, Chain string
		SegmentBlocks  uint64
		Segments       []struct{ First, Last uint64 }
	}
	raw, e := os.ReadFile(filepath.Join(*data, "index.json"))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &index); e != nil {
		return e
	}
	if index.Network != "mainnet" || index.Chain == "" || index.SegmentBlocks != 100000 {
		return fmt.Errorf("expected mainnet C 100000-block segments")
	}
	cfg := genesis.GetConfig(1)
	var g core.Genesis
	if e = json.Unmarshal([]byte(cfg.CChainGenesis), &g); e != nil {
		return e
	}
	_, asset, e := genesis.FromConfig(cfg)
	if e != nil {
		return e
	}
	ctx := &snow.Context{NetworkID: 1, AVAXAssetID: asset, NetworkUpgrades: upgrade.GetConfig(1)}
	extra := cp.GetExtra(g.Config)
	extra.AvalancheContext = extras.AvalancheContext{SnowCtx: ctx}
	extra.NetworkUpgrades = extras.GetNetworkUpgrades(ctx.NetworkUpgrades)
	if e = extra.Verify(); e != nil {
		return e
	}
	if e = cp.SetEthUpgrades(g.Config); e != nil {
		return e
	}
	if *end >= 1640340 {
		return fmt.Errorf("this phase limits replay to pre-Berlin blocks")
	}
	if *dir == "" {
		*dir, e = os.MkdirTemp("", "pager-evm-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(*dir)
	} else {
		entries, e := os.ReadDir(*dir)
		if e == nil && len(entries) > 0 {
			return fmt.Errorf("state directory must be empty")
		}
	}
	p, e := pagerdb.Open(*dir, pagerdb.Options{Workers: 4, RetainVersions: 2, CheckpointInterval: 10000})
	if e != nil {
		return e
	}
	defer func() {
		if err := p.Close(); result == nil {
			result = err
		}
	}()
	db, e := adapter.NewDatabase(p)
	if e != nil {
		return e
	}
	s, e := state.New(common.Hash{}, db, nil)
	if e != nil {
		return e
	}
	for a, v := range g.Alloc {
		s.SetBalance(a, uint256.MustFromBig(v.Balance))
		s.SetNonce(a, v.Nonce)
		s.SetCode(a, v.Code)
		for k, v := range v.Storage {
			s.SetState(a, k, v)
		}
	}
	if _, e = db.CommitBlock(s); e != nil {
		return e
	}
	cc := chain{make(map[uint64]*types.Header)}
	prev := index.Genesis
	parentTime := g.Timestamp
	start := time.Now()
	var count, txs, gas, imports, exports uint64
	for _, seg := range index.Segments {
		if seg.First > *end {
			break
		}
		if seg.First != count+1 {
			return fmt.Errorf("segment gap after %d", count)
		}
		f, e := os.Open(filepath.Join(*data, fmt.Sprintf("seg-%09d.bin.zst", seg.First)))
		if e != nil {
			return e
		}
		z, e := zstd.NewReader(f)
		if e != nil {
			f.Close()
			return e
		}
		e = func() error {
			defer f.Close()
			defer z.Close()
			for n := seg.First; n <= seg.Last && n <= *end; n++ {
				var size [4]byte
				if _, e := io.ReadFull(z, size[:]); e != nil {
					return e
				}
				length := binary.LittleEndian.Uint32(size[:])
				if length > 64<<20 {
					return fmt.Errorf("oversized block %d", n)
				}
				raw := make([]byte, length)
				if _, e := io.ReadFull(z, raw); e != nil {
					return e
				}
				var b types.Block
				if e := rlp.DecodeBytes(raw, &b); e != nil {
					return fmt.Errorf("block %d decode: %w", n, e)
				}
				if b.NumberU64() != n || b.ParentHash() != prev {
					return fmt.Errorf("block %d chain discontinuity", n)
				}
				s, e = state.New(common.Hash{}, db, nil)
				if e != nil {
					return e
				}
				h := b.Header()
				bc := core.NewEVMBlockContext(h, cc, nil)
				originalGetHash := bc.GetHash
				bc.GetHash = func(height uint64) common.Hash {
					if height == 0 {
						return index.Genesis
					}
					return originalGetHash(height)
				}
				gp := new(core.GasPool).AddGas(h.GasLimit)
				var used uint64
				var receipts types.Receipts
				if e = core.ApplyUpgrades(g.Config, &parentTime, core.NewBlockContext(h.Number, h.Time), s); e != nil {
					return e
				}
				for i, tx := range b.Transactions() {
					s.SetTxContext(tx.Hash(), i)
					r, e := core.ApplyTransaction(g.Config, cc, bc, gp, s, h, tx, &used, vm.Config{})
					if e != nil {
						return fmt.Errorf("block %d tx %d execution: %w", n, i, e)
					}
					receipts = append(receipts, r)
					if e = db.Flush(s); e != nil {
						return fmt.Errorf("block %d tx %d flush: %w", n, i, e)
					}
				}
				// Receipt commitments use Ethereum's receipt trie.
				if used != h.GasUsed {
					return fmt.Errorf("block %d gas mismatch after tx %d: got %d want %d", n, len(receipts)-1, used, h.GasUsed)
				}
				if types.DeriveSha(receipts, trie.NewStackTrie(nil)) != h.ReceiptHash {
					return fmt.Errorf("block %d receipts mismatch after tx %d", n, len(receipts)-1)
				}
				if types.CreateBloom(receipts) != h.Bloom {
					return fmt.Errorf("block %d bloom mismatch after tx %d", n, len(receipts)-1)
				}
				if ext := customtypes.BlockExtData(&b); len(ext) > 0 {
					rules := cp.GetRulesExtra(g.Config.Rules(h.Number, cp.IsMergeTODO, h.Time))
					atxs, e := atomic.ExtractAtomicTxs(ext, rules.IsApricotPhase5, atomic.Codec)
					if e != nil {
						return fmt.Errorf("block %d atomic decode: %w", n, e)
					}
					for i, a := range atxs {
						switch a.UnsignedAtomicTx.(type) {
						case *atomic.UnsignedImportTx:
							imports++
						case *atomic.UnsignedExportTx:
							exports++
						}
						if e = a.UnsignedAtomicTx.EVMStateTransfer(ctx, extstate.New(s)); e != nil {
							return fmt.Errorf("block %d atomic tx %d: %w", n, i, e)
						}
					}
				}
				if _, e = db.CommitBlock(s); e != nil {
					return fmt.Errorf("block %d commit: %w", n, e)
				}
				if n == *end || (*rootEvery > 0 && n%*rootEvery == 0) {
					rootStart := time.Now()
					root, err := db.StateRoot()
					if err != nil {
						return fmt.Errorf("block %d state root: %w", n, err)
					}
					if root != h.Root {
						return fmt.Errorf("block %d STATE ROOT MISMATCH: got %s want %s", n, root, h.Root)
					}
					fmt.Printf("state-root PASS block=%d root=%s elapsed=%s\n", n, root, time.Since(rootStart).Round(time.Millisecond))
				}
				count = n
				txs += uint64(len(receipts))
				gas += used
				prev = b.Hash()
				parentTime = h.Time
				cc.headers[n] = h
				delete(cc.headers, n-256)
				if n%10000 == 0 {
					fmt.Printf("verified=%d txs=%d elapsed=%s blocks/s=%.1f\n", n, txs, time.Since(start).Round(time.Millisecond), float64(n)/time.Since(start).Seconds())
				}
			}
			return nil
		}()
		if e != nil {
			return e
		}
	}
	if count != *end {
		return fmt.Errorf("dump ended at %d, requested %d", count, *end)
	}
	if e = p.Flush(); e != nil {
		return e
	}
	if e = p.Verify(); e != nil {
		return e
	}
	fmt.Printf("PASS blocks=%d txs=%d gas=%d imports=%d exports=%d elapsed=%s blocks/s=%.1f\n", count, txs, gas, imports, exports, time.Since(start).Round(time.Millisecond), float64(count)/time.Since(start).Seconds())
	return nil
}
