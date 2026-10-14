// Writes a synthetic fixture to stdout. Usage: go run ./cmd/fixture [blocks].
package main

import (
	"github.com/containerman17/slop/2026/02_pagerdb_wasm/internal/fixture"
	"os"
	"strconv"
)

func main() {
	blocks := 20
	if len(os.Args) > 1 {
		var err error
		blocks, err = strconv.Atoi(os.Args[1])
		if err != nil || blocks < 1 {
			panic("blocks must be positive")
		}
	}
	if err := fixture.Write(os.Stdout, blocks); err != nil {
		panic(err)
	}
}
