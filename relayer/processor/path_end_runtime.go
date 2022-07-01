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
		packetSendCache:      make(packetSendCache),
		connectionSendCache:  make(connectionSendCache),
		channelSendCache:     make(channelSendCache),
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
		if pathEnd.info.ShouldRelayChannel(ch) {
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


// shouldSendPacketMessage determines if the packet flow message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendPacketMessage(message packetIBCMessage, counterparty *pathEndRuntime) bool {
	if !pathEnd.channelStateCache[message.channelKey] {
		// channel is not open, do not send
		return false
	}
	msgSendCache, ok := pathEnd.packetSendCache[message.channelKey]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	channelSendCache, ok := msgSendCache[message.action]
	if !ok {
		// in progress cache does not exist for this action, so can send
		return true
	}
	inProgress, ok := channelSendCache[message.sequence]
	if !ok {
		// in progress cache does not exist for this sequence, so can send.
		return true
	}
	if pathEnd.latestBlock.Height-inProgress.sendHeight < blocksToRetryPacketAfter {
		// this message was sent less than blocksToRetryPacketAfter ago, do not send again yet.
		return false
	}
	if inProgress.retryCount == maxMessageSendRetries {
		// giving up on sending this packet flow message
		// remove all retention of this connection handshake in pathEnd.messagesCache.PacketFlow and counterparty
		toDelete := make(map[string][]uint64)
		toDeleteCounterparty := make(map[string][]uint64)
		switch message.action {
		case MsgRecvPacket:
			toDelete[MsgRecvPacket] = []uint64{message.sequence}
			toDeleteCounterparty[MsgTransfer] = []uint64{message.sequence}
		case MsgAcknowledgement, MsgTimeout, MsgTimeoutOnClose:
			toDelete[message.action] = []uint64{message.sequence}
			toDeleteCounterparty[MsgRecvPacket] = []uint64{message.sequence}
			toDelete[MsgTransfer] = []uint64{message.sequence}
		}
		// delete in progress send for this specific message
		pathEnd.packetSendCache[message.channelKey].deleteCachedMessages(map[string][]uint64{message.action: []uint64{message.sequence}})
		// delete all packet flow retention history for this sequence
		pathEnd.messageCache.PacketFlow[message.channelKey].DeleteCachedMessages(toDelete)
		counterparty.messageCache.PacketFlow[message.channelKey].DeleteCachedMessages(toDeleteCounterparty)
		return false
	}

	return true
}

// shouldSendConnectionMessage determines if the connection handshake message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendConnectionMessage(message connectionIBCMessage, counterparty *pathEndRuntime) bool {
	msgSendCache, ok := pathEnd.connectionSendCache[message.action]
	if !ok {
		// in progress cache does not exist for this action, so can send.
		return true
	}
	inProgress, ok := msgSendCache[message.connectionKey]
	if !ok {
		// in progress cache does not exist for this connection, so can send.
		return true
	}
	if pathEnd.latestBlock.Height-inProgress.sendHeight < blocksToRetryPacketAfter {
		// this message was sent less than blocksToRetryPacketAfter ago, do not send again yet.
		return false
	}
	if inProgress.retryCount == maxMessageSendRetries {
		// giving up on sending this connection handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ConnectionKey)
		toDeleteCounterparty := make(map[string][]ConnectionKey)
		switch message.action {
		case MsgConnectionOpenTry:
			toDeleteCounterparty[MsgConnectionOpenInit] = []ConnectionKey{message.connectionKey}
		case MsgConnectionOpenAck:
			toDeleteCounterparty[MsgConnectionOpenTry] = []ConnectionKey{message.connectionKey}
			toDelete[MsgConnectionOpenInit] = []ConnectionKey{message.connectionKey}
		case MsgConnectionOpenConfirm:
			toDeleteCounterparty[MsgConnectionOpenAck] = []ConnectionKey{message.connectionKey}
			toDelete[MsgConnectionOpenTry] = []ConnectionKey{message.connectionKey}
			toDeleteCounterparty[MsgConnectionOpenInit] = []ConnectionKey{message.connectionKey}
		}
		// delete in progress send for this specific message
		pathEnd.connectionSendCache.deleteCachedMessages(map[string][]ConnectionKey{message.action: []ConnectionKey{message.connectionKey}})
		// delete all connection handshake retention history for this connection
		pathEnd.messageCache.ConnectionHandshake.DeleteCachedMessages(toDelete)
		counterparty.messageCache.ConnectionHandshake.DeleteCachedMessages(toDeleteCounterparty)
		return false
	}

	return true
}

// shouldSendConnectionMessage determines if the channel handshake message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendChannelMessage(message channelIBCMessage, counterparty *pathEndRuntime) bool {
	msgSendCache, ok := pathEnd.channelSendCache[message.action]
	if !ok {
		// in progress cache does not exist for this action, so can send.
		return true
	}
	inProgress, ok := msgSendCache[message.channelKey]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	if pathEnd.latestBlock.Height-inProgress.sendHeight < blocksToRetryPacketAfter {
		// this message was sent less than blocksToRetryPacketAfter ago, do not send again yet.
		return false
	}
	if inProgress.retryCount == maxMessageSendRetries {
		// giving up on sending this channel handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ChannelKey)
		toDeleteCounterparty := make(map[string][]ChannelKey)
		switch message.action {
		case MsgChannelOpenTry:
			toDeleteCounterparty[MsgChannelOpenInit] = []ChannelKey{message.channelKey}
		case MsgChannelOpenAck:
			toDeleteCounterparty[MsgChannelOpenTry] = []ChannelKey{message.channelKey}
			toDelete[MsgChannelOpenInit] = []ChannelKey{message.channelKey}
		case MsgChannelOpenConfirm:
			toDeleteCounterparty[MsgChannelOpenAck] = []ChannelKey{message.channelKey}
			toDelete[MsgChannelOpenTry] = []ChannelKey{message.channelKey}
			toDeleteCounterparty[MsgChannelOpenInit] = []ChannelKey{message.channelKey}
		}
		// delete in progress send for this specific message
		pathEnd.channelSendCache.deleteCachedMessages(map[string][]ChannelKey{message.action: []ChannelKey{message.channelKey}})
		// delete all connection handshake retention history for this channel
		pathEnd.messageCache.ChannelHandshake.DeleteCachedMessages(toDelete)
		counterparty.messageCache.ChannelHandshake.DeleteCachedMessages(toDeleteCounterparty)
		return false
	}

	return true
}

func (pathEnd *pathEndRuntime) trackSentPacketMessage(message packetIBCMessage) {
	msgSendCache, ok := pathEnd.packetSendCache[message.channelKey]
	if !ok {
		msgSendCache = make(packetChannelMessageCache)
		pathEnd.packetSendCache[message.channelKey] = msgSendCache
	}
	channelSendCache, ok := msgSendCache[message.action]
	if !ok {
		channelSendCache = make(packetMessageSendCache)
		msgSendCache[message.action] = channelSendCache
	}

	retryCount := uint64(0)

	if inProgress, ok := channelSendCache[message.sequence]; ok {
		retryCount = inProgress.retryCount + 1
	}

	channelSendCache[message.sequence] = inProgressSend{
		sendHeight: pathEnd.latestBlock.Height,
		message:    message.message,
		retryCount: retryCount,
	}
}

func (pathEnd *pathEndRuntime) trackSentConnectionMessage(message connectionIBCMessage) {
	msgSendCache, ok := pathEnd.connectionSendCache[message.action]
	if !ok {
		msgSendCache = make(connectionKeySendCache)
		pathEnd.connectionSendCache[message.action] = msgSendCache
	}

	retryCount := uint64(0)

	if inProgress, ok := msgSendCache[message.connectionKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgSendCache[message.connectionKey] = inProgressSend{
		sendHeight: pathEnd.latestBlock.Height,
		message:    message.message,
		retryCount: retryCount,
	}
}

func (pathEnd *pathEndRuntime) trackSentChannelMessage(message channelIBCMessage) {
	msgSendCache, ok := pathEnd.channelSendCache[message.action]
	if !ok {
		msgSendCache = make(channelKeySendCache)
		pathEnd.channelSendCache[message.action] = msgSendCache
	}

	retryCount := uint64(0)

	if inProgress, ok := msgSendCache[message.channelKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgSendCache[message.channelKey] = inProgressSend{
		sendHeight: pathEnd.latestBlock.Height,
		message:    message.message,
		retryCount: retryCount,
	}
}
