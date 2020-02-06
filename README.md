# Relayer

The `relayer` package contains some basic relayer implementations that are
meant to be used by users wishing to relay packets between IBC enabled chains.
It is also well documented and intended as a place where users who are
interested in building their own relayer can come for working examples.

### Command Structure:

- [x] `chains` - return a json representation of the configured chains
- [x] `keys` - needs testing
  * [x] `add [chain-id] [name]` - Add a new key with name generating and returning mnemonic
  * [x] `delete [chain-id] [name]` - Delete a key with name
  * [x] `export [chain-id] [name]` - Export the mnemonic from a given key
  * [x] `list [chain-id]` - list the keys associated with a given chain
  * [x] `restore [chain-id] [name] [mnemonic]` - Restore a key to a chain's keychain with a name and mnemonic
  * [x] `show [chain-id] [name]` - Show details for a key from a given chain
- [ ] `lite` - @melekes working on ensuring commands work as intended
  * [ ] `init [chain-id] [header-hash] [height]` - Initialize from a header hash and a height
  * [ ] `init-force [chain-id]` - Initialize from provider configured for `chain-id`
  * [ ] `init-url [chain-id] [url]` - pass in a URL which resolves to `[]TrustOptions` and initialize from latest header
  * [ ] `header [chain-id] [height]` - Returns a header at height from the database
  * [ ] `latest-header [chain-id]` - Returns the latest header from the database
  * [ ] `latest-height [chain-id]` - Returns the latest height from the database
  * [ ] `update [chain-id]` - Updates to latest header from configured provider
  * [ ] `reset [chain-id]` - Deletes on disk lite client forcing re-initialization
- [ ] `query` - Some implementations working/complete others just stubbs. All queries from the `relayer` return proofs!
  * [x] `client [chain-id] [client-id]` - Query details for an individual client
  * [x] `clients [chain-id]` - Query the list of clients on a given chain
  * [ ] `connection [chain-id] [client-id] [connection-id]` - Query details for an individual connection
  * [ ] `connections [chain-id] [client-id]`- Query the list of connections associated with a client
  * [ ] `channel [chain-id] [connection-id] [channel-id]` - Query details for an individual channel
  * [ ] `channels [chain-id] [connection-id]` - Query the list of channels associated with a client
  * [ ] `seq-send [chain-id] [channel-id]` - Query the `seq-send` for the configured key on a given chain and channel
  * [ ] `seq-recv [chain-id] [channel-id]` - Query the `seq-recv` for the configured key on a given chain and channel
  * [x] `header [chain-id] [height]` - Query the header at a given height
  * [x] `latest-header [chain-id]` - Query the latest height then return that header
  * [x] `latest-height [chain-id]` - Query the latest height
  * [x] `node-state [chain-id] [height]` - Query the node state at a given height or latest if height not passed
- [ ] `start` - run the configured strategy on an ongoing basis. This is currently in the design phase
- [ ] `transactions` - Run IBC transactions, testing currently blocked on working lite client commands
  * [ ] `client [src-chain-id] [dst-chain-id] [client-id]` - Create a client for dst on src
  * [ ] `clients [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id]` - Create clients for src and dst on opposite chains, sends one transaction to each chain
  * [ ] `connection [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]` - Repair or create a connection with the given identifiers between src and dst. Runs connection-step in a loop till completion
  * [ ] `connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]` - Send the next transaction set to repair or create a connection between src and dst.
  * [ ] `channel [src-chain-id] [dst-chain-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]`- Repair or create a channel with the given identifiers between src and dst. Runs channel-step in a loop till completion
  * [ ] `channel-step [src-chain-id] [dst-chain-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]`- Send the next transaction set to repair or create a channel between src and dst.
  * [ ] `flush-chan [src-chain-id] [dst-chain-id] [src-channel-id] [src-port-id]` relay packets on channel in src chain to dst chain 

### Notes:

- Relayers should gracefully handle `ErrInsufficentFunds` when/if accounts run
  out of funds
    * _Stretch_: Relayer should notify a configurable endpoint when it hits
      `ErrInsufficentFunds`

### Open Questions

- Do we want to force users to name their `ibc.Client`s, `ibc.Connection`s,
 `ibc.Channel`s and `ibc.Port`s? Can we use randomly generated identifiers
 instead?
