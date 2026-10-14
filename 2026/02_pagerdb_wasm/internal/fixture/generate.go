// Package fixture generates synthetic CLOB1 messages. It contains no source records.
package fixture

import (
	"encoding/binary"
	"io"
	"math/rand"
)

// Measured shape over 30M source messages: Add/Cancel/Replace/Modify counts
// 15386807/12825097/807780/980316; buy fraction .5009099; 9812 symbols;
// top 1/10/100 symbol shares .01594023/.10251797/.36354957;
// distinct price levels per symbol p50/p90/p99 47/432/1957;
// absolute distance from quoted mid (bps) p50/p90/p99 11/1203/10310;
// qty p50/p90/p99 100/1000/7800; crossing Adds .04277002;
// cancel age in message positions p50/p90/p99 2879/1040436/12640200.
// The generator approximates these aggregates, not the original event stream.
const Symbols = 9812

type order struct {
	id        uint64
	price     uint64
	symbol    uint16
	side      byte
	available bool
}

// Write writes blocks of 50,000 records using a fixed seed. Synthetic prices,
// identifiers and symbol numbers have no relationship to source values.
func Write(w io.Writer, blocks int) error {
	r := rand.New(rand.NewSource(1))
	var header [16]byte
	copy(header[:], "CLOB1")
	binary.LittleEndian.PutUint32(header[8:], 50000)
	binary.LittleEndian.PutUint32(header[12:], uint32(blocks))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	history := make([]order, 0, blocks*28000)
	symbol := func() uint16 {
		q := r.Float64()
		switch {
		case q < .01594023:
			return 0
		case q < .10251797:
			return uint16(1 + r.Intn(9))
		case q < .36354957:
			return uint16(10 + r.Intn(90))
		default:
			return uint16(100 + r.Intn(Symbols-100))
		}
	}
	quantity := func() uint32 {
		q := r.Intn(17174903)
		switch {
		case q < 935372:
			return 1
		case q < 2802799:
			return uint32(2 + r.Intn(9))
		case q < 10407599:
			return uint32(10 * (1 + r.Intn(10)))
		case q < 15610752:
			return uint32(100 * (2 + r.Intn(9)))
		default:
			return uint32(100 * (11 + r.Intn(100)))
		}
	}
	price := func(sym uint16, side byte, aggressive bool) uint64 {
		mid := int64(1000000 + int(sym%100)*10000)
		if aggressive {
			if side == 1 {
				return uint64(mid * 5)
			}
			return 1
		}
		levels := 47
		if sym < 981 {
			levels = 432
		}
		if sym < 98 {
			levels = 1957
		}
		if sym < 10 {
			levels = 6000
		}
		level := r.Intn(levels)
		q := float64(level) / float64(levels)
		var bps float64
		switch {
		case q < .5:
			bps = 1 + q*20
		case q < .9:
			bps = 11 + (q-.5)*2980
		case q < .99:
			bps = 1203 + (q-.9)*101188
		default:
			bps = 10310 + (q-.99)*1000000
		}
		delta := int64(float64(mid) * bps / 10000)
		if delta < 100 {
			delta = 100
		}
		delta = delta / 100 * 100
		if side == 1 {
			if delta >= mid {
				return 100
			}
			return uint64(mid - delta)
		}
		return uint64(mid + delta)
	}
	block := make([]byte, 50000*40)
	var nextID uint64 = 1
	for b := 0; b < blocks; b++ {
		clear(block)
		for i := 0; i < 50000; i++ {
			record := block[i*40 : (i+1)*40]
			draw := r.Intn(30000000)
			op := byte(1)
			switch {
			case draw < 15386807:
				op = 1
			case draw < 28211904:
				op = 2
			case draw < 29019684:
				op = 3
			default:
				op = 4
			}
			side := byte(1)
			if r.Float64() >= .5009099 {
				side = 2
			}
			sym := symbol()
			id := nextID
			var newID uint64
			var p uint64
			chosen := -1
			if op != 1 && len(history) > 0 {
				q := r.Float64()
				var lag int
				switch {
				case q < .5:
					lag = 1 + int(q*5758)
				case q < .9:
					lag = 2879 + int((q-.5)*2593892)
				case q < .99:
					lag = 1040436 + int((q-.9)*128886266)
				default:
					lag = 12640200 + int((q-.99)*100000000)
				}
				target := len(history) - 1 - int(float64(lag)*.54)
				if target < 0 {
					target = 0
				}
				for j := target; j < len(history) && j < target+256; j++ {
					if history[j].available {
						chosen = j
						break
					}
				}
				if chosen < 0 {
					for j := len(history) - 1; j >= 0; j-- {
						if history[j].available {
							chosen = j
							break
						}
					}
				}
			}
			if chosen >= 0 {
				o := &history[chosen]
				sym, side, id, p = o.symbol, o.side, o.id, o.price
				if op == 2 || op == 3 {
					o.available = false
				}
			} else if op != 1 {
				op = 1
			}
			if op == 1 || op == 3 {
				p = price(sym, side, r.Float64() < .04277002)
				if op == 3 {
					newID = nextID
				} else {
					id = nextID
				}
				history = append(history, order{id: nextID, price: p, symbol: sym, side: side, available: true})
				nextID++
			}
			record[0], record[1] = op, side
			binary.LittleEndian.PutUint16(record[2:], sym)
			binary.LittleEndian.PutUint16(record[4:], uint16(r.Intn(200)))
			binary.LittleEndian.PutUint16(record[6:], uint16(r.Intn(8)))
			if op != 2 {
				binary.LittleEndian.PutUint32(record[8:], quantity())
				binary.LittleEndian.PutUint64(record[16:], p)
			}
			binary.LittleEndian.PutUint64(record[24:], id)
			binary.LittleEndian.PutUint64(record[32:], newID)
		}
		if _, err := w.Write(block); err != nil {
			return err
		}
	}
	return nil
}
