# subnet-evm upgrade.json files for live Avalanche L1s

These are the subnet-evm `upgrade.json` files a node must run today for the
mainnet and Fuji L1s we know about. A node reads the file from
`<chain-config-dir>/<blockchainID>/upgrade.json`; the file name here is
`<network>/<name>-<blockchainID>.json`, so copy it to that path. To check a file
against a running chain, call `eth_getChainConfig` on its RPC and compare
`result.upgrades` with the file as parsed JSON (this is the parsed
`UpgradeConfig`, the same object `upgrade.json` unmarshals into). A node without
the file it needs fails at the first activation block with `invalid merkle root`.

Live checks ran on 2026-10-27 JST. RPC URLs come from
`https://glacier-api.avax.network/v1/chains?network=mainnet` (field `rpcUrl`).
The comparison ignores address case and zero-value fields that the RPC adds.
"held" means the file came from our own node deployments or backups; "live"
means the file is `result.upgrades` written verbatim.

| network | name | blockchainID | evmChainId | stateUpgrades | source | live check |
|---|---|---|---|---|---|---|
| mainnet | beam | `2tmrrBo1Lgt1mzzvPSFt73kkQKFas5d1AP88tv9cicwoFp8BSn` | 4337 | no | live | held file was STALE, replaced by live |
| mainnet | dexalot | `21Ths5Afqi5r4PaoV8r8cruGZWhN11y5rxvy89K8px7pKy3P8E` | 432204 | no | live | held file was STALE, replaced by live |
| mainnet | dfk | `q2aTwKuyzgs8pynF7UXBZCU7DejbZbZ6EUyHr3JQzYgwNPUPi` | 53935 | yes | live 2026-08-06 | NO-RPC now (405) |
| mainnet | gunz | `2M47TxWHGnhNtq6pM5zPXdATBtuqubxn5EPFgFmEawCQr9WFML` | 43419 | no | live | MATCH |
| mainnet | hashfire | `2vcBHFK7jiv74dyYeUCUiCsKy2jbWx6xEQzNfQPiLmYidBaDfX` | 7272 | no | held | MATCH |
| mainnet | innovo | `vFHT9J4F6PhqCTMmqkKKUKW3yfsjd6tNvhmcK4MkFa3Pmnnqe` | 10036 | yes | held | MATCH |
| mainnet | lamina1 | `UhReZTXT8Cqsjat9ghRtCe5kBQPQexQB5zG5Fvf3egrdYfyoJ` | 10849 | no | held, from the lamina1-integration README (2025-02) | NO-RPC (405) |
| mainnet | numbers | `2PDRxzc6jMbZSTLb3sufkVszgQc2jtDnYZGtDTAAfom1CTwPsE` | 10507 | no | held | MATCH (mainnetrpc.num.network) |
| mainnet | numine | `2c1BN4N9qEhNmW4yCpeLK24SfjFQLyS1Z7FtgRDaYxZWFUUKxf` | 8021 | no | held | NO-RPC (405); matches the config our epochdb archive node synced to tip with |
| mainnet | pandasea | `2gYSmRb7tjS4wb5FHFx8NBhwQgARWEdr8MGVC7v8UWJJFkEpTj` | 7776 | no | held | MATCH (rpc-mainnet-pandasea-rl.cogitus.io) |
| mainnet | playa3ull | `k2SFEZ2MZr9UGXiycnA1DdaLqZTKDaHK7WUXVLhJk5F9DD8r1` | 3011 | yes | held | MATCH |
| mainnet | space | `22CN6x5LAPEkvLDdz4UwG3XXtZV69Su3bcspiYtkF9k5f9rcCt` | 8227 | no | held | NO-RPC (405); matches the config our epochdb archive node synced to tip with |
| mainnet | step | `2jRZvKtXY5nyWTqRwFh1KMHGrCRxJoULu4r2CsayWRnjdDGbV1` | 1234 | no | held | NO-RPC (rpc.step.network does not answer) |
| mainnet | straitsx | `EJ4DyXHe4ydhsLLMiDPsHtoq5RDqgyao6Lwb9znKhs59q4NQx` | 5566 | yes | held | MATCH |
| mainnet | titan | `24gRTyzFmXUPebVwzQHuwU6RFeKHHDxRftRVaSAe7MHRXWxZ4S` | 84358 | yes | held | MATCH |
| mainnet | uptn | `2tsFdSeemtqK9vaPgjPrcwvKFKPw1T6PHufaZ26tKbAFSkXPTE` | 6119 | yes | held | MATCH |
| fuji | beam | `y97omoP2cSyEVfdSztQHXD9EnfnVP9YKjZwAxhUfGbLAPYT9t` | 13337 | yes | live | MATCH |
| fuji | dexalot | `XuEPnCE59rtutASDPCDeYw8geQaGWwteWjkDXYLWvssfuirde` | 432201 | no | live | MATCH |
| fuji | dispatch | `2D8RG4UpSXbPbvPCAWppNJyqTG2i2CAXSkTgmTBBvs7GKNZjsY` | 779672 | no | live | MATCH |
| fuji | echo | `98qnjenm7MBd8G2cPZoRvZrgJC33JGSAAKghsQ6eojbLCeRNp` | 173750 | yes | live | MATCH |
| fuji | egmtester | `Buqpg8qoENGbZEC1LY1ngkHYgS5ftZ8M22QCf3e7DWbc3yRDM` | 55197 | yes | live | MATCH |
| fuji | tixchain | `rbPy3vFTgtGccWouLkCXqgDTxyEHEpam2kA24fZUmjgKMNcXX` | 723107 | yes | live | MATCH |
| fuji | tkintl-alpha-csl-ava-staging | `2Huy194qfidjYSAXrVGzwa8HMEXtKSxB55ja5yjfQGYbc4XqZS` | 131076 | no | live | MATCH |
| fuji | uptn | `2Pk23ZAG5gkQ4DobkRsmwW3NaCwR46zxsDv5EY2GViGL6ow18a` | 6118 | yes | live | MATCH |

Stale held files: beam and dexalot mainnet held files lacked
`networkUpgradeOverrides.heliconTimestamp: 1790100120` (2026-09-23 03:02 JST);
everything else was equal.

PandaSea's file uses `txBlockListConfig`, a precompile that exists only in
PandaSea's subnet-evm fork. Stock subnet-evm rejects this file.

## No file needed

`eth_getChainConfig` returns empty `upgrades` for these chains, so they need no
`upgrade.json` (an empty `{}` file is not the same as no file, so do not write one).

Mainnet: Blaze (`2SfshBPqRJexmqdWBY2xYSTq2Rp2dDisuax7nQB6GyNqPjSWWY`),
Dinari Financial Network (`C5GXnNHNEfDxUydtjZ5B5J6XspdAZJGLsX3u4wxCXZgE3N4Gq`),
Elysium (`3jVp8Ee4CarD5Sua3qDPwNGNRCEetSSGToWcwzwkKeHbbChcL`),
even (`2YnwDMW9shSuV6nuYpAhm3DE2KvYJP2GFDvUde6XG8jE5gxvpL`),
FIFA Blockchain (`SUDoK9P89PCcguskyof41fZexw7U3zubDP2DZpGf3HbFWwJ4E`),
Henesys (`2LFmzhHDKxkreihEtPanVmofuFn63bsh8twnRXEbDhBtCJxURB`),
Loyal (`2rpeyPqn1Th7WsebmecpHgQEvFHWq3vzrcRi9vKgMc32rTjUKX`),
north-investments (`289ecuoAnBmyZupLLN9DZWxCAFau8TSUh9YkkGwBYCAxxfByJw`),
Orange (`AowVWzuCAyLrTPSLHHaHeMJSdxRhhWbvmEuXncPnKv1hT5wf4`),
QR1020Q1MP (`w79fSBBwXNtbhaTVxurSTt8x1ipcDPrtKS6ZcEX5p8DmyLNKk`),
technical-ivory-gopher (`BLA3Z8uLTfAgPCnYEKiH7aBAVSwqLH4YqNcf2icaTdwg5voa2`),
The Grotto (`axF6i526AKGeduJeA6PQ8ewGQhtLoy2VFgxR9fknWnpf8iiFD`),
TixChain (`Bb5XKrQoBT6ypFmFUppAqfSmKCqcqYwjGCn3hHS6gibRTjiDj`),
tomatochain (`2PXzkiiy7zhXsPBDkhW2kwMvjeHk8kDXKbv5A6G3SjroMVxfkT`),
Turing Chain (`p3eNXxT7Myp9jLrRZvj4rzTHDQMvQVJsYLknWhcXgqSU3Dwdi`).

Fuji: aurorapay-dvp-dev (`2my13nX2M4yMHDoHdtnAiRBEAPFFuckp3Af31sr6mexEevU18P`),
coloured-lavender-blackbird (`2g7668m91DYSLFBpCAjxrbLtFUALp2Jcn4HGbetKSkC3JE5oyD`),
Dinari Paper Network (`2Yz62sy7E1ptSw45s8wYz46XtGKofsoPqnsPEqW9oUkERD2LC`),
dt1026t1td (`2Hg3rGpM5qbD3WoRCp748qHpra6cVjZEp7dy6uMG2JFAuBcE1D`),
etotest (`2br9miLmzsiyfeSeAYi7hShb7nwV3PzFXPaZJ2uESft6hMfjjq`),
impressed-harlequin-shark (`2QU8aavp4LRMzaRSGDYKSHfzNyF8QrGHYEFZLrNUD13bdqYd3L`),
iris-testnet (`269JyvJZP33Lq9BA8DiNJCmqde5RgtUrpzEq2PbvLPPNv1zjKq`),
jelly (`27i5Wk1SRtfh6p1aLBNaCTenr9ELfahZDAzsaoKebz3CgAAJzZ`),
LifeAI testnet v1 (`hasgwAhYjddyh8u7TQ6B5V6yFgfz78YvyiWDpd2JAvpJwFfq9`),
lifeai-test (`28L5yUmRk3mxJ2hNyRt2G3hQGiR7ZnhdFRsXpmzgojKBm9nVV9`),
LTmax (`2EVX5Bd71LRNVUgvyuvh3dcrU2atPwKJFTkBJmqs3mJqevqsiF`),
MixA Testnet (`2SbpzYaSco9CztrNxfG84xXr1YKGXiRh5DaBZAGAV5bfrgszeU`),
modex (`2tCocn4cmdz6ZNAUhasixpQWko9F9oRrcfvDeUZCPkkgRCUzS5`),
mon (`2SbdFFeBVfR8iyYTq5dzMmUVtfEsZmW23yF1XmB1yK7QFdrNa`),
monthly-gold-poc (`hXKXRCmdaG3xW2Vevr7RjcvEmmc6bm7ERbbfR9Um2PFAkDxnv`),
northasset (`xjZvDBY8zo2FpkSzobPjTLL8HZyE6h1M2vrDbFMytn2wNkNkY`),
ozeki (`4Ws8L2pxS4Ya7n8S43sE5ypoptDAQJWQuxjFahLaZfBtm5tnP`),
QR0203Q2TS (`fvPsEin7WcCXpGRUdwSY3WunLVuSgUE31ipXbHF3r3zat4gbq`),
QR0213Q1TS (`22GJNfCB39E2n4pnCd2X98CmxKXTYyubmPVRWsFQWXTM1mVAeF`),
rm-gasless-relayer-test (`2VBTFzU4zQ1ZsRrnXDPpnNkwHYYmkgun64rV6zXhQk3hcWXP24`),
senior-purple-gopher (`wfapNnyaAnWpPESnzRtVA6PinAnZLWqSVZLKd4y3hCG8Sn5wj`),
Soshi (`2sDUH9VzjQJdALDUCQcnYxC7hfW5awJC3mZNN4ySFG52Lq4GCa`),
sushi-tempura (`2bQaaqGG5DZvJMBWnRe9VC5sFCz2QY5GntGzCuHrr2oqcEwnHq`),
swift-amber-heron (`2NkpPCyY6iQexE1NzJsc6s6Yf8L9T2KLoVXAiEnswwdN4V9soq`),
Titan L1 Testnet (`Hb3EduWUYuKWvENKL6NYqjZFYRMXriBfhaSojT7dSn7SjQZzU`),
Watr (`2ZZiR6T2sJjebQguABb53rRpzme8zfK4R9zt5vMM8MX1oUm3g`).

## Could not check

These chains have no public RPC that answers `eth_getChainConfig`:

- Held file, not verified: dfk, lamina1, numine, space, step (see table).
- Listed by Glacier, no file and no answer (HTTP 405): Blockticity mainnet
  (`2pV5K35V1ohNSYhRU3z1Bud2oZys9bak7QYcmvbVvnieh4FJXc`, 28530),
  bored-red-manatee Fuji (`gCZXKpKLTqdek4eZbaxTUVbeFXGWys81Z76fmrTjwMF43nA6N`, 20744).

Chains outside the Glacier list (for example other P-chain L1s with no public
RPC) are not covered unless they appear in the table.
