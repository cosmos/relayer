package processor

import "github.com/cosmos/relayer/v2/relayer/provider"

// pathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which are
// processed during Run(ctx).
type pathEndRuntime struct {
	info PathEnd

	chainProvider provider.ChainProvider

	// cached data
	latestBlock          provider.LatestBlock
	messageCache         IBCMessagesCache
	clientState          provider.ClientState
	clientTrustedState   provider.ClientTrustedState
	connectionStateCache ConnectionStateCache
	channelStateCache    ChannelStateCache
	latestHeader         provider.IBCHeader
	ibcHeaderCache       IBCHeaderCache

	// New messages and other data arriving from the handleNewMessagesForPathEnd method.
	incomingCacheData chan ChainProcessorCacheData

	// Cache in progress sends for the different IBC message types
	// to avoid retrying a message immediately after it is sent.
	packetSendCache     packetSendCache
	connectionSendCache connectionSendCache
	channelSendCache    channelSendCache

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
}

func newPathEndRuntime(pathEnd PathEnd) *pathEndRuntime {
	return &pathEndRuntime{
		info:                 pathEnd,
		incomingCacheData:    make(chan ChainProcessorCacheData, 100),
		connectionStateCache: make(ConnectionStateCache),
		channelStateCache:    make(ChannelStateCache),
		messageCache:         NewIBCMessagesCache(),
		ibcHeaderCache:       make(IBCHeaderCache),
	}
}

func (pathEnd *pathEndRuntime) MergeCacheData(d ChainProcessorCacheData) {
	pathEnd.inSync = d.InSync
	pathEnd.latestBlock = d.LatestBlock
	pathEnd.latestHeader = d.LatestHeader
	pathEnd.clientState = d.ClientState

	// TODO messageCache Merge will likely need to move into the PathProcessor, and make sure passes client, connection, or channel filter
	pathEnd.messageCache.Merge(d.IBCMessagesCache) // Merge incoming packet IBC messages into the backlog

	// TODO connectionStateCache Merge will likely need to move into the PathProcessor, and make sure passes connection filter
	pathEnd.connectionStateCache.Merge(d.ConnectionStateCache) // Update latest connection open state for chain

	// TODO channelStateCache Merge will likely need to move into the PathProcessor, and make sure passes connection filter
	pathEnd.channelStateCache.Merge(d.ChannelStateCache) // Update latest channel open state for chain

	pathEnd.ibcHeaderCache.Merge(d.IBCHeaderCache)  // Update latest IBC header state
	pathEnd.ibcHeaderCache.Prune(ibcHeadersToCache) // Only keep most recent IBC headers
}
