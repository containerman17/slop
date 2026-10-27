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
`https://glacier-api.avax.network/v1/chains?network=mainnet` (field `rpcUrl`),
from Glacier's RPC proxy `https://glacier-api.avax.network/v1/ext/bc/<blockchainID>/rpc`
(it answers from Ava Labs' own node, so its `upgrades` are that node's
`upgrade.json`), or from project and validator RPCs.
The comparison ignores address case and zero-value fields that the RPC adds.
"held" means the file came from our own node deployments or backups; "live"
means the file is `result.upgrades` written verbatim.

| network | name | blockchainID | evmChainId | stateUpgrades | source | live check |
|---|---|---|---|---|---|---|
| mainnet | aibtrust | `2FUHgWJcZ4j8FrEi1DsdGyq6vMWQXeQGbZLuzcU6sFAazvnrYd` | 80000 | no | live (Glacier proxy) | MATCH |
| mainnet | beam | `2tmrrBo1Lgt1mzzvPSFt73kkQKFas5d1AP88tv9cicwoFp8BSn` | 4337 | no | live | held file was STALE, replaced by live |
| mainnet | dexalot | `21Ths5Afqi5r4PaoV8r8cruGZWhN11y5rxvy89K8px7pKy3P8E` | 432204 | no | live | held file was STALE, replaced by live |
| mainnet | dfk | `q2aTwKuyzgs8pynF7UXBZCU7DejbZbZ6EUyHr3JQzYgwNPUPi` | 53935 | yes | live 2026-08-06 | NO-RPC now (405) |
| mainnet | gunz | `2M47TxWHGnhNtq6pM5zPXdATBtuqubxn5EPFgFmEawCQr9WFML` | 43419 | no | live | MATCH |
| mainnet | hashfire | `2vcBHFK7jiv74dyYeUCUiCsKy2jbWx6xEQzNfQPiLmYidBaDfX` | 7272 | no | held | MATCH |
| mainnet | innovo | `vFHT9J4F6PhqCTMmqkKKUKW3yfsjd6tNvhmcK4MkFa3Pmnnqe` | 10036 | yes | held | MATCH |
| mainnet | intainmarkets | `m4xWma3wWpxHAAKWyDXX6oqvzKUkqCXnTJTwKmTkReJpcEuS6` | 151122 | yes | live (Glacier proxy) | MATCH |
| mainnet | lamina1 | `UhReZTXT8Cqsjat9ghRtCe5kBQPQexQB5zG5Fvf3egrdYfyoJ` | 10849 | no | held, from the lamina1-integration README (2025-02) | NO-RPC (405) |
| mainnet | mugen | `2GiVhBPR19ZaHYQivmheYv8RkEBjfdu7AyDvSocbWWjR4gz1qf` | 29111 | yes | live (Glacier proxy) | MATCH |
| mainnet | numbers | `2PDRxzc6jMbZSTLb3sufkVszgQc2jtDnYZGtDTAAfom1CTwPsE` | 10507 | no | held | MATCH (mainnetrpc.num.network) |
| mainnet | numine | `2c1BN4N9qEhNmW4yCpeLK24SfjFQLyS1Z7FtgRDaYxZWFUUKxf` | 8021 | no | held | NO-RPC (405); matches the config our epochdb archive node synced to tip with |
| mainnet | pandasea | `2gYSmRb7tjS4wb5FHFx8NBhwQgARWEdr8MGVC7v8UWJJFkEpTj` | 7776 | no | held | MATCH (rpc-mainnet-pandasea-rl.cogitus.io) |
| mainnet | playa3ull | `k2SFEZ2MZr9UGXiycnA1DdaLqZTKDaHK7WUXVLhJk5F9DD8r1` | 3011 | yes | held | MATCH |
| mainnet | space | `22CN6x5LAPEkvLDdz4UwG3XXtZV69Su3bcspiYtkF9k5f9rcCt` | 8227 | no | held | NO-RPC (405); matches the config our epochdb archive node synced to tip with |
| mainnet | step | `2jRZvKtXY5nyWTqRwFh1KMHGrCRxJoULu4r2CsayWRnjdDGbV1` | 1234 | no | held | NO-RPC (rpc.step.network does not answer) |
| mainnet | straitsx | `EJ4DyXHe4ydhsLLMiDPsHtoq5RDqgyao6Lwb9znKhs59q4NQx` | 5566 | yes | held | MATCH |
| mainnet | titan | `24gRTyzFmXUPebVwzQHuwU6RFeKHHDxRftRVaSAe7MHRXWxZ4S` | 84358 | yes | held | MATCH |
| mainnet | uptn | `2tsFdSeemtqK9vaPgjPrcwvKFKPw1T6PHufaZ26tKbAFSkXPTE` | 6119 | yes | held | MATCH |
| fuji | apertum | `V8YHL6yZV62tx2MdLi98wjAY2Qn6sP7zdiEKX5bvxj515vhK7` | 89898 | no | live | MATCH |
| fuji | beam | `y97omoP2cSyEVfdSztQHXD9EnfnVP9YKjZwAxhUfGbLAPYT9t` | 13337 | yes | live | MATCH |
| fuji | dexalot | `XuEPnCE59rtutASDPCDeYw8geQaGWwteWjkDXYLWvssfuirde` | 432201 | no | live | MATCH |
| fuji | dispatch | `2D8RG4UpSXbPbvPCAWppNJyqTG2i2CAXSkTgmTBBvs7GKNZjsY` | 779672 | no | live | MATCH |
| fuji | echo | `98qnjenm7MBd8G2cPZoRvZrgJC33JGSAAKghsQ6eojbLCeRNp` | 173750 | yes | live | MATCH |
| fuji | egmtester | `Buqpg8qoENGbZEC1LY1ngkHYgS5ftZ8M22QCf3e7DWbc3yRDM` | 55197 | yes | live | MATCH |
| fuji | fscfive | `zYh1CWZEBF8bt8qW3Stg2t8WnqXjKyHwTo3S92swbvDp8SFvr` | 888888 | no | live (validator node) | MATCH |
| fuji | gunz | `ryk9vkvNuKtewME2PeCgybo9sdWXGmCkBrrx4VPuZPdVdAak8` | 49321 | no | live | MATCH |
| fuji | intainmarkets | `2ALtzRYgRpRWnTgjdrMArkMvU6RTpcjs7VWmupqYaPrHDrHLSd` | 101122 | no | live (validator node) | MATCH |
| fuji | numine | `26XPa2jA4T31hKmaUGJt3g32xwsbaUfvcnMx1m18dWmrgxV2Mw` | 8022 | yes | live (validator node) | MATCH |
| fuji | providenceone | `2c244m6Vhz9RmMzKUuHQzytBmkdkoi92M5S742cGmjpx6voeLk` | 23030323 | no | live (validator node) | MATCH |
| fuji | prvl1 | `VDSkkiU59Sg9wkMuRgifpP95KS2DbUNRNzvA4qDL9v6hHTXqo` | 7888 | no | live (validator node) | MATCH, head block 2025-12-16 |
| fuji | rizenet | `gs51JsazmyXrsFHL9dWUu1wPT9wgFt8BhLBFBLzNHkTkL4weS` | 79123 | no | live (validator node) | MATCH |
| fuji | tixchain | `rbPy3vFTgtGccWouLkCXqgDTxyEHEpam2kA24fZUmjgKMNcXX` | 723107 | yes | live | MATCH |
| fuji | tkintl-alpha-csl-ava-staging | `2Huy194qfidjYSAXrVGzwa8HMEXtKSxB55ja5yjfQGYbc4XqZS` | 131076 | no | live | MATCH |
| fuji | uptn | `2Pk23ZAG5gkQ4DobkRsmwW3NaCwR46zxsDv5EY2GViGL6ow18a` | 6118 | yes | live | MATCH |

Stale held files: beam and dexalot mainnet held files lacked
`networkUpgradeOverrides.heliconTimestamp: 1790100120` (2026-09-23 03:02 JST);
everything else was equal.

PandaSea's file uses `txBlockListConfig`, a precompile that exists only in
PandaSea's subnet-evm fork. Stock subnet-evm rejects this file. The same holds
for Fuji fscfive (`nettingConfig`, `privacyConfig`, `qedisIdentityConfig`).

## No file needed

`eth_getChainConfig` returns empty `upgrades` for these chains, so they need no
`upgrade.json` (an empty `{}` file is not the same as no file, so do not write one).

Mainnet: Blaze (`2SfshBPqRJexmqdWBY2xYSTq2Rp2dDisuax7nQB6GyNqPjSWWY`),
BnryMainnet (`J3MYb3rDARLmB7FrRybinyjKqVTqmerbCr9bAXDatrSaHiLxQ`),
ChamonixNetwork (`v8FsmtYmWSE3dozTVr9oQaHDP9uTrrNYm4nREouba9fmjKJEJ`),
Dinari Financial Network (`C5GXnNHNEfDxUydtjZ5B5J6XspdAZJGLsX3u4wxCXZgE3N4Gq`),
Elysium (`3jVp8Ee4CarD5Sua3qDPwNGNRCEetSSGToWcwzwkKeHbbChcL`),
even (`2YnwDMW9shSuV6nuYpAhm3DE2KvYJP2GFDvUde6XG8jE5gxvpL`),
FIFA Blockchain (`SUDoK9P89PCcguskyof41fZexw7U3zubDP2DZpGf3HbFWwJ4E`),
fortitudemadeira (`2C1D31mrPpAaSkseb3iywafDvH5drvMJVKh1xZv3tmYWrbMkEf`),
Frqtal (`2jQuqy3RpCmVQHZCdpz9SJD92yP5VL4GgKBGeGJPYJxm9vkNNH`),
goldensilenthawk (`2HvYVUizFgTaRAxGCJdKdZVdNgRRXoUGXuuJhZUPmeywNjBjvp`),
goldleader (`2PDZJhEoUr5944QGTDpztZvCDiWHa3iGbXenTmpqq3xQcqiW3A`),
Henesys (`2LFmzhHDKxkreihEtPanVmofuFn63bsh8twnRXEbDhBtCJxURB`),
kite (`3USaEfTcoUhHxpKXvpAG916UKCUEyjrtkg2hBArBG3JyDP7my`),
Loyal (`2rpeyPqn1Th7WsebmecpHgQEvFHWq3vzrcRi9vKgMc32rTjUKX`),
MakaChain (`2TfGh1pyDoumsDgTxDmWoseRD7GGRwrpX19HLsQdeCRqwK9BqZ`),
north-investments (`289ecuoAnBmyZupLLN9DZWxCAFau8TSUh9YkkGwBYCAxxfByJw`),
Orange (`AowVWzuCAyLrTPSLHHaHeMJSdxRhhWbvmEuXncPnKv1hT5wf4`),
QR1020Q1MP (`w79fSBBwXNtbhaTVxurSTt8x1ipcDPrtKS6ZcEX5p8DmyLNKk`),
technical-ivory-gopher (`BLA3Z8uLTfAgPCnYEKiH7aBAVSwqLH4YqNcf2icaTdwg5voa2`),
The Grotto (`axF6i526AKGeduJeA6PQ8ewGQhtLoy2VFgxR9fknWnpf8iiFD`),
TixChain (`Bb5XKrQoBT6ypFmFUppAqfSmKCqcqYwjGCn3hHS6gibRTjiDj`),
tomatochain (`2PXzkiiy7zhXsPBDkhW2kwMvjeHk8kDXKbv5A6G3SjroMVxfkT`),
Turing Chain (`p3eNXxT7Myp9jLrRZvj4rzTHDQMvQVJsYLknWhcXgqSU3Dwdi`).

Fuji: 41 chains, listed with the RPC that answered in
[`fuji-active-chains-2026-10-27.txt`](fuji-active-chains-2026-10-27.txt) (rows marked `EMPTY`).

## Could not check

These chains have no public RPC that answers `eth_getChainConfig`:

- Held file, not verified: dfk, lamina1, numine, space, step (see table).
- Mainnet, no file and no answering RPC: Blockticity
  (`2pV5K35V1ohNSYhRU3z1Bud2oZys9bak7QYcmvbVvnieh4FJXc`, 28530),
  STLINK (`2HR9yTVXBwxJjmejn9yt1UQnrBpQVQRpxUQe6YUtEaWh9yqnPN`, 35299; its
  genesis has the same year-9999 Etna as aibtrust and mugen, so it probably
  needs an Etna override too), Athera (`LEJHppkiGCMMrTyH8y6CGSuWaB6g3ZN6TVqJ9WUrTwwUNiz6N`),
  CodeNekt (`ZG7cT4B1u3y7piZ9CzfejnTKnNAoehcifbJWUwBqgyD3RuEqK`),
  DOS Chain (`22v7AG7h6qaVxd4bLvAsSsg2LZ4RCn5iVYgFn7a2Fj1LCuYwjv`),
  Frqtal (`2FJM6WqyDGX2PZYipnatYXuXsNcKLio5fJMkEaGgpP4kHDTPw2`),
  Lynq 01 (`BXLoBWScGdbWcFozx2PDv9hEcH3fXNvNmX3GAc25gbbRxs7nc`, private),
  blerifymain (`2aVPq3NSjjf8fqQE68myUM3AiTQLM9hBHg1XkBwByfTWma3BHv`),
  ParadeA (`q41H38p3LxuGfz1ZLBvN7rSzDx8deQdmpLDY4HU7K2SYiK7oe`).
- Mainnet, registered but with no validator that has a positive balance (halted):
  ParadeA (`GRPDgKqXDuxEy129BHrLW1aw9kv51stHBkgTA3cKik3pnaenV`,
  `V5PeDXYsRb8g33R8qivUZJV59Ybwx44b3nfqFKdRw5e9WQrRV`),
  mjms (`4s6dn5ToLrL9VQGwiuwrGjW2CgiRSvJRiU6zQcPJnjeT9jVWj`).
- Fuji: 104 of the 161 active Subnet-EVM chains, rows marked `UNKNOWN` in
  [`fuji-active-chains-2026-10-27.txt`](fuji-active-chains-2026-10-27.txt).
  19 of them are AvaCloud chains whose Glacier RPC returns `Forbidden`, among them
  the Fuji twins of mainnet chains that need a file (aibtrust, mugen, innovo).
  Others are known projects with dead testnet RPCs (pandasea, dfk, numbers
  `captevm`, stlink, bnry) and many throwaway test chains.

## Which chains are covered

Mainnet: the 65 L1s with active validators in a P-chain census of 2026-09-13
(height 25,494,732), all Subnet-EVM. Fuji: the 161 Subnet-EVM chains whose subnet
had an active validator at P-chain height 303,871 (2026-10-27 15:13 JST), found
with `platform.getAllValidatorsAt` and checked with `platform.getCurrentValidators`
(an L1 validator counts only with a positive balance). Fuji bored-red-manatee
(`gCZXKpKLTqdek4eZbaxTUVbeFXGWys81Z76fmrTjwMF43nA6N`) has no validator with a
balance, so it is not active.
