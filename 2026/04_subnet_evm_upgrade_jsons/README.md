# subnet-evm upgrade.json files for live Avalanche L1s

These are the subnet-evm `upgrade.json` files a node must run today for the
mainnet and Fuji L1s. A node reads the file from
`<chain-config-dir>/<blockchainID>/upgrade.json`; the file name here is
`<network>/<name>-<blockchainID>.json`, so copy it to that path. A node without
the file it needs fails at the first activation block with `invalid merkle root`.

## How each chain was checked

All checks ran on 2026-10-27 JST. For each chain we called `eth_getChainConfig`
and read `result.upgrades`. This is the parsed `UpgradeConfig`, the same object
`upgrade.json` unmarshals into, so a non-empty value written verbatim is the
file. An answering node that follows the chain tip has the same state root as
the validators, so its config is correct for every activation up to now.

The `upgrade.json` column has three values:

- **file**: the chain needs this file. The live config equals it.
- **empty, no file needed**: the live config is `{}`. Do not write an empty
  `{}` file; no file is the correct setup.
- **no result**: no RPC answers, so we do not know.

Where the answer came from:

- **public RPC**: the chain's own RPC (`subnets.avax.network` is AvaCloud).
- **Glacier proxy**: `https://glacier-api.avax.network/v1/ext/bc/<blockchainID>/rpc`,
  which forwards to a node that Ava Labs runs.
- **validator node**: the RPC port of a validator of the chain.
- **Builder Hub managed node**: `https://managed-nodes.avax.network/ext/bc/<blockchainID>/rpc`.
- **our archive node at tip**: our own node that follows the chain.

Which chains: every Subnet-EVM chain whose subnet has an active validator,
found with `platform.getAllValidatorsAt` and checked with
`platform.getCurrentValidators` (an L1 validator counts only with a positive
balance). Mainnet: P-chain height 25,758,109 (2026-10-27 23:05 JST), 71 of 74
active chains (Relation, Consequence and Matter run another VM). Fuji: P-chain
height 303,871 (2026-10-27 15:13 JST), 161 of 167 active chains.

## Mainnet

71 chains: 17 need a file, 43 empty (no file needed), 11 no result.

| name | blockchainID | EVM chain ID | upgrade.json | how it was verified |
|---|---|---|---|---|
| AIBTRUSTMainnet | `2FUHgWJcZ4j8FrEi1DsdGyq6vMWQXeQGbZLuzcU6sFAazvnrYd` | 80000 | [file](mainnet/aibtrust-2FUHgWJcZ4j8FrEi1DsdGyq6vMWQXeQGbZLuzcU6sFAazvnrYd.json) | live config equals file (Glacier proxy) |
| beam | `2tmrrBo1Lgt1mzzvPSFt73kkQKFas5d1AP88tv9cicwoFp8BSn` | 4337 | [file](mainnet/beam-2tmrrBo1Lgt1mzzvPSFt73kkQKFas5d1AP88tv9cicwoFp8BSn.json) | live config equals file (public RPC `subnets.avax.network`) |
| dexalotevm | `21Ths5Afqi5r4PaoV8r8cruGZWhN11y5rxvy89K8px7pKy3P8E` | 432204 | [file](mainnet/dexalot-21Ths5Afqi5r4PaoV8r8cruGZWhN11y5rxvy89K8px7pKy3P8E.json) | live config equals file (public RPC `subnets.avax.network`) |
| DFK Chain | `q2aTwKuyzgs8pynF7UXBZCU7DejbZbZ6EUyHr3JQzYgwNPUPi` | 53935 | [file](mainnet/dfk-q2aTwKuyzgs8pynF7UXBZCU7DejbZbZ6EUyHr3JQzYgwNPUPi.json) (not verified) | held file, no RPC answers; equal to live config on 2026-08-06 |
| gunz | `2M47TxWHGnhNtq6pM5zPXdATBtuqubxn5EPFgFmEawCQr9WFML` | 43419 | [file](mainnet/gunz-2M47TxWHGnhNtq6pM5zPXdATBtuqubxn5EPFgFmEawCQr9WFML.json) | live config equals file (public RPC `subnets.avax.network`) |
| Hashfire | `2vcBHFK7jiv74dyYeUCUiCsKy2jbWx6xEQzNfQPiLmYidBaDfX` | 7272 | [file](mainnet/hashfire-2vcBHFK7jiv74dyYeUCUiCsKy2jbWx6xEQzNfQPiLmYidBaDfX.json) | live config equals file (public RPC `subnets.avax.network`) |
| InnovoMarketsMainnet | `vFHT9J4F6PhqCTMmqkKKUKW3yfsjd6tNvhmcK4MkFa3Pmnnqe` | 10036 | [file](mainnet/innovo-vFHT9J4F6PhqCTMmqkKKUKW3yfsjd6tNvhmcK4MkFa3Pmnnqe.json) | live config equals file (public RPC `subnets.avax.network`) |
| IntainMarkets | `m4xWma3wWpxHAAKWyDXX6oqvzKUkqCXnTJTwKmTkReJpcEuS6` | 151122 | [file](mainnet/intainmarkets-m4xWma3wWpxHAAKWyDXX6oqvzKUkqCXnTJTwKmTkReJpcEuS6.json) | live config equals file (Glacier proxy) |
| MUGEN | `2GiVhBPR19ZaHYQivmheYv8RkEBjfdu7AyDvSocbWWjR4gz1qf` | 29111 | [file](mainnet/mugen-2GiVhBPR19ZaHYQivmheYv8RkEBjfdu7AyDvSocbWWjR4gz1qf.json) | live config equals file (Glacier proxy) |
| numbersevm | `2PDRxzc6jMbZSTLb3sufkVszgQc2jtDnYZGtDTAAfom1CTwPsE` | 10507 | [file](mainnet/numbers-2PDRxzc6jMbZSTLb3sufkVszgQc2jtDnYZGtDTAAfom1CTwPsE.json) | live config equals file (public RPC `mainnetrpc.num.network`) |
| NUMINEMainnet | `2c1BN4N9qEhNmW4yCpeLK24SfjFQLyS1Z7FtgRDaYxZWFUUKxf` | 8021 | [file](mainnet/numine-2c1BN4N9qEhNmW4yCpeLK24SfjFQLyS1Z7FtgRDaYxZWFUUKxf.json) | live config equals file (our archive node at tip) |
| PandaSea | `2gYSmRb7tjS4wb5FHFx8NBhwQgARWEdr8MGVC7v8UWJJFkEpTj` | 7776 | [file](mainnet/pandasea-2gYSmRb7tjS4wb5FHFx8NBhwQgARWEdr8MGVC7v8UWJJFkEpTj.json) | live config equals file (public RPC `rpc-mainnet-pandasea-rl.cogitus.io`) |
| PLAYA3ULL | `k2SFEZ2MZr9UGXiycnA1DdaLqZTKDaHK7WUXVLhJk5F9DD8r1` | 3011 | [file](mainnet/playa3ull-k2SFEZ2MZr9UGXiycnA1DdaLqZTKDaHK7WUXVLhJk5F9DD8r1.json) | live config equals file (public RPC `subnets.avax.network`) |
| StepNetwork | `2jRZvKtXY5nyWTqRwFh1KMHGrCRxJoULu4r2CsayWRnjdDGbV1` | 1234 | [file](mainnet/step-2jRZvKtXY5nyWTqRwFh1KMHGrCRxJoULu4r2CsayWRnjdDGbV1.json) (not verified) | held file, no RPC answers; a stock node synced to tip with it (2026-09) |
| StraitsX | `EJ4DyXHe4ydhsLLMiDPsHtoq5RDqgyao6Lwb9znKhs59q4NQx` | 5566 | [file](mainnet/straitsx-EJ4DyXHe4ydhsLLMiDPsHtoq5RDqgyao6Lwb9znKhs59q4NQx.json) | live config equals file (public RPC `subnets.avax.network`) |
| TITANL | `24gRTyzFmXUPebVwzQHuwU6RFeKHHDxRftRVaSAe7MHRXWxZ4S` | 84358 | [file](mainnet/titan-24gRTyzFmXUPebVwzQHuwU6RFeKHHDxRftRVaSAe7MHRXWxZ4S.json) | live config equals file (public RPC `subnets.avax.network`) |
| UPTN | `2tsFdSeemtqK9vaPgjPrcwvKFKPw1T6PHufaZ26tKbAFSkXPTE` | 6119 | [file](mainnet/uptn-2tsFdSeemtqK9vaPgjPrcwvKFKPw1T6PHufaZ26tKbAFSkXPTE.json) | live config equals file (Glacier proxy) |
| ApertumMainnet | `YDJ1r9RMkewATmA7B35q1bdV18aywzmdiXwd9zGBq3uQjsCnn` | 2786 | empty, no file needed | live config is `{}` (our archive node at tip) |
| arcadeunion | `2bg411HX8DoPrGJzwcTLGiWSxizcZ5XKSoGwUMfSnndWgiWrMd` | 242424 | empty, no file needed | live config is `{}` (public RPC `rpc.arcadeunion.com`) |
| Blaze | `2SfshBPqRJexmqdWBY2xYSTq2Rp2dDisuax7nQB6GyNqPjSWWY` | 46975 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| BnryMainnet | `J3MYb3rDARLmB7FrRybinyjKqVTqmerbCr9bAXDatrSaHiLxQ` | 836 | empty, no file needed | live config is `{}` (public RPC `rpc-binaryholdings.cogitus.io`) |
| ChamonixNetwork | `v8FsmtYmWSE3dozTVr9oQaHDP9uTrrNYm4nREouba9fmjKJEJ` | 74400 | empty, no file needed | live config is `{}` (Glacier proxy) |
| Datagram Chain | `RWfhP7moxu627ysqgaj4GoD3PZbdcJpAX56t6kHyuafSJGsNb` | 968 | empty, no file needed | live config is `{}` (our archive node at tip) |
| DComm | `2QJ6d1ue6UyXNXrMdGnELFc2AjMdMqs8YbX3sT3k4Nin2RcWSm` | 326663 | empty, no file needed | no RPC; a stock node synced to tip without a file (2026-09) |
| DEP Chain | `2WhgNQDsAmetGvuzYEn1sWR5tuANaAd2cCu5fpCi7bTfcWP3s9` | 79952 | empty, no file needed | live config is `{}` (our archive node at tip) |
| DinariFinancialNetwork | `C5GXnNHNEfDxUydtjZ5B5J6XspdAZJGLsX3u4wxCXZgE3N4Gq` | 202110 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| DOS Chain | `2ewKoUrSjnviEgGmeTiELHBmNjxVTVczBPowST471rYUZvA9bk` | 7979 | empty, no file needed | live config is `{}` (our archive node at tip) |
| Drama | `cW2yyntWNpRBbY3mtzZLdQEjYZfGopwomhyTjUPMj4gtA8z3o` | 710520 | empty, no file needed | live config is `{}` (public RPC `rpc.drama.center`) |
| Elysium | `3jVp8Ee4CarD5Sua3qDPwNGNRCEetSSGToWcwzwkKeHbbChcL` | 5782 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| even | `2YnwDMW9shSuV6nuYpAhm3DE2KvYJP2GFDvUde6XG8jE5gxvpL` | 33345 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| FIFABlockchain | `SUDoK9P89PCcguskyof41fZexw7U3zubDP2DZpGf3HbFWwJ4E` | 13322 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| fortitudemadeira | `2C1D31mrPpAaSkseb3iywafDvH5drvMJVKh1xZv3tmYWrbMkEf` | 94876 | empty, no file needed | live config is `{}` (Glacier proxy) |
| Frqtal | `2jQuqy3RpCmVQHZCdpz9SJD92yP5VL4GgKBGeGJPYJxm9vkNNH` | 62786 | empty, no file needed | live config is `{}` (public RPC `rpc.frqtal.org`) |
| goldensilenthawk | `2HvYVUizFgTaRAxGCJdKdZVdNgRRXoUGXuuJhZUPmeywNjBjvp` | 70467 | empty, no file needed | live config is `{}` (Glacier proxy) |
| goldleader | `2PDZJhEoUr5944QGTDpztZvCDiWHa3iGbXenTmpqq3xQcqiW3A` | 39158 | empty, no file needed | live config is `{}` (Glacier proxy) |
| HanChain | `2uG4mYNKeovqcq93znwnGBs6FiucrTYj7NiLFKasayThwVY4LJ` | 16888 | empty, no file needed | no RPC; a stock node synced to tip without a file (2026-09) |
| HanChain | `5ZcL3SHkihFtDi4iDXuRsMu3UskEe4A6uvaaTjgWVFpVQrJWj` | 726578 | empty, no file needed | no RPC; a stock node synced to tip without a file (2026-09) |
| HanChain | `9AT9wo2BYhB3xjFHnPjW9R8pyNcGqcgNRjuhrHWUvF9wiLXU8` | 726579 | empty, no file needed | live config is `{}` (validator node) |
| Henesys | `2LFmzhHDKxkreihEtPanVmofuFn63bsh8twnRXEbDhBtCJxURB` | 68414 | empty, no file needed | live config is `{}` (public RPC `henesys-rpc.msu.io`) |
| kite | `3USaEfTcoUhHxpKXvpAG916UKCUEyjrtkg2hBArBG3JyDP7my` | 2366 | empty, no file needed | live config is `{}` (public RPC `rpc.gokite.ai`) |
| Loyal | `2rpeyPqn1Th7WsebmecpHgQEvFHWq3vzrcRi9vKgMc32rTjUKX` | 72379 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| MakaChain | `2TfGh1pyDoumsDgTxDmWoseRD7GGRwrpX19HLsQdeCRqwK9BqZ` | 777178 | empty, no file needed | live config is `{}` (public RPC `rpc.makachain.io`) |
| nftchain | `2RyzsmGypNQZPby1miwMMV8spTvhgd9qd2peNRzU1mErUQqSSw` | 7773777 | empty, no file needed | live config is `{}` (our archive node at tip) |
| northinvestments | `289ecuoAnBmyZupLLN9DZWxCAFau8TSUh9YkkGwBYCAxxfByJw` | 122225 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| omnicoin | `B5gLCyeRWScxf1CeGHfXjEkcp5au8dQYQjtSV6cRammsLjsbK` | 88008 | empty, no file needed | live config is `{}` (our archive node at tip) |
| Orange | `AowVWzuCAyLrTPSLHHaHeMJSdxRhhWbvmEuXncPnKv1hT5wf4` | 1510 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| REXChain | `26hUagfWRE5QqtMtStwoggSvTY2AwRqUNRcaFEVm1ZNVCAqLqi` | 12393 | empty, no file needed | live config is `{}` (public RPC `rpc.rexchain.is`) |
| rwachain | `xFwrfjdyhZRFbMUbGY3TjxM58iNxwkRazHL91xxwwnhRuUi8L` | 196966569 | empty, no file needed | live config is `{}` (our archive node at tip) |
| smartchain | `2t51dXsuxUvd9teY9TKEJmgxmxMk3CRF88UYTA4HQgjeYZqzSX` | 7771777 | empty, no file needed | live config is `{}` (our archive node at tip) |
| technicalivorygopher | `BLA3Z8uLTfAgPCnYEKiH7aBAVSwqLH4YqNcf2icaTdwg5voa2` | 77974 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| TheGrotto | `axF6i526AKGeduJeA6PQ8ewGQhtLoy2VFgxR9fknWnpf8iiFD` | 36463 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| TixChain | `Bb5XKrQoBT6ypFmFUppAqfSmKCqcqYwjGCn3hHS6gibRTjiDj` | 13790 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| tomatochain | `2PXzkiiy7zhXsPBDkhW2kwMvjeHk8kDXKbv5A6G3SjroMVxfkT` | 77294 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| TSxBastion | `2btMkKMfyrDJQ7Khf4gYvcgtxSVjAjzjNhTgMaN3fSGj7F1bH4` | 454 | empty, no file needed | live config is `{}` (validator node) |
| TSxHorizon | `qoqDS2EGjEybQZZVUGnf38Rm4sMdxAvjikhysjoL84yd9pWka` | 455 | empty, no file needed | live config is `{}` (validator node) |
| TSxTestnet | `2eJJVa9rc7PF92phYg1nHTa9RyGv131KsCdgiutZQyY8dSAMZ1` | 445 | empty, no file needed | live config is `{}` (validator node) |
| TuringChain | `p3eNXxT7Myp9jLrRZvj4rzTHDQMvQVJsYLknWhcXgqSU3Dwdi` | 120623 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| vlxL1 | `zNXAbLSpaqfn3PTLi6ztbeomNtj4wsa7tBxSPyvXYEKstrtsR` | 90000 | empty, no file needed | live config is `{}` (our archive node at tip) |
| VYOChain | `2Ge2BkkjNupU2WwJ3SEbaoQGWhTb9HBK34RXqPbSTSzhnrqcJk` | 1771 | empty, no file needed | live config is `{}` (our archive node at tip) |
| Watr | `EypLFUSzC2wdbFJovYS3Af1E7ch1DJf7KxKoGR5QFPErxQkG1` | 192 | empty, no file needed | live config is `{}` (our archive node at tip) |
| Athera | `LEJHppkiGCMMrTyH8y6CGSuWaB6g3ZN6TVqJ9WUrTwwUNiz6N` | 1517 | no result | no RPC answers |
| blerifymain | `2aVPq3NSjjf8fqQE68myUM3AiTQLM9hBHg1XkBwByfTWma3BHv` | 42902 | no result | no RPC answers |
| Blockticity | `2pV5K35V1ohNSYhRU3z1Bud2oZys9bak7QYcmvbVvnieh4FJXc` | 28530 | no result | no RPC answers |
| CodeNekt | `ZG7cT4B1u3y7piZ9CzfejnTKnNAoehcifbJWUwBqgyD3RuEqK` | 235235 | no result | no RPC answers |
| DOS Chain | `22v7AG7h6qaVxd4bLvAsSsg2LZ4RCn5iVYgFn7a2Fj1LCuYwjv` | 7979 | no result | no RPC answers |
| Frqtal | `2FJM6WqyDGX2PZYipnatYXuXsNcKLio5fJMkEaGgpP4kHDTPw2` | 62786 | no result | no RPC answers |
| Lynq 01 | `BXLoBWScGdbWcFozx2PDv9hEcH3fXNvNmX3GAc25gbbRxs7nc` | 93134 | no result | no RPC answers |
| OVI | `2o7i6nc6Y4QWndwH6VRnDL18f4pMr3iKqrBiF5fuSmjKAUr4dg` | 91820 | no result | no RPC answers |
| ParadeA | `q41H38p3LxuGfz1ZLBvN7rSzDx8deQdmpLDY4HU7K2SYiK7oe` | 43113778 | no result | no RPC answers |
| settlement | `2grpF11sCe1QT7PDVgRtc5uq3JQrL6tthennwZG6sMxJ3TrSLP` | 43117 | no result | no RPC answers |
| STLINK | `2HR9yTVXBwxJjmejn9yt1UQnrBpQVQRpxUQe6YUtEaWh9yqnPN` | 35299 | no result | no RPC answers |

## Fuji

161 chains: 27 need a file, 53 empty (no file needed), 81 no result.

| name | blockchainID | EVM chain ID | upgrade.json | how it was verified |
|---|---|---|---|---|
| AIBTRUSTTestnet | `S85PT12J9jAvUS5VuZoTSWZx6wVDU2rNmtQWws3oUutzxpKnv` | 79903 | [file](fuji/aibtrust-S85PT12J9jAvUS5VuZoTSWZx6wVDU2rNmtQWws3oUutzxpKnv.json) | live config equals file (Glacier proxy) |
| ApertumTestnet | `V8YHL6yZV62tx2MdLi98wjAY2Qn6sP7zdiEKX5bvxj515vhK7` | 89898 | [file](fuji/apertum-V8YHL6yZV62tx2MdLi98wjAY2Qn6sP7zdiEKX5bvxj515vhK7.json) | live config equals file (public RPC `rpc-testnet.apertum.io`) |
| beam | `y97omoP2cSyEVfdSztQHXD9EnfnVP9YKjZwAxhUfGbLAPYT9t` | 13337 | [file](fuji/beam-y97omoP2cSyEVfdSztQHXD9EnfnVP9YKjZwAxhUfGbLAPYT9t.json) | live config equals file (public RPC `subnets.avax.network`) |
| dispatch | `2D8RG4UpSXbPbvPCAWppNJyqTG2i2CAXSkTgmTBBvs7GKNZjsY` | 779672 | [file](fuji/dispatch-2D8RG4UpSXbPbvPCAWppNJyqTG2i2CAXSkTgmTBBvs7GKNZjsY.json) | live config equals file (public RPC `subnets.avax.network`) |
| echo | `98qnjenm7MBd8G2cPZoRvZrgJC33JGSAAKghsQ6eojbLCeRNp` | 173750 | [file](fuji/echo-98qnjenm7MBd8G2cPZoRvZrgJC33JGSAAKghsQ6eojbLCeRNp.json) | live config equals file (public RPC `subnets.avax.network`) |
| EGMTester | `Buqpg8qoENGbZEC1LY1ngkHYgS5ftZ8M22QCf3e7DWbc3yRDM` | 55197 | [file](fuji/egmtester-Buqpg8qoENGbZEC1LY1ngkHYgS5ftZ8M22QCf3e7DWbc3yRDM.json) | live config equals file (public RPC `subnets.avax.network`) |
| fscfive | `zYh1CWZEBF8bt8qW3Stg2t8WnqXjKyHwTo3S92swbvDp8SFvr` | 888888 | [file](fuji/fscfive-zYh1CWZEBF8bt8qW3Stg2t8WnqXjKyHwTo3S92swbvDp8SFvr.json) | live config equals file (validator node) |
| gunzillaevm | `ryk9vkvNuKtewME2PeCgybo9sdWXGmCkBrrx4VPuZPdVdAak8` | 49321 | [file](fuji/gunz-ryk9vkvNuKtewME2PeCgybo9sdWXGmCkBrrx4VPuZPdVdAak8.json) | live config equals file (public RPC `rpc.gunz.dev`) |
| InnovoMarketsTestnet | `2nLc1UT7yUqzVcZeKcBzZVpH1kmkedY3LRinX8nAkypxRtwAaF` | 54414 | [file](fuji/innovo-2nLc1UT7yUqzVcZeKcBzZVpH1kmkedY3LRinX8nAkypxRtwAaF.json) | live config equals file (Glacier proxy) |
| IntainMarketsTest | `2ALtzRYgRpRWnTgjdrMArkMvU6RTpcjs7VWmupqYaPrHDrHLSd` | 101122 | [file](fuji/intainmarkets-2ALtzRYgRpRWnTgjdrMArkMvU6RTpcjs7VWmupqYaPrHDrHLSd.json) | live config equals file (validator node) |
| JONO122 | `2XvEdm5FGHcFWAp6bGookz8BB9uo4mcVyfGf1LALogYkvMfYXd` | 5668 | [file](fuji/jono122-2XvEdm5FGHcFWAp6bGookz8BB9uo4mcVyfGf1LALogYkvMfYXd.json) | live config equals file (Glacier proxy) |
| jono13 | `2es7oPnyjrMCNseCbi2429dbQaCAkp5DhTfERr4gGEKmMEcuko` | 595581 | [file](fuji/jono13-2es7oPnyjrMCNseCbi2429dbQaCAkp5DhTfERr4gGEKmMEcuko.json) | live config equals file (Glacier proxy) |
| jono14 | `wUMSB5vY7VdocbaHUi8SG5DSDcdv14Yt824p12m9Lqv5a551F` | 807424 | [file](fuji/jono14-wUMSB5vY7VdocbaHUi8SG5DSDcdv14Yt824p12m9Lqv5a551F.json) | live config equals file (Glacier proxy) |
| jonostage | `5jXGo1pToYqbY6SU3c6SCzzSLA18MXiMxNhWQSxdoJ1xtvooV` | 847799 | [file](fuji/jonostage-5jXGo1pToYqbY6SU3c6SCzzSLA18MXiMxNhWQSxdoJ1xtvooV.json) | live config equals file (Glacier proxy) |
| liquidfitest | `2VqFWVQdfEysKCUKHpAKJjsaVd8VcYXCiZC2bbEHpKDQWTen3C` | 989898 | [file](fuji/liquidfitest-2VqFWVQdfEysKCUKHpAKJjsaVd8VcYXCiZC2bbEHpKDQWTen3C.json) | live config equals file (Glacier proxy) |
| main | `XuEPnCE59rtutASDPCDeYw8geQaGWwteWjkDXYLWvssfuirde` | 432201 | [file](fuji/dexalot-XuEPnCE59rtutASDPCDeYw8geQaGWwteWjkDXYLWvssfuirde.json) | live config equals file (public RPC `subnets.avax.network`) |
| MTPDev | `2qAgn5cGvNsDnUkfW5ctG5KFy3DfPhyQuEJ4KtzkkxSSfaCick` | 95784 | [file](fuji/mtpdev-2qAgn5cGvNsDnUkfW5ctG5KFy3DfPhyQuEJ4KtzkkxSSfaCick.json) | live config equals file (Glacier proxy) |
| MUGENTestnet | `YY8TTyP4tePUPf2TLkBZcR5hW4vYNMmNAshivAdaC9wTBJpzk` | 30406 | [file](fuji/mugen-YY8TTyP4tePUPf2TLkBZcR5hW4vYNMmNAshivAdaC9wTBJpzk.json) | live config equals file (Glacier proxy) |
| numine | `26XPa2jA4T31hKmaUGJt3g32xwsbaUfvcnMx1m18dWmrgxV2Mw` | 8022 | [file](fuji/numine-26XPa2jA4T31hKmaUGJt3g32xwsbaUfvcnMx1m18dWmrgxV2Mw.json) | live config equals file (validator node) |
| ProvidenceOne | `2c244m6Vhz9RmMzKUuHQzytBmkdkoi92M5S742cGmjpx6voeLk` | 23030323 | [file](fuji/providenceone-2c244m6Vhz9RmMzKUuHQzytBmkdkoi92M5S742cGmjpx6voeLk.json) | live config equals file (validator node) |
| PrvL1 | `VDSkkiU59Sg9wkMuRgifpP95KS2DbUNRNzvA4qDL9v6hHTXqo` | 7888 | [file](fuji/prvl1-VDSkkiU59Sg9wkMuRgifpP95KS2DbUNRNzvA4qDL9v6hHTXqo.json) | live config equals file (validator node) |
| RizenetTestnet | `gs51JsazmyXrsFHL9dWUu1wPT9wgFt8BhLBFBLzNHkTkL4weS` | 79123 | [file](fuji/rizenet-gs51JsazmyXrsFHL9dWUu1wPT9wgFt8BhLBFBLzNHkTkL4weS.json) | live config equals file (validator node) |
| STLINKTIS | `2uooowuGgTW1MceKH8ijPE941TueBk25j4LAbVfTkhhWMUEQeD` | 57922 | [file](fuji/stlink-2uooowuGgTW1MceKH8ijPE941TueBk25j4LAbVfTkhhWMUEQeD.json) | live config equals file (Glacier proxy) |
| TESTNetFirstTEST | `2JQpYaTf3HoxfZA2h8aHEpTYFPJFpmW5KNwH8uMiUvadWAEMTH` | 7158 | [file](fuji/testnetfirsttest-2JQpYaTf3HoxfZA2h8aHEpTYFPJFpmW5KNwH8uMiUvadWAEMTH.json) | live config equals file (Glacier proxy) |
| TixChain | `rbPy3vFTgtGccWouLkCXqgDTxyEHEpam2kA24fZUmjgKMNcXX` | 723107 | [file](fuji/tixchain-rbPy3vFTgtGccWouLkCXqgDTxyEHEpam2kA24fZUmjgKMNcXX.json) | live config equals file (public RPC `subnets.avax.network`) |
| tkintlalphacslavastaging | `2Huy194qfidjYSAXrVGzwa8HMEXtKSxB55ja5yjfQGYbc4XqZS` | 131076 | [file](fuji/tkintl-alpha-csl-ava-staging-2Huy194qfidjYSAXrVGzwa8HMEXtKSxB55ja5yjfQGYbc4XqZS.json) | live config equals file (public RPC `subnets.avax.network`) |
| UPTNTEST | `2Pk23ZAG5gkQ4DobkRsmwW3NaCwR46zxsDv5EY2GViGL6ow18a` | 6118 | [file](fuji/uptn-2Pk23ZAG5gkQ4DobkRsmwW3NaCwR46zxsDv5EY2GViGL6ow18a.json) | live config equals file (Glacier proxy) |
| Akave Community | `56g16Hr1SHQRzdM8JLm3GKYv7APVHY8T2TyeZLvDVzCaTRS7W` | 21213 | empty, no file needed | live config is `{}` (validator node) |
| Alpine Summit | `E6r9QDbjyMyxkEoRJd3EZZyauHH8bnop1hEubWRHWSgZy3C9A` | 566839 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| aurorapaydvpdev | `2my13nX2M4yMHDoHdtnAiRBEAPFFuckp3Af31sr6mexEevU18P` | 39357 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| Aurum | `2JrFCfysXwtrNfCbNQjHtiwaNvNvU9co8oLdNUtUBue7yvGZu1` | 531 | empty, no file needed | live config is `{}` (Glacier proxy) |
| colouredlavenderblackbird | `2g7668m91DYSLFBpCAjxrbLtFUALp2Jcn4HGbetKSkC3JE5oyD` | 66850 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| criminalsalmontiger | `Hb3EduWUYuKWvENKL6NYqjZFYRMXriBfhaSojT7dSn7SjQZzU` | 85011 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| Crystal Meadow | `NXwA8hHPhbkmGbRFsVBXxP4xBZxXnR9gNA3eaFd7SNS3CJ5A8` | 348408 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| cyberia | `2uzoh7wVAuvyLKfTJ1bcZi4KCMRtogxXbK7Q7QGA1128QsnTYM` | 181811 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| cyberia | `yLf57F9pL13HbAf7fkKv6vGSkHs4C54jNLD6VgLJu5qKZfgpE` | 764881 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| DEP Chain Testnet | `Bp4P5VcMyD2gVbBYEUeGtuShfdNHz9KX4bc1rTAuk9bYyTWEW` | 20867 | empty, no file needed | live config is `{}` (validator node) |
| DinariPaperNetwork | `2Yz62sy7E1ptSw45s8wYz46XtGKofsoPqnsPEqW9oUkERD2LC` | 179205 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| DOS Testnet | `JASJZyVTWR7aviy4eY5yE8AVfdXtH33c1AinvzhLcVBARhcm9` | 3939 | empty, no file needed | live config is `{}` (public RPC `test.doschain.com`) |
| enigmathames | `UquKUwzetdduda7ej7swjo1uwRL3QcX8xb9bnRdYfbJCs2WEC` | 66583 | empty, no file needed | live config is `{}` (Glacier proxy) |
| etotest | `2br9miLmzsiyfeSeAYi7hShb7nwV3PzFXPaZJ2uESft6hMfjjq` | 2230 | empty, no file needed | live config is `{}` (public RPC `l1.avax.network`) |
| Frozen Pine | `JynEK91ofr4JYJY1DrEp2e53ihEigLT2LTUyMcgTDPaZwBAz9` | 332789 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| goldleader | `yJDDTFz737xMofDqtMHTgV41EnuFvN49GjkLfqAp1qPbf134s` | 73865 | empty, no file needed | live config is `{}` (Glacier proxy) |
| ICMTest | `2qHPRc4pKDDhBByYFxzxfTqYV5Z9QszfLPGBvyqmE2JD5rMwy5` | 444567 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| impressedharlequinshark | `2QU8aavp4LRMzaRSGDYKSHfzNyF8QrGHYEFZLrNUD13bdqYd3L` | 17070 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| iristestnet | `269JyvJZP33Lq9BA8DiNJCmqde5RgtUrpzEq2PbvLPPNv1zjKq` | 11632 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| jelly | `27i5Wk1SRtfh6p1aLBNaCTenr9ELfahZDAzsaoKebz3CgAAJzZ` | 99062 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| kiteaitestnet | `nQ9iPT7ZQQx1qwBTs68EGiU8cu1GHMMjzvYktmA3oRs4esg3V` | 2368 | empty, no file needed | live config is `{}` (public RPC `rpc-testnet.gokite.ai`) |
| lifeaitest | `28L5yUmRk3mxJ2hNyRt2G3hQGiR7ZnhdFRsXpmzgojKBm9nVV9` | 94909 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| LifeAItestnetv | `hasgwAhYjddyh8u7TQ6B5V6yFgfz78YvyiWDpd2JAvpJwFfq9` | 99991 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| LTmax | `2EVX5Bd71LRNVUgvyuvh3dcrU2atPwKJFTkBJmqs3mJqevqsiF` | 37558 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| Lumen Shore | `2nHTmoRqXRFrJR3dUkJGeCsb1EXGa48jsHKZsEFizvBrCLs26K` | 512250 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| MixATestnet | `2SbpzYaSco9CztrNxfG84xXr1YKGXiRh5DaBZAGAV5bfrgszeU` | 27777 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| modex | `2tCocn4cmdz6ZNAUhasixpQWko9F9oRrcfvDeUZCPkkgRCUzS5` | 52118 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| mon | `2SbdFFeBVfR8iyYTq5dzMmUVtfEsZmW23yF1XmB1yK7QFdrNa` | 77242 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| monthlygoldpoc | `hXKXRCmdaG3xW2Vevr7RjcvEmmc6bm7ERbbfR9Um2PFAkDxnv` | 56357 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| Natural Zipper Chain | `2poMrZuvkGiZh7n1cs11zqXJ8QdZATtJHdKaTqQQ43gYhUXGb1` | 74157 | empty, no file needed | live config is `{}` (validator node) |
| northasset | `xjZvDBY8zo2FpkSzobPjTLL8HZyE6h1M2vrDbFMytn2wNkNkY` | 88188 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| ozeki | `4Ws8L2pxS4Ya7n8S43sE5ypoptDAQJWQuxjFahLaZfBtm5tnP` | 4127 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| Prism Fjord | `2iw1nGgCktKQLAnJXwB7TtseyQRkmHgYPNZ1aBz7UTGHCn3MMU` | 641562 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| QRQTP | `2buASsMqVt5LevMeRYKgpV1xViYewcd2q3v9PY6ATqAKUv59AY` | 20491 | empty, no file needed | live config is `{}` (Glacier proxy) |
| QRQTP | `L29t2oSvEeP8pSubZehCwySGXY6MSGDVo61FD5mYHkbhBEf7w` | 39411 | empty, no file needed | live config is `{}` (Glacier proxy) |
| QRQTP | `L8eapB4NDYH6SLNPEnuiE5tG1pzYen6B4ckvphZqUHmVrKXBz` | 34384 | empty, no file needed | live config is `{}` (Glacier proxy) |
| QRQTS | `22GJNfCB39E2n4pnCd2X98CmxKXTYyubmPVRWsFQWXTM1mVAeF` | 49925 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| QRQTS | `fvPsEin7WcCXpGRUdwSY3WunLVuSgUE31ipXbHF3r3zat4gbq` | 39447 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| rmgaslessrelayertest | `2VBTFzU4zQ1ZsRrnXDPpnNkwHYYmkgun64rV6zXhQk3hcWXP24` | 52860 | empty, no file needed | live config is `{}` (public RPC `l1.avax.network`) |
| rwachainfuji | `vbvFXBFDHMdWBC2Gvei7eZMDwt5zrMXSni7VY7gw8NCQhe7Ah` | 191919 | empty, no file needed | live config is `{}` (validator node) |
| Safe Laxative Chain | `2sCakkCUJoKfqoS37fMmMiZvVn7ftrJgus9gW9SpD1X8fPem2q` | 26173 | empty, no file needed | live config is `{}` (validator node) |
| SecondTEST | `2HJ95937F5yY8BAMiSLsPGitc9rEDEupMsYd5VWt4R19jkSrhQ` | 76932 | empty, no file needed | live config is `{}` (Glacier proxy) |
| seniorpurplegopher | `wfapNnyaAnWpPESnzRtVA6PinAnZLWqSVZLKd4y3hCG8Sn5wj` | 44955 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| silverserenehawk | `epQVP6gEab4TZeFr72qCRmTav2so8WZvnUFUV1KugWf4vrnSe` | 25151 | empty, no file needed | live config is `{}` (Glacier proxy) |
| Smart coin | `2EzF8hyLVBbww1Tv4LAdCwKwZH8UbZvFtv1Y4A3zNTsmKvmeXr` | 330208 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| Smart coin | `2W18EUujfJ85YGEFSGf76VnvrDWVYjMKHw9yPvih4AFuwv48NV` | 105401 | empty, no file needed | live config is `{}` (Builder Hub managed node) |
| Soshi | `2sDUH9VzjQJdALDUCQcnYxC7hfW5awJC3mZNN4ySFG52Lq4GCa` | 80952 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| sushitempura | `2bQaaqGG5DZvJMBWnRe9VC5sFCz2QY5GntGzCuHrr2oqcEwnHq` | 23903 | empty, no file needed | live config is `{}` (public RPC `subnets.avax.network`) |
| swiftamberheron | `2NkpPCyY6iQexE1NzJsc6s6Yf8L9T2KLoVXAiEnswwdN4V9soq` | 23815 | empty, no file needed | live config is `{}` (public RPC `l1.avax.network`) |
| TastyPurpleMole | `2cfykT2vjWMfBVtqrQiQAg2D7mahaB3KWEhoNX3wHBDgTuy1xk` | 28019 | empty, no file needed | live config is `{}` (Glacier proxy) |
| watrTestnet | `2ZZiR6T2sJjebQguABb53rRpzme8zfK4R9zt5vMM8MX1oUm3g` | 92870 | empty, no file needed | live config is `{}` (Glacier proxy) |
| WTTTP | `21rMdJMMgpBdV2Hu3ovJVK87K8mh8WvddK7y4ZqXsyRamaYVXH` | 21557 | empty, no file needed | live config is `{}` (Glacier proxy) |
| YENTRIA | `Loe3eUnTTLKh52roR6NPcrowaddXXWqxPooTP9yxwkaTWSA5d` | 49012 | empty, no file needed | live config is `{}` (Glacier proxy) |
| (no name) | `2aRm3B4kAGK49yrrrmSdeK5q69fzpKMmAsXFdzFSN9yDZPvMFk` | 21200 | no result | no RPC answers |
| 02CardPlayTestChain | `4AvstMu2f8w7WM3LuUdjH5H1KiCQThVtaTJXEF1Mjn51zRan2` | 20261017 | no result | no RPC answers |
| ABP testnet | `HXvA5VJJ4tXjkSC3Va6yWiEtYGHETL21MKZjNEw3BFgMJrjqV` | 528 | no result | no RPC answers |
| ageFuji | `2TgG548qi5GSyPvnCT7vN6tUK3QowAGnXMHHyzbviaGuKzow8J` | 777011 | no result | no RPC answers |
| agentl1 | `25RZxbbr4RZh1xzxCKpmX6SzM77XWKhx65rfuHYBJKfJth4GYs` | 990777 | no result | no RPC answers |
| arcadeunion | `2qK8zvR4L1FN4J2hVSKJaFT8FfrHdGpYVAoqfsV6BXTETFKBfd` | 242424 | no result | no RPC answers |
| Avalanche Corner | `8EvhgfepL6sftuvzCsVVDK7EsBPmSf4CckZ1GeKvUUV3tuM4w` | 82087 | no result | no RPC answers |
| Avalanche fuji chain | `25m4h9wLZpJm2uMFWyBRmvVvEQpStB5mEL3byu4wn7U72dUyuY` | 57738 | no result | no RPC answers |
| benzonet | `21iisL1nkpM2AauUadAz7p1gK3waRBZLEJme3LU3gsWpaxy792` | 68420 | no result | no RPC answers |
| Blerify | `2ruoantX5nFehfW2aWZnQnkfH8fW9j9bdJPvArTWsZiAFEd9kP` | 42900 | no result | no RPC answers |
| BnryTestnet | `tJGE3Wrw2aCiR8AyEEWVCKhYHKcW4oWCU9N1tbXWjTisDDskK` | 835 | no result | no RPC answers |
| Brown Combat Chain | `2gS2ukbNXLNzCWgvT9dtRWdkfrN6QCAgvTVCfBWhvKCfKCraVy` | 23142 | no result | no RPC answers |
| Cairn | `igU3qLmDNE4ZYYeSCeZXyJywx6vkDqgcarzkHKzFqoAZ4NKGf` | 161464 | no result | no RPC answers |
| captevm | `2oo5UvYgFQikM7KBsMXFQE3RQv3xAFFc8JY2GEBNBF1tp4JaeZ` | 10508 | no result | no RPC answers |
| chainjffgfmcibifi | `pn3NgCwUvyQMHwGQCVLHjzDdGgQZ7pVaDNTFDULTB6RMD1uTv` | 519336152 | no result | no RPC answers |
| chkfuji | `RkJGt3ZenMaUz64EP1cUZeisftYMb9WaJdvYw54xfvkUccgke` | 191919 | no result | no RPC answers |
| cyberia | `Lm8HNjKAQ7ujA7XDphehkqbnu8wKjfzxpF1THX3HLyzoJhfv7` | 49407 | no result | no RPC answers |
| datagram | `VhdtacxdFewFKCnCJkyavYWeohf1DjgaQnQAg7H6LKoaCbyp8` | 131 | no result | no RPC answers |
| DComm | `Tap4NfbT2LWhhww4YUrnsLyv6wSbVHhd9YQeJVQqiazpomaVW` | 32666 | no result | no RPC answers |
| Dep | `2fJ7mnC2fENmNfaicf74QKwUYTjQNP7yprbz7L9SPn9YV6Nbr5` | 80796 | no result | no RPC answers |
| DFK Chain | `32sexHqc3tBQsik8h7WP5F2ruL5svqhX5opeTgXCRVX8HpbKF` | 335 | no result | no RPC answers |
| drama | `51nKdczJVk32pejdpUmKqJzBzeg23BtZxE5jiKdbWVCuUmnAL` | 710521 | no result | no RPC answers |
| DropLinked Attestation | `2WU6TwdaXZ3cHs1ytAG7Ju1KXWeb9f9Zr2LHKXSEGpkR2pJLcc` | 692544 | no result | no RPC answers |
| dtttd | `2RoxFcaNymDKQDfhU6Mr8oy8CVG1vTK3eeh6sjVjShDp4s6XDR` | 87293 | no result | no RPC answers |
| dtttd | `57jCJkgQy4CHRyXwWRwtM8AguSxPNBjfp3fxJLm1yDZC1SFcU` | 48296 | no result | no RPC answers |
| dtttd | `MyABYoR3xRXGsDFQAK7QracpKE1tBZyQVnsbo9Sg9c4Vq7HpJ` | 83303 | no result | no RPC answers |
| FrqtalTestnet | `2919i9HeqHMJHoH9rbBY2xxATrPMzcAFhHtAzS9WVA7g1PtTfa` | 21817 | no result | no RPC answers |
| FrqtalTestnet | `cUj9F1iJ7AXPcGb3RqsmXScgT4wNSiDiU75PpG4TRVLm4ksQt` | 21817 | no result | no RPC answers |
| fsevm baseline | `RjVeiJ1mzJYdxEkWN7WekDvQ8XSwMJb5f6KfPSmeAB1CgL73Q` | 72026 | no result | no RPC answers |
| fsevm tuned | `vkxKXnvH2mC21qqG1wop3itnJ2BpqFdnNWY7S7JPJTSWqrwku` | 72027 | no result | no RPC answers |
| Gleaming Haze Chain | `kDVqNTYF1WNV1NTxRr95p9Mu3zHptKkwW5RN6pau6VzNcZwhE` | 75968 | no result | no RPC answers |
| jono13 | `18QZDAMAUv9kaJog52wohkEny2aUYHkxZJtTFCzTjiCLKvPLM` | 595581 | no result | no RPC answers |
| kl1malakit | `7QjKdSJDN7MKMReG2rLwECfrdzBayVorzzHhoTNLNr6J7XYEU` | 3008 | no result | no RPC answers |
| MakaChain | `rruErbEb336prxBuKRH9TdA4R4f35Ac5Vaw7he2h3Nd2gnHjd` | 777177 | no result | no RPC answers |
| mirotst | `2cS9EU7CV4UGWtDLusi2mYMH8BCDpNuq3Aaa6rnwJrczXkqCvS` | 44451 | no result | no RPC answers |
| mirotst | `2k377KNNmF728HNAanMJcdst36PCbX2WdARBDyyB74Z8PWPGDJ` | 22880 | no result | no RPC answers |
| myl1 | `2bU2988XvYbG4z85k39QxhDNomHWREnTKzYWLViU1RMCeSwEea` | 99999 | no result | no RPC answers |
| NEXORATWO | `FyUfzvhv4bVgLzWVqWAjVWfPetLrW1zJrz76bBx2iGekDGaCD` | 20260917 | no result | no RPC answers |
| NofiTestnet | `2oWGZSj5rHmUrPTBdw9TNY74eVxge9GwisXvQQjw5bKDQKCRpq` | 880024 | no result | no RPC answers |
| Nyxor L1 | `2SrgwcLrpgt2kLrqhTr6fMaPFd1kN467ai3eaK4HcFyBq1YbaY` | 98421 | no result | no RPC answers |
| ONEWORDCHAIN | `VJqh73j1HpyFDVgHdj1VoDrsZKYcoXgL43sVjp8uyAT3GM1Uw` | 27515 | no result | no RPC answers |
| PandaSea | `nWZ4nNB1i9bT2tYUdEFP5P6S9nLGx289QaWezJge7vPLP9nd4` | 7770 | no result | no RPC answers |
| peridotdev | `214vC6YULpdhfkvEHSDwVHFMTbDdYRUpvVkqXeuuEmhUxGZLpp` | 43214001 | no result | no RPC answers |
| pltok | `dA8bad9zaEg9Ei7Wi6rDortg9NvvGTwtNiDhZfDGkfMhkGhVJ` | 7708 | no result | no RPC answers |
| Private Wei Chain | `E8SqdSRuTJ4hfs57ypBQZfCNnR4cozMwBriF9Xbey56wFYspH` | 87537 | no result | no RPC answers |
| provena | `2dmUcoiPtijDDCX3UpxEHYukdMPX6gKYcdh2TmPPg5WFDyagp8` | 61778 | no result | no RPC answers |
| QRQTD | `2p9Pft81x75dYLJd3DgKHtUzKQ7aSt4eTq3YvfjsidFXbm4bJr` | 82793 | no result | no RPC answers |
| Radiant Crest | `ZVX8x57XKXg95mMFS6KvQJ2EdBMdbGoFWmY1FJoJzJ6J1qUJG` | 257942 | no result | no RPC answers |
| RealTestnet | `F9iT4dEEByJ4hCn76XtgEqu3UwEEL1PA25ZCkWSn6TMR1div9` | 117711 | no result | no RPC answers |
| REXChain | `25xqA8rtWLkmZL38Xap4uszE4CikMFC8JgtmRykQjAJLx1psQS` | 12392 | no result | no RPC answers |
| rjtts | `CiZqkCQqJEb49HLdfN3hWz6zAjE3CFHgJ6nRvJvYEjvnEyALC` | 95104 | no result | no RPC answers |
| rndnet | `TEXRUQF3PY9hUU4XWkhvnWuvp8wk3m9V6dKwnSoLJXodWhR9H` | 528 | no result | no RPC answers |
| rt | `23cXDQmGBtouf7Qg4TYsU96GfuDmWD2Sthnao4UTwkk63g1Gnu` | 220377823 | no result | no RPC answers |
| rt | `27B4DDddLgjon3EingqoU26BHQMaiQmvPcHJpQkkWu3NuHLfRY` | 34737338 | no result | no RPC answers |
| rt | `29CPRVay4xcTemyy3NkkTrWNczmYkfWY4cehDPmgzGWmyPs8Rx` | 543842380 | no result | no RPC answers |
| rt | `2v7JP8NpTbqGnTpAoPSq62qDxL1FNgVL3GVMTsQwCDC3U3WwaF` | 609198036 | no result | no RPC answers |
| rt | `FczvNe327rcVrke1A7rWZkSWked2HtYueSLSLqXpmKQoTRNGj` | 282277365 | no result | no RPC answers |
| rt | `b4e7inkrixcbGnKp4vhW3ZnGoq7cknKMkTbvsbnpUWybtECgu` | 916147609 | no result | no RPC answers |
| rt | `niof7r1V4ngwtb3pv6NQQa22YWHvj2xbDegYKnB436Zkim1v2` | 232476375 | no result | no RPC answers |
| rt | `wntJSwUuG5WMTr8BPQPrCQ5ZTddrhHvKtQ6QzQAVENc2gdhdz` | 474736591 | no result | no RPC answers |
| rt22084738 | `2v7HJe6sncd1s3YMf4iXADgkiuXFRkECfcaALZ218yjETox2hV` | 122088022 | no result | no RPC answers |
| rt67191856 | `AKRm1keRuNUH9jTuLVPs1eCzF2gErqKCiZhGoPdN5ULMF1iWH` | 367195873 | no result | no RPC answers |
| rt84268796 | `F1JEDkbrXrmnHnFik6QCQf8vi9o4eA14mXJ2HcNTy3TWK1vWj` | 584271642 | no result | no RPC answers |
| rt87395063 | `2oWuJ3dJs4HshcUwqC4L7S8kJdoP74e67ydue3tqJS6Y1h3Uwk` | 987396836 | no result | no RPC answers |
| rwa | `2V5qotZnbwJRZ1mtb7pBCcid3DTaJors7on95CxdUSnxfdAFHF` | 727001 | no result | no RPC answers |
| secondaryharlequincheetah | `orMPCSAXmDY5nNqrKgJQku5EPbTSMGQNDvBL8S8Uu1CLg7vtp` | 4995 | no result | no RPC answers |
| SentinelL1 | `2PqooUD2u82cQVbEHUfSnK5GTnRfUg17d8RUuccc97y27NkZhd` | 99999 | no result | no RPC answers |
| Signal | `2F1SfodznZumMGnFTDTBnFC5uupPy5HHVzypedYAuDNQ6reXaZ` | 89841 | no result | no RPC answers |
| Skarn | `2Ja9YVJwsbXH52EHHTzGMYj4fvgMWftG7SRBxU1pz3JwzqSULo` | 7527 | no result | no RPC answers |
| Skarn | `hWsS2QVncdzzb3LMYRfMUpgBN5PtYzUHT4zDVg1JUVd1eLbzF` | 7527 | no result | no RPC answers |
| SKY | `SDxMzdWdPTwdypG6mnXSz4tp7vcqKFc2vEe85YnRP1sAwhHen` | 29358 | no result | no RPC answers |
| SL1023Q1TS | `rR57kqYaiciEKHPPU43io745cNEc8fuw3XiJtYNtH5vQEmZiN` | 94970 | no result | no RPC answers |
| SLQTS | `2vZ2uEeN4s3W4yyWbidpxeqcVDBCg42noCVYsWEGranW2STWeX` | 4991 | no result | no RPC answers |
| Sundial Testnet | `2a4hDpqsD2vhLq8DGDf6HeZnKD4wX8Kx9oyPmfCehmf9A662Z9` | 113311 | no result | no RPC answers |
| Tassat Testnet 01 | `2kp8PruHBv8AyDo2n2M9nVrGBJpyFGxxHwBetYJFCBQtuiKY49` | 49733 | no result | no RPC answers |
| tFrqtal | `2jUuRq6ZXqHvZdS8HrF25BvA5Cevu2NUYCdFPNBVizUdW9B8gS` | 21817 | no result | no RPC answers |
| tFrqtal | `XtQ2kYTutd4NCKQnvmiGFayALopRXJ4KjzN1uu1VALiLmfww3` | 21817 | no result | no RPC answers |
| TradeDar Regulatory Chain STG | `2bMykTrjmqzmL5o8tYyjpRkYQB83r2ncFQGF3MkLNwMSQaahsn` | 54858 | no result | no RPC answers |
| Troubled Homely Splinter Chain | `64q4cp3bCBpFzNyYoJ3SnTiFnMM4HtkqGvqX6ZC6FRJBa7bEa` | 21211 | no result | no RPC answers |
| trustechledger | `2SndTptHdH3TKvoABAa5HKyb3AiqEnprc6nnwxj7PHq7muPgNq` | 173427 | no result | no RPC answers |
| VJTTD | `23f5WZwsP5d2iUtontakgEuhecxVzs9t9uQeNEKtuXHiFWmnFh` | 58882 | no result | no RPC answers |

## Files for chains that are not active now

These chains had a file but have no active validator at the snapshot. The files stay here in case the chains restart.

| name | blockchainID | EVM chain ID | upgrade.json | how it was verified |
|---|---|---|---|---|
| lamina1 | `UhReZTXT8Cqsjat9ghRtCe5kBQPQexQB5zG5Fvf3egrdYfyoJ` | 10849 | [file](mainnet/lamina1-UhReZTXT8Cqsjat9ghRtCe5kBQPQexQB5zG5Fvf3egrdYfyoJ.json) (not verified) | held file, no RPC answers; from the lamina1-integration README (2025-02) |
| Space | `22CN6x5LAPEkvLDdz4UwG3XXtZV69Su3bcspiYtkF9k5f9rcCt` | 8227 | [file](mainnet/space-22CN6x5LAPEkvLDdz4UwG3XXtZV69Su3bcspiYtkF9k5f9rcCt.json) | live config equals file (our archive node at tip) |

## Notes

- PandaSea's file uses `txBlockListConfig`, a precompile that exists only in
  PandaSea's subnet-evm fork. Stock subnet-evm rejects this file. The same holds
  for Fuji fscfive (`nettingConfig`, `privacyConfig`, `qedisIdentityConfig`).
- Mainnet STLINK has no result, but its genesis has the same year-9999 Etna as
  AIBTRUST and MUGEN, and both of those need an Etna override. STLINK probably
  needs a file too.
- An answer proves only activations whose time is already past. A future
  activation that is in the validators' file but not in the answering node's
  file does not show yet.
