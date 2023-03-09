# Implementing a new Chain

Adding a Chain for IBC relaying is composed of two main components:
- `ChainProvider` implementation
- `ChainProcessor` implementation

## ChainProvider

The `ChainProvider` implementation contains the methods required to query for relevant data, assemble IBC messages to be sent to the chain, and manage the keys for the wallets that will be sending transactions to the chain.

`ChainProvider` non-imported methods are used for assembling messages with intention of sending to the chain. The `PathProcessor` uses these during runtime. The CLI methods also use these for things such as linking paths and flushing packets and acks.

`KeyProvider` methods are used for the key lifecycle, used by the CLI to manage relayer wallets.

`QueryProvider` methods are all of the queries against blockchain nodes that are needed for relaying.

## ChainProcessor

The [`ChainProcessor`](../relayer/processor/chain_processor.go) implementation is responsible for staying in sync with the chain, either through polling or pub/sub, and sharing IBC messages and other relevant IBC information such as IBC headers, client states, connection states, and channel states with the `PathProcessor`.

```go
type ChainProcessor interface {
	Run(ctx context.Context, initialBlockHistory uint64) error
	Provider() provider.ChainProvider
	SetPathProcessors(pathProcessors PathProcessors)
}
```

The  implementation should use the `ChainProvider` when possible to make queries.

### Startup

At the beginning of the `Run` method, the latest committed height of the chain should be queried with a retry on error. Once the latest height has been determined, the `initialBlockHistory` parameter should be subtracted to determine which block should be queried first.

After this, before the main poll loop or subscriber begins, two `ChainProcessor` caches should be initialized:

```go
	// holds open state for known connections
	connectionStateCache processor.ConnectionStateCache

	// holds open state for known channels
	channelStateCache processor.ChannelStateCache
```

These caches are aliased types to `map[ConnectionKey]bool` and `map[ChannelKey]bool` respectively. The `PathProcessor` needs to know which connections are open and which channels are open. A value of `true` for the specific `ConnectionKey` or `ChannelKey` will inform the `PathProcessor` that the connection or channel is open later on once these caches are shared with the `PathProcessor`.

During the initalization of these caches, separate mappings should also be built for which connections belong to which clients and which channels belong to which connections. The example of these in the `CosmosChainProcessor` are:

```go
	// map of connection ID to client ID
	connectionClients map[string]string

	// map of channel ID to connection ID
	channelConnections map[string]string
```

These are used later on when sharing data with the `PathProcessor`(s) to filter channels and connections for a single client ID, since a `PathProcessor` is scoped to two chains, and a single client per chain.

After these four caches are initialized in `Run`, the main poll loop or subscriber can begin.

### Main ChainProcessor Process

The `CosmosChainProcessor` uses a poll loop `queryCycle` to stay in sync with the latest chain blocks and IBC messages in those blocks. This loop will run frequently to check for new blocks and parse any IBC messages in those blocks. This poll loop starts in the `Run` function after the `Startup` tasks above.

It stores any new `IBCHeader`s into an `IBCHeaderCache` (`map[uint64]provider.IBCHeader`), which are necessary so that the `PathProcessor` can update the light clients.

A chain-specific `IBCHeader` implementation is required:

```go
type IBCHeader interface {
	Height() uint64
	ConsensusState() ibcexported.ConsensusState
	// require conversion implementation for third party chains
	ToCosmosValidatorSet() (*tmtypes.ValidatorSet, error)
}
```

For reference, view the `CosmosIBCHeader` implementation in the [Cosmos Provider](../relayer/chains/cosmos/provider.go).

#### High Level Sequence Flow

1. Query latest committed chain height
2. Initialize `IBCHeaderCache` and `IBCMessagesCache`
3. Iterate for height `i` from last successfully processed height to latest height:

- a. Query transactions within block at height `i`
- b. Query `IBCHeader` for height `i` (done in parallel with block transactions)
- c. Save latest block height and time on `ChainProcessor` (type `provider.LatestBlock`)
- d. Cache `IBCHeader` in `IBCHeaderCache` from step `2`.
- e. Iterate through transactions in block, and IBC messages within those transactions. Construct IBC message types that can be shared with the `PathProcessor` (`provider.PacketInfo`, `provider.ChannelInfo`, `provider.ConnectionInfo`), and cache those messages on the `IBCMessagesCache` from step `2`. When observing these messages, the `ChainProcessor` caches should be updated, such as setting the value in `connectionStateCache` to `false` if a connection open init or try event is observed, and `true` if a connection open ack or confirm event is observed. For more information about these steps, see Event Parsers and Message Handlers below.
- f. Save the latest successfully processed height

4. If no new blocks were processed, but the `ChainProcessor` is now in sync with the latest height of the chain, trigger the `PathProcessor`s with `pp.ProcessBacklogIfReady()`
5. If new blocks were processed, iterate through the `PathProcessor`s and pass the relevant data to them:

- a. Latest block from `3c`
- b. Latest `IBCHeader` from `3b` for most recent successfully queried block
- c. All new `IBCHeader`s in the `IBCHeaderCache` (built by steps `2` and `3d`)
- d. All new IBC messages in the `IBCMessagesCache` (built by steps `2` and `3e`)
- e. `InSync` for whether the latest successfully processed block is the latest block of the chain
- f. `ClientState` for the latest `ConsensusHeight` of the relevant client. `CosmosChainProcessor` will query for this if it's not yet cached on the `latestClientState`, otherwise it will return the most recent cached value.
- g. `ConnectionStateCache` with the connection states filtered for only the connections on the relevant client
- h. `ChannelStateCache` with the channel states filtered for only the channels on the relevant client

#### Event Parsers

For Comet BFT chains, the IBC messages are parsed in the `CosmosChainProcessor` by parsing the comet events from every new block. This will be different for non-comet chains, but these items will need to be accounted for:

- For client IBC messages (e.g. MsgCreateClient, MsgUpdateClient, MsgUpgradeClient, MsgSubmitMisbehaviour), message should be parsed into `provider.ClientState`.
- For connection handshake IBC messages (e.g. MsgConnectionOpenInit, MsgConnectionOpenTry, MsgConnectionOpenAck, MsgConnectionOpenConfirm), message should be parsed into `provider.ConnectionInfo`
- For channel handshake IBC messages (e.g. MsgChannelOpenInit, MsgChannelOpenTry, MsgChannelOpenAck, MsgChannelOpenConfirm, MsgChannelCloseInit, MsgChannelCloseConfim), message should be parsed into `provider.ChannelInfo`
- For packet-flow IBC messages (e.g. MsgTransfer, MsgRecvPacket, MsgAcknowledgement), message should be parsed into `provider.PacketInfo`

#### Message Handlers

After IBC messages have been parsed from the blocks, some actions are necessary to keep the `ChainProcessor` local caches up to date and also construct the data that will be shared with the `PathProcessor`(s):

- For new packet messages that have been parsed into `provider.PacketInfo` by the event parsers or similar, check if the packet is relevant to any of the connected `PathProcessor`(s) by calling `IBCMessagesCache.PacketFlow.ShouldRetainSequence`. If true, retain the message with `IBCMessagesCache.PacketFlow.Retain`. This allows the relayer to avoid unnecessary processing by dropping packets that will not be ignored by all connected `PathProcessor`s.
- For client messages, update the `ChainProcessor` local `latestClientState` cache by storing the parsed `provider.ClientState` in the map for the client ID key.
- For connection handshake messages that have been parsed into `provider.ConnectionInfo`, update the `ChainProcessor` `connectionStateCache`. MsgConnectionOpenAck and MsgConnectionOpenConfirm mean the connection is open. MsgConnectionOpenInit and MsgConnectionOpenTry mean the connection is not open. Finally, retain the message unconditionally with `IBCMessagesCache.ConnectionHandshake.Retain`
- For channel handshake messages that have been parsed into `provider.ChannelInfo`, update the `ChainProcessor` `channelConnections` cache to save the connection ID for the channel. Additionally, update the `ChainProcessor` `channelStateCache` with the open state of the channel. MsgChannelOpenAck and MsgChannelOpenConfirm mean the channel is open. MsgChannelOpenInit, MsgChannelOpenTry, and MsgChannelCloseConfirm mean the channel is not open. Finally, retain the message unconditionally with `IBCMessagesCache.ChannelHandshake.Retain`


