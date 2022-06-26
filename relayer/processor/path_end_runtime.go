package processor

import "github.com/cosmos/relayer/v2/relayer/provider"

// pathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which are
// processed during Run(ctx).
type pathEndRuntime struct {
	info PathEnd

	// TODO populate known connections and channels as they are discovered on this client.
	// channel handshakes will not work until these are populated properly.

	// includes all known and discovered connections that use this PathEnd's client.
	knownConnections []string
	// includes all known and discovered channels on connections that use this PathEnd's client.
	knownChannels []string

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

func (pathEnd *pathEndRuntime) isRelevantConnection(connectionID string) bool {
	for _, conn := range pathEnd.knownConnections {
		if conn == connectionID {
			return true
		}
	}
	return false
}

// mergeMessageCache merges relevant IBC messages for packet flows, connection handshakes, and channel handshakes.
func (pathEnd *pathEndRuntime) mergeMessageCache(messageCache IBCMessagesCache) {
	packetMessages := make(ChannelPacketMessagesCache)
	connectionHandshakeMessages := make(ConnectionMessagesCache)
	channelHandshakeMessages := make(ChannelMessagesCache)

	for ch, pmc := range messageCache.PacketFlow {
		if !pathEnd.info.ShouldRelayChannel(ch) {
			packetMessages[ch] = pmc
		}
	}
	pathEnd.messageCache.PacketFlow.Merge(packetMessages)

	for action, cmc := range messageCache.ConnectionHandshake {
		newCmc := make(ConnectionMessageCache)
		for conn, msg := range cmc {
			if pathEnd.info.ClientID == conn.ClientID {
				// can complete connection handshakes on this client
				// since PathProcessor holds reference to the counterparty chain pathEndRuntime.
				newCmc[conn] = msg
				break
			}
		}
		if len(newCmc) == 0 {
			continue
		}
		connectionHandshakeMessages[action] = newCmc
	}
	pathEnd.messageCache.ConnectionHandshake.Merge(connectionHandshakeMessages)

	for action, cmc := range messageCache.ChannelHandshake {
		newCmc := make(ChannelMessageCache)
		for ch, msg := range cmc {
			for _, chID := range pathEnd.knownChannels {
				if chID == ch.ChannelID {
					// if channel's connection is known, and connection is on top of this client,
					// then we can complete channel handshake for this channel
					// since PathProcessor holds reference to the counterparty chain pathEndRuntime.
					newCmc[ch] = msg
					break
				}
			}
		}
		if len(newCmc) == 0 {
			continue
		}
		channelHandshakeMessages[action] = newCmc
	}
	pathEnd.messageCache.ChannelHandshake.Merge(channelHandshakeMessages)
}

func (pathEnd *pathEndRuntime) MergeCacheData(d ChainProcessorCacheData) {
	pathEnd.inSync = d.InSync
	pathEnd.latestBlock = d.LatestBlock
	pathEnd.latestHeader = d.LatestHeader
	pathEnd.clientState = d.ClientState

	pathEnd.mergeMessageCache(d.IBCMessagesCache) // Merge incoming packet IBC messages into the backlog

	pathEnd.connectionStateCache.Merge(d.ConnectionStateCache) // Update latest connection open state for chain
	pathEnd.channelStateCache.Merge(d.ChannelStateCache)       // Update latest channel open state for chain

	pathEnd.ibcHeaderCache.Merge(d.IBCHeaderCache)  // Update latest IBC header state
	pathEnd.ibcHeaderCache.Prune(ibcHeadersToCache) // Only keep most recent IBC headers
}
