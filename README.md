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

### Command Structure:

- [x] `chains` - return a json representation of the configured chains
- [x] `keys` - needs testing
  * [x] `add [chain-id] [name]` - Add a new key with name generating and returning mnemonic
  * [x] `delete [chain-id] [name]` - Delete a key with name
  * [x] `export [chain-id] [name]` - Export the mnemonic from a given key
  * [x] `list [chain-id]` - list the keys associated with a given chain
  * [x] `restore [chain-id] [name] [mnemonic]` - Restore a key to a chain's keychain with a name and mnemonic
  * [x] `show [chain-id] [name]` - Show details for a key from a given chain
- [x] `lite` - @melekes working on ensuring commands work as intended
  * [x] `init [chain-id] (flags)` - Initialize from a header hash and a height
  * [x] `header [chain-id] [height]` - Returns a header at height from the database
  * [x] `update [chain-id] [header-hash] [height]` - Updates lite client to given header
  * [x] `delete [chain-id]` - Deletes on disk lite client forcing re-initialization
- [ ] `query` - Some implementations working/complete others just stubbs. All queries from the `relayer` return proofs!
  * [x] `header [chain-id] [height]` - Query the header at a given height
  * [x] `node-state [chain-id] [height]` - Query the node state at a given height or latest if height not passed
  * [x] `client [chain-id] [client-id]` - Query details for an individual client
  * [x] `clients [chain-id]` - Query the list of clients on a given chain
  * [x] `connection [chain-id] [connection-id]` - Query details for an individual connection
  * [x] `connections [chain-id] [client-id]`- Query the list of connections associated with a client
  * [x] `channel [chain-id] [connection-id] [channel-id]` - Query details for an individual channel
  * [ ] `channels [chain-id] [connection-id]` - Query the list of channels associated with a client
  * [ ] `seq-send [chain-id] [channel-id]` - Query the `seq-send` for the configured key on a given chain and channel
  * [ ] `seq-recv [chain-id] [channel-id]` - Query the `seq-recv` for the configured key on a given chain and channel
- [ ] `start` - run the configured strategy on an ongoing basis. This is currently in the design phase
- [ ] `transactions` - Run IBC transactions, testing currently blocked on working lite client commands
  * [x] `client [src-chain-id] [dst-chain-id] [client-id]` - Create a client for dst on src
  * [x] `clients [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id]` - Create clients for src and dst on opposite chains, sends one transaction to each chain
  * [ ] `connection [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]` - Repair or create a connection with the given identifiers between src and dst. Runs connection-step in a loop till completion
  * [ ] `connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]` - Send the next transaction set to repair or create a connection between src and dst.
  * [ ] `channel [src-chain-id] [dst-chain-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]`- Repair or create a channel with the given identifiers between src and dst. Runs channel-step in a loop till completion
  * [ ] `channel-step [src-chain-id] [dst-chain-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]`- Send the next transaction set to repair or create a channel between src and dst.
  * [ ] `flush-chan [src-chain-id] [dst-chain-id] [src-channel-id] [src-port-id]` relay packets on channel in src chain to dst chain 
  * [ ] `raw` - raw connection and channel step commands
    - [ ] `conn-init` 
    - [ ] `conn-try` 
    - [ ] `conn-ack`
    - [ ] `conn-confirm` 
    - [ ] `chan-init` 
    - [ ] `chan-try` 
    - [ ] `chan-ack`
    - [ ] `chan-confirm`
    - [ ] `chan-close-init`
    - [ ] `chan-close-confirm`


### Notes:

- Relayers should gracefully handle `ErrInsufficentFunds` when/if accounts run
  out of funds
    * _Stretch_: Relayer should notify a configurable endpoint when it hits
      `ErrInsufficentFunds`

### Open Questions

- Do we want to force users to name their `ibc.Client`s, `ibc.Connection`s,
 `ibc.Channel`s and `ibc.Port`s? Can we use randomly generated identifiers
 instead?

 
