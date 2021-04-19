# Relayer

![Relayer](./docs/images/github-repo-banner.gif)

[![Project Status: WIP – Initial development is in progress, but there has not yet been a stable, usable release suitable for the public.](https://img.shields.io/badge/repo%20status-WIP-yellow.svg?style=flat-square)](https://www.repostatus.org/#wip)
![GitHub Workflow Status](https://img.shields.io/github/workflow/status/cosmos/relayer/BUILD%20-%20build%20and%20binary%20upload?style=flat-square)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue?style=flat-square&logo=go)](https://godoc.org/github.com/cosmos/relayer)
[![Go Report Card](https://goreportcard.com/badge/github.com/cosmos/relayer?style=flat-square)](https://goreportcard.com/report/github.com/cosmos/relayer)
[![License: Apache-2.0](https://img.shields.io/github/license/cosmos/relayer.svg?style=flat-square)](https://github.com/cosmos/relayer/blob/master/LICENSE)
[![Lines Of Code](https://img.shields.io/tokei/lines/github/cosmos/relayer?style=flat-square)](https://github.com/cosmos/relayer)
[![Version](https://img.shields.io/github/tag/cosmos/relayer.svg?style=flat-square)](https://github.com/umee-network/cosmos/relayer/latest)

This repository contains a Golang implementation of a Cosmos [IBC](https://ibcprotocol.org/)
relayer. The `relayer` package contains a basic relayer implementation that is
meant for users wanting to relay packets/data between sets of
[IBC](https://ibcprotocol.org/)-enabled chains.

The relayer implementation also acts as a base reference implementation for those
wanting to build their [IBC](https://ibcprotocol.org/)-compliant relayer.
  
> **NOTE:** The relayer is currently in alpha and is not production ready. If it
> is used in production, it should always be run in a secure environment and
> only with just enough funds to relay transactions. Security critical operations
> **should** manually verify that the client identifier used in the configuration
> file corresponds to the correct initial consensus state of the counter-party
> chain. This can be done by querying the initial consensus state and the header
> of the counter-party and verifying that the root and hash of the next validator
> set match. This can be considered equivalent to checking the sha hash of a
> download or a GPG signature.

- [Relayer](#relayer)
  - [Basic Usage](#basic-usage)
  - [Features](#features)
  - [Relayer Terminology](#relayer-terminology)
  - [Recommended Pruning Settings](#recommended-pruning-settings)
  - [Compatibility Table](#compatibility-table)
  - [Testnet](#testnet)
  - [Demo](#demo)
  - [Security Notice](#security-notice)
  - [Code of Conduct](#code-of-conduct)

## Basic Usage

To setup and start the IBC relayer between two IBC-enabled networks, the following
steps are typically performed:

1. Install the latest release.
2. Initialize the relayer's configuration.

   ```shell
   $ rly config init
   ```

3. Add relevant chain configurations to the relayer's configuration. See the
   [Chain](https://pkg.go.dev/github.com/cosmos/relayer/relayer#Chain) type for
   more information.

   e.g. chain configuration:

   ```shell
   # chain_a_config.json
   {
     "chain-id": "chain-a",
     "rpc-addr": "http://127.0.0.1:26657",
     "account-prefix": "cosmos",
     "gas-adjustment": 1.5,
     "gas-prices": "0.001umuon",
     "trusting-period": "10m"
   }
   ```

   ```shell
   $ rly chains add -f chain_a_config.json
   $ rly chains add -f chain_b_config.json
   ```

4. Either import or create new keys for the relayer to use when signing and
   relaying transactions.

   ```shell
   $ rly keys add relayer-chain-a # relayer key for chain_a
   $ rly keys add relayer-chain-b # relayer key for chain_b
   ```

5. Assign the relayer chain-specific keys created or imported above to the
   specific chain's configuration. Note, `key` from step (3).

   ```shell
   $ rly chains edit chain-a key relayer-chain-a
   $ rly chains edit chain-b key relayer-chain-b
   ```

6. Both relayer accounts, e.g. `relayer-chain-a` and `relayer-chain-b`, need to
   funded with tokens in order to successfully sign and relay transactions
   between the IBC-connected networks. How this occurs depends on the network,
   context and environment, e.g. local or test networks can use a faucet.
7. Ensure both relayer accounts are funded by querying each.

   ```shell
   $ rly q balance chain-a
   $ rly q balance chain-b
   ```

8. Now we are ready to initialize the light clients on each network. The relayer
   will used the configured RPC endpoints from each network to fetch header
   information and initialize the light clients.

   ```shell
   $ rly light init chain-a -f
   $ rly light init chain-b -f
   ```

9. Next, we generate a new path representing a client, connection, channel and a
   specific port between the two networks.

   ```shell
   $ rly paths generate chain-a chain-b transfer --port=transfer
   ```

10. Finally, we start the relayer on the path created in step (9). The relayer
    will periodically update the clients and listen for IBC messages to relay.

    ```shell
    $ rly start transfer
    ```

## Features

The relayer supports the following:

- creating/updating IBC Tendermint light clients
- creating IBC connections
- creating IBC transfer channels.
- initiating a cross chain transfer
- relaying a cross chain transfer transaction, its acknowledgement, and timeouts
- relaying from state
- relaying from streaming events
- sending an UpgradePlan proposal for an IBC breaking upgrade
- upgrading clients after a counter-party chain has performed an upgrade for IBC breaking changes

The relayer currently cannot:

- create clients with user chosen parameters (such as UpgradePath)
- submit IBC client unfreezing proposals
- monitor and submit misbehavior for clients
- use IBC light clients other than Tendermint such as Solo Machine
- connect to chains which don't implement/enable IBC
- connect to chains using a different IBC implementation (chains not using SDK's `x/ibc` module)

## Relayer Terminology

A `path` represents an abstraction between two IBC-connected networks. Specifically,
the `path` abstraction contains metadata about a source chain, a destination
chain and a relaying strategy between thw two networks. The metadata for both
the source and destination networks contains the following:

- `chain-id`: The chain ID of the network.
- `client-id`: The client ID on the corresponding chain representing the other chain's light client.
- `connection-id`: The connection ID on the corresponding chain representing a connection to the other chain.
- `channel-id`: The channel ID on the corresponding chain's connection representing a channel on the other chain.
- `port-id`: The IBC port ID which a relevant module binds to on the corresponding chain.
- `order`: Determines if packets from a sending module must be `ORDERED` or `UNORDERED`.
- `version`: IBC version.

Two chains may have many different paths between them. Any path with different
clients, connections, or channels are considered uniquely different and non-fungible.

When using with live networks, it is advised to pre-select your desired parameters
for your clients, connections, and channels. The relayer will automatically
reuse any existing clients that match your configurations since clients,
connections, and channels are public goods (no one has control over them).

## Recommended Pruning Settings

The relayer relies on old headers and proofs constructed at past block heights
to facilitate correct[IBC](https://ibcprotocol.org/) behavior. For this reason,
connected full nodes may prune old blocks once they have passed the unbonding
period of the chain but not before. Not pruning at all is not necessary for a
fully functional relayer, however, pruning everything will lead to many issues!

Here are the settings used to configure SDK-based full nodes (assuming 3 week unbonding period):

```shell
... --pruning=custom --pruning-keep-recent=362880 --pruning-keep-every=0 --pruning-interval=100
```

`362880 (3*7*24*60*60 / 5 = 362880)` represents a 3 week unbonding period (assuming 5 seconds per block).

Note, operators can tweak `--pruning-keep-every` and `--pruning-interval` to their
liking.

## Compatibility Table

| chain                                    | build                                                                                                                                                   | supported ports |
|------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| [gaia](https://github.com/cosmos/gaia)   | ![GitHub Workflow Status](https://img.shields.io/github/workflow/status/cosmos/relayer/TESTING%20-%20gaia%20to%20gaia%20integration?style=flat-square)  | transfer        |
| [akash](https://github.com/ovrclk/akash) | ![GitHub Workflow Status](https://img.shields.io/github/workflow/status/cosmos/relayer/TESTING%20-%20akash%20to%20gaia%20integration?style=flat-square) | transfer        |

## Testnet

If you would like to join a relayer testnet, please [check out the instructions](./testnets/README.md).

## Demo

![Demo](./docs/images/demo.gif)

While the relayer is under active development, it is meant primarily as a learning
tool to better understand the Inter-Blockchain Communication (IBC) protocol. In
that vein, the following demo demonstrates the core functionality which will
remain even after the changes:

```bash
# ensure go and jq are installed 
# Go Documentation: https://golang.org/doc/install
# jq Documentation: https://stedolan.github.io/jq/download

# First, download and build the gaia source code so we have a working blockchain to test against
$ make get-gaia build-gaia

# two-chainz creates two gaia-based chains with data directories in this repo
# it also builds and configures the relayer for operations with those chains
$ ./scripts/two-chainz
# NOTE: If you want to stop the two gaia-based chains running in the background use `killall gaiad`

# At this point the relayer --home directory is ready for normal operations between
# ibc-0 and ibc-1. Looking at the folder structure of the relayer at this point is helpful
# NOTE: to install tree try `brew install tree` on mac or `apt install tree` on linux
$ tree ~/.relayer

# See if the chains are ready to relay over
$ rly chains list

# See the current status of the path you will relay over
$ rly paths list

# Now you can connect the two chains with one command:
$ rly tx link demo -d -o 3s

# Check the token balances on both chains
$ rly q balance ibc-0
$ rly q bal ibc-1

# Then send some tokens between the chains
$ rly tx transfer ibc-0 ibc-1 1000000samoleans $(rly chains address ibc-1)
$ rly tx relay demo -d
$ rly tx acks demo -d

# See that the transfer has completed
$ rly q bal ibc-0
$ rly q bal ibc-1

# Send the tokens back to the account on ibc-0
$ rly tx transfer ibc-1 ibc-0 1000000ibc/27A6394C3F9FF9C9DCF5DFFADF9BB5FE9A37C7E92B006199894CF1824DF9AC7C $(rly chains addr ibc-0)
$ rly tx relay demo -d
$ rly tx acks demo -d

# See that the return trip has completed
$ rly q bal ibc-0
$ rly q bal ibc-1

# NOTE: you will see the stake balances decreasing on each chain. This is to pay for fees
# You can change the amount of fees you are paying on each chain in the configuration.
```

## Security Notice

If you would like to report a security critical bug related to the relayer repo,
please send an email to [`security@cosmosnetwork.dev`](mailto:security@cosmosnetwork.dev)

## Code of Conduct

The Cosmos community is dedicated to providing an inclusive and harassment free
experience for contributors. Please visit [Code of Conduct](CODE_OF_CONDUCT.md) for more information.
