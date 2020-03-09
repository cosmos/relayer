# Relayer

The `relayer` package contains some basic relayer implementations that are
meant to be used by users wishing to relay packets between IBC enabled chains.
It is also well documented and intended as a place where users who are
interested in building their own relayer can come for working examples.

### Building and testing the relayer

The current implementation status is below. To test the functionality that does work you can do the following:

> NOTE: The relayer relies on `cosmos/cosmos-sdk@ibc-alpha` and `tendermint/tendermint@v0.33.0-dev2`. If you run into problems building it likely related to those dependancies. Also the `two-chains.sh` script requires that the `cosmos/gaia` and `cosmos/relayer` repos be present locally and buildable. Read the script and change the paths as needed.

```bash
$ ./two-chains.sh
# NOTE: Follow the ouput instructions and set $RLY and $GAIA

# First, initialize the lite clients locally for each chain
$ relayer --home $RLY lite init ibc0 -f
$ relayer --home $RLY lite init ibc1 -f

# If you would like to see the folder structure of the relayer
# try running `tree $RLY`

# Now you can create the clients for each chain on their counterparties
$ relayer --home $RLY tx clients ibc0 ibc1 ibconeclient ibczeroclient

# Then query them for more info:
$ relayer --home $RLY q clients ibc0
$ relayer --home $RLY q client ibc0 ibconeclient
$ relayer --home $RLY q clients ibc1
$ relayer --home $RLY q client ibc1 ibczeroclient

# NOTE: The following are unimplemented commands
# Next create a connection
$ relayer --home $RLY tx connection ibc0 ibc1 ibconeclient ibczeroclient ibconeconn ibczeroconn

# Now you can query for the connection
$ relayer --home $RLY q connections ibc0
$ relayer --home $RLY q connection ibc0 ibconeconn
$ relayer --home $RLY q connections ibc1
$ relayer --home $RLY q connection ibc1 ibczeroconn

# Next  create a channel
$ relayer --home $RLY tx channel ibc0 ibc1 ibconeconn ibczeroconn ibconchan ibczerochan bank bank

# Now you can to query for the channel
$ relayer --home $RLY q channels ibc0
$ relayer --home $RLY q channel ibc0 ibconechan bank
$ relayer --home $RLY q channels ibc1
$ relayer --home $RLY q channel ibc1 ibczerochan bank

# TODO: figure out the commands to flush and send packets from chain to chain
```
### Current Work:

- [ ] `query` - https://github.com/cosmos/relayer/issues/19
- [ ] `start` - https://github.com/cosmos/relayer/issues/21
- [ ] `transactions`- https://github.com/cosmos/relayer/issues/10

### Notes:

- Relayers should gracefully handle `ErrInsufficentFunds` when/if accounts run
  out of funds
    * _Stretch_: Relayer should notify a configurable endpoint when it hits
      `ErrInsufficentFunds`

### Open Questions

- Do we want to force users to name their `ibc.Client`s, `ibc.Connection`s,
 `ibc.Channel`s and `ibc.Port`s? Can we use randomly generated identifiers
 instead? The current build of the relayer only works this way, and it will end up requiring
 quite a bit of user input. The relayer should query to ensure primatives for a counterparty
 chain exist and default to using those first before deciding to create its own.
 It should also create as many of the primatives as possible to random or generated 
 identifiers (hash of chain-ids or chainid-$(rand)).

 ## Setting up Developer Environment

 Working with the relayer can frequently involve working with local developement branches of `gaia`, `cosmos-sdk` and the `relayer`. To setup your environment to point at the local versions of the code and reduce the amount of time in your read-eval-print loops try the following:

 1. Set `replace github.com/cosmos/cosmos-sdk => /path/to/local/github.com/comsos/cosmos-sdk` at the end of the `go.mod` files for the `relayer` and `gaia`. This will force building from the local versions when running the `./two-chains.sh` script.
 2. After `./two-chains.sh` has run, use `go run main.go --home $RLY` for any relayer commands you are working on. This allows you make changes and immediately test them as long as there are no server side changes. 
 3. If you make changes in `cosmos-sdk` that need to be reflected server-side, be sure to re-run `./two-chains.sh`.
 4. If you need to work off of a `gaia` branch other than `ibc-alpha`, change the branch name at the top of the `./two-chains.sh` script. 