#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
for f in testdata/cases/*.wat; do
 cargo run --quiet --locked --release --manifest-path instrument/Cargo.toml --bin wat -- "$f" "${f%.wat}.wasm"
done
