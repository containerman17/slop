#!/bin/sh
set -eu
cd "$(dirname "$0")"
RUSTFLAGS='-C target-feature=+bulk-memory,-simd128,-reference-types,-multivalue -C link-arg=--initial-memory=536870912 -C link-arg=--max-memory=536870912' cargo build --release --target wasm32-unknown-unknown -p wasm-native
cp target/wasm32-unknown-unknown/release/wasm_native.wasm ../matcher.wasm
# Re-encode padded linker immediates without lowering bulk operations.
wasm-opt ../matcher.wasm -o ../matcher.wasm
chmod 644 ../matcher.wasm
