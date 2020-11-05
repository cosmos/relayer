# Relayer

![GOZ](./docs/images/github-repo-banner.png)

![Relayer Build](https://github.com/cosmos/relayer/workflows/Build%20then%20run%20CI%20Chains/badge.svg)

The Cosmos IBC `relayer` package contains a basic relayer implementation that is
meant for users wishing to relay packets/data between sets of IBC enabled chains.
In addition, it is well documented and intended as an example where anyone who is
interested in building their own relayer can come for complete, working, examples.

### Security Notice

If you would like to report a security critical bug related to the relayer repo, please send an email to [`security@cosmosnetwork.dev`](mailto:security@cosmosnetwork.dev)

## Code of Conduct

The iqlusion team is dedicated to providing an inclusive and harassment free experience for contributors. Please visit [Code of Conduct](CODE_OF_CONDUCT.md) for more information.

## Testnet

If you would like to join a relayer testnet, please [check out the instructions](./testnets/README.md).

### Compatibility Table:

> NOTE: 

| chain | tests | supported ports |
|-------|--------|----------------|
| [`gaia`](https://github.com/cosmos/gaia) | ![gaia](https://github.com/cosmos/relayer/workflows/TESTING%20-%20gaia%20to%20gaia%20integration/badge.svg) | `transfer` |

## Demoing the Relayer

![Demo](./docs/images/demo.gif)

While the relayer is under active development, it is meant primarily as a learning tool to better understand the Inter-Blockchain Communication (IBC) protocol. In that vein, the following demo demonstrates the core functionality which will remain even after the changes:

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

# See that the transfer has completed
$ rly q bal ibc-0
$ rly q bal ibc-1

# Send the tokens back to the account on ibc-0
$ rly tx xfer ibc-1 ibc-0 1000000transfer/ibczeroxfer/samoleans $(rly ch addr ibc-0)
$ rly tx rly demo -d

# See that the return trip has completed
$ rly q bal ibc-0
$ rly q bal ibc-1

# NOTE: you will see the stake balances decreasing on each chain. This is to pay for fees
# You can change the amount of fees you are paying on each chain in the configuration.
```

## Setting up Developer Environment

Working with the relayer can frequently involve working with local development branches of `gaia`, `akash`, `cosmos-sdk` and the `relayer`. To setup your environment to point at the local versions of the code and reduce the amount of time in your read-eval-print loops try the following:

1. Set `replace github.com/cosmos/cosmos-sdk => /path/to/local/github.com/comsos/cosmos-sdk` at the end of the `go.mod` files for the `relayer` and `gaia`. This will force building from the local version of the `cosmos-sdk` when running the `./dev-env` script.
2. After `./dev-env` has run, you can use `go run main.go` for any relayer commands you are working on. This allows you make changes and immediately test them as long as there are no server side changes.
3. If you make changes in `cosmos-sdk` that need to be reflected server-side, be sure to re-run `./two-chainz`.
4. If you need to work off of a `gaia` branch other than `master`, change the branch name at the top of the `./two-chainz` script.
