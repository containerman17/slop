//go:build linux

package pagerwasm

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Optional log layout separates syscall count from the anonymous-memory bound.
// Records contain leaf IDs, leaf bytes and a root, but no crash recovery protocol.
type idealLog struct {
	fd, leaf       int
	rootOnly       bool
	offset         int64
	header, buffer []byte
}

func (l *idealLog) commit(ids []int, data []byte, snapshot bool, r idealCommitResult, durable bool) idealCommitResult {
	start := time.Now()
	var bufs [][]byte
	if l.rootOnly {
		bufs = [][]byte{r.root[:]}
	} else {
		header := l.header[:48+len(ids)*4]
		binary.LittleEndian.PutUint64(header, uint64(len(ids)))
		binary.LittleEndian.PutUint64(header[8:], uint64(l.leaf))
		copy(header[16:48], r.root[:])
		for i, id := range ids {
			binary.LittleEndian.PutUint32(header[48+i*4:], uint32(id))
		}
		if !snapshot {
			for i, id := range ids {
				copy(l.buffer[i*l.leaf:(i+1)*l.leaf], data[id*l.leaf:(id+1)*l.leaf])
			}
			data = l.buffer[:len(ids)*l.leaf]
		}
		bufs = [][]byte{header, data}
	}
	r.pack = time.Since(start)
	start = time.Now()
	// One vectored write combines the small index header and contiguous leaf payload.
	for len(bufs) > 0 {
		n, err := unix.Pwritev(l.fd, bufs, l.offset)
		r.calls++
		r.bytes += n
		l.offset += int64(n)
		if err != nil {
			r.err = err
			return r
		}
		if n == 0 {
			r.err = io.ErrShortWrite
			return r
		}
		for len(bufs) > 0 && n >= len(bufs[0]) {
			n -= len(bufs[0])
			bufs = bufs[1:]
		}
		if len(bufs) > 0 {
			bufs[0] = bufs[0][n:]
		}
	}
	r.write = time.Since(start)
	if durable {
		start = time.Now()
		r.err = unix.Fdatasync(l.fd)
		r.sync = time.Since(start)
	}
	return r
}
func (l *idealLog) verify(t *testing.T, f *os.File, want []byte, roots [][32]byte, pool *idealPool) {
	t.Helper()
	if l.rootOnly {
		var got [32]byte
		for block, want := range roots {
			if _, err := f.ReadAt(got[:], int64(block)*32); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("root file mismatch block %d", block)
			}
		}
		if l.offset != int64(len(roots))*32 {
			t.Fatal("root file length mismatch")
		}
		t.Logf("verified root-only file: %d roots; no state data persisted", len(roots))
		return
	}
	state := make([]byte, len(want))
	if _, err := f.ReadAt(state, 0); err != nil {
		t.Fatal(err)
	}
	tr := newIdealTree(state, l.leaf, pool)
	offset := int64(len(state))
	header := make([]byte, 48)
	idsBuf := make([]byte, len(state)/l.leaf*4)
	payload := make([]byte, len(state))
	for block, root := range roots {
		if _, err := f.ReadAt(header, offset); err != nil {
			t.Fatal(err)
		}
		offset += 48
		n := int(binary.LittleEndian.Uint64(header))
		if n < 1 || n > len(state)/l.leaf || binary.LittleEndian.Uint64(header[8:]) != uint64(l.leaf) || !bytes.Equal(header[16:48], root[:]) {
			t.Fatalf("log header mismatch block %d", block)
		}
		indices := idsBuf[:n*4]
		if _, err := f.ReadAt(indices, offset); err != nil {
			t.Fatal(err)
		}
		offset += int64(len(indices))
		data := payload[:n*l.leaf]
		if _, err := f.ReadAt(data, offset); err != nil {
			t.Fatal(err)
		}
		offset += int64(len(data))
		ids := make([]int, n)
		for i := range ids {
			id := int(binary.LittleEndian.Uint32(indices[i*4:]))
			if id >= len(state)/l.leaf || (i > 0 && id <= ids[i-1]) {
				t.Fatal("invalid log leaf ID")
			}
			ids[i] = id
			copy(state[id*l.leaf:(id+1)*l.leaf], data[i*l.leaf:(i+1)*l.leaf])
		}
		tr.update(ids, data, true)
		if tr.nodes[1] != root {
			t.Fatalf("log replay root mismatch block %d", block)
		}
	}
	if offset != l.offset || !bytes.Equal(state, want) {
		t.Fatal("log replay final state mismatch")
	}
	t.Logf("verified packed log replay: %d roots, %d bytes including initial state", len(roots), offset)
}
