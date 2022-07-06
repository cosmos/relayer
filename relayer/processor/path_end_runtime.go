package processor

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// pathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which are
// processed during Run(ctx).
type pathEndRuntime struct {
	log *zap.Logger

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
	packetProcessingCache     packetProcessingCache
	connectionProcessingCache connectionProcessingCache
	channelProcessingCache    channelProcessingCache

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
}

func newPathEndRuntime(log *zap.Logger, pathEnd PathEnd) *pathEndRuntime {
	return &pathEndRuntime{
		log: log.With(
			zap.String("chain_id", pathEnd.ChainID),
			zap.String("client_id", pathEnd.ClientID),
		),
		info:                      pathEnd,
		incomingCacheData:         make(chan ChainProcessorCacheData, 100),
		connectionStateCache:      make(ConnectionStateCache),
		channelStateCache:         make(ChannelStateCache),
		messageCache:              NewIBCMessagesCache(),
		ibcHeaderCache:            make(IBCHeaderCache),
		packetProcessingCache:     make(packetProcessingCache),
		connectionProcessingCache: make(connectionProcessingCache),
		channelProcessingCache:    make(channelProcessingCache),
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
	action := message.action
	sequence := message.info.Sequence
	channelKey := message.channelKey()
	if message.info.Height >= counterparty.latestBlock.Height {
		pathEnd.log.Debug("Waiting to relay packet message until counterparty height has incremented",
			zap.Any("channel", channelKey),
			zap.String("action", action),
			zap.Uint64("sequence", sequence),
		)
		return false
	}
	if !pathEnd.channelStateCache[channelKey] {
		// channel is not open, do not send
		pathEnd.log.Warn("Refusing to relay packet message because channel is not open",
			zap.Any("channel", channelKey),
			zap.String("action", action),
			zap.Uint64("sequence", sequence),
		)
		return false
	}
	msgProcessCache, ok := pathEnd.packetProcessingCache[channelKey]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	channelProcessingCache, ok := msgProcessCache[action]
	if !ok {
		// in progress cache does not exist for this action, so can send
		return true
	}
	inProgress, ok := channelProcessingCache[sequence]
	if !ok {
		// in progress cache does not exist for this sequence, so can send.
		return true
	}
	blocksSinceLastProcessed := pathEnd.latestBlock.Height - inProgress.lastProcessedHeight
	if inProgress.assembled {
		if blocksSinceLastProcessed < blocksToRetrySendAfter {
			// this message was sent less than blocksToRetrySendAfter ago, do not attempt to send again yet.
			return false
		}
	} else {
		if blocksSinceLastProcessed < blocksToRetryAssemblyAfter {
			// this message was sent less than blocksToRetryAssemblyAfter ago, do not attempt assembly again yet.
			return false
		}
	}
	if inProgress.retryCount == maxMessageSendRetries {
		pathEnd.log.Error("Giving up on sending packet message after max retries",
			zap.Any("channel", channelKey),
			zap.String("action", action),
			zap.Uint64("sequence", sequence),
			zap.Int("max_retries", maxMessageSendRetries),
		)
		// giving up on sending this packet flow message
		// remove all retention of this connection handshake in pathEnd.messagesCache.PacketFlow and counterparty
		toDelete := make(map[string][]uint64)
		toDeleteCounterparty := make(map[string][]uint64)
		switch action {
		case MsgRecvPacket:
			toDelete[MsgRecvPacket] = []uint64{sequence}
			toDeleteCounterparty[MsgTransfer] = []uint64{sequence}
		case MsgAcknowledgement, MsgTimeout, MsgTimeoutOnClose:
			toDelete[action] = []uint64{sequence}
			toDeleteCounterparty[MsgRecvPacket] = []uint64{sequence}
			toDelete[MsgTransfer] = []uint64{sequence}
		}
		// delete in progress send for this specific message
		pathEnd.packetProcessingCache[channelKey].deleteCachedMessages(map[string][]uint64{action: []uint64{sequence}})
		// delete all packet flow retention history for this sequence
		pathEnd.messageCache.PacketFlow[channelKey].DeleteCachedMessages(toDelete)
		counterparty.messageCache.PacketFlow[channelKey].DeleteCachedMessages(toDeleteCounterparty)
		return false
	}

	return true
}

// shouldSendConnectionMessage determines if the connection handshake message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendConnectionMessage(message connectionIBCMessage, counterparty *pathEndRuntime) bool {
	action := message.action
	connectionKey := connectionInfoConnectionKey(message.info).Counterparty()
	if message.info.Height >= counterparty.latestBlock.Height {
		pathEnd.log.Debug("Waiting to relay connection message until counterparty height has incremented",
			zap.Any("connection", connectionKey),
			zap.String("action", action),
		)
		return false
	}
	msgProcessCache, ok := pathEnd.connectionProcessingCache[action]
	if !ok {
		// in progress cache does not exist for this action, so can send.
		return true
	}
	inProgress, ok := msgProcessCache[connectionKey]
	if !ok {
		// in progress cache does not exist for this connection, so can send.
		return true
	}
	blocksSinceLastProcessed := pathEnd.latestBlock.Height - inProgress.lastProcessedHeight
	if inProgress.assembled {
		if blocksSinceLastProcessed < blocksToRetrySendAfter {
			// this message was sent less than blocksToRetrySendAfter ago, do not attempt to send again yet.
			return false
		}
	} else {
		if blocksSinceLastProcessed < blocksToRetryAssemblyAfter {
			// this message was sent less than blocksToRetryAssemblyAfter ago, do not attempt assembly again yet.
			return false
		}
	}
	if inProgress.retryCount == maxMessageSendRetries {
		pathEnd.log.Error("Giving up on sending connection message after max retries",
			zap.String("action", action),
		)
		// giving up on sending this connection handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ConnectionKey)
		toDeleteCounterparty := make(map[string][]ConnectionKey)
		switch action {
		case MsgConnectionOpenTry:
			toDeleteCounterparty[MsgConnectionOpenInit] = []ConnectionKey{connectionKey}
		case MsgConnectionOpenAck:
			toDeleteCounterparty[MsgConnectionOpenTry] = []ConnectionKey{connectionKey}
			toDelete[MsgConnectionOpenInit] = []ConnectionKey{connectionKey}
		case MsgConnectionOpenConfirm:
			toDeleteCounterparty[MsgConnectionOpenAck] = []ConnectionKey{connectionKey}
			toDelete[MsgConnectionOpenTry] = []ConnectionKey{connectionKey}
			toDeleteCounterparty[MsgConnectionOpenInit] = []ConnectionKey{connectionKey}
		}
		// delete in progress send for this specific message
		pathEnd.connectionProcessingCache.deleteCachedMessages(map[string][]ConnectionKey{action: []ConnectionKey{connectionKey}})

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
	action := message.action
	channelKey := channelInfoChannelKey(message.info).Counterparty()
	if message.info.Height >= counterparty.latestBlock.Height {
		pathEnd.log.Debug("Waiting to relay channel message until counterparty height has incremented",
			zap.Any("channel", channelKey),
			zap.String("action", action),
		)
		return false
	}
	msgProcessCache, ok := pathEnd.channelProcessingCache[action]
	if !ok {
		// in progress cache does not exist for this action, so can send.
		return true
	}
	inProgress, ok := msgProcessCache[channelKey]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	blocksSinceLastProcessed := pathEnd.latestBlock.Height - inProgress.lastProcessedHeight
	if inProgress.assembled {
		if blocksSinceLastProcessed < blocksToRetrySendAfter {
			// this message was sent less than blocksToRetrySendAfter ago, do not attempt to send again yet.
			return false
		}
	} else {
		if blocksSinceLastProcessed < blocksToRetryAssemblyAfter {
			// this message was sent less than blocksToRetryAssemblyAfter ago, do not attempt assembly again yet.
			return false
		}
	}
	if inProgress.retryCount == maxMessageSendRetries {
		pathEnd.log.Error("Giving up on sending channel message after max retries",
			zap.String("action", action),
			zap.Int("max_retries", maxMessageSendRetries),
		)
		// giving up on sending this channel handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ChannelKey)
		toDeleteCounterparty := make(map[string][]ChannelKey)
		switch action {
		case MsgChannelOpenTry:
			toDeleteCounterparty[MsgChannelOpenInit] = []ChannelKey{channelKey}
		case MsgChannelOpenAck:
			toDeleteCounterparty[MsgChannelOpenTry] = []ChannelKey{channelKey}
			toDelete[MsgChannelOpenInit] = []ChannelKey{channelKey}
		case MsgChannelOpenConfirm:
			toDeleteCounterparty[MsgChannelOpenAck] = []ChannelKey{channelKey}
			toDelete[MsgChannelOpenTry] = []ChannelKey{channelKey}
			toDeleteCounterparty[MsgChannelOpenInit] = []ChannelKey{channelKey}
		}
		// delete in progress send for this specific message
		pathEnd.channelProcessingCache.deleteCachedMessages(map[string][]ChannelKey{action: []ChannelKey{channelKey}})

		// delete all connection handshake retention history for this channel
		pathEnd.messageCache.ChannelHandshake.DeleteCachedMessages(toDelete)
		counterparty.messageCache.ChannelHandshake.DeleteCachedMessages(toDeleteCounterparty)

		return false
	}

	return true
}

func (pathEnd *pathEndRuntime) trackProcessingPacketMessage(message packetIBCMessage, assembled bool) {
	action := message.action
	sequence := message.info.Sequence
	channelKey := message.channelKey()
	msgProcessCache, ok := pathEnd.packetProcessingCache[channelKey]
	if !ok {
		msgProcessCache = make(packetChannelMessageCache)
		pathEnd.packetProcessingCache[channelKey] = msgProcessCache
	}
	channelProcessingCache, ok := msgProcessCache[action]
	if !ok {
		channelProcessingCache = make(packetMessageSendCache)
		msgProcessCache[action] = channelProcessingCache
	}

	retryCount := uint64(0)

	if inProgress, ok := channelProcessingCache[sequence]; ok {
		retryCount = inProgress.retryCount + 1
	}

	channelProcessingCache[sequence] = processingMessage{
		lastProcessedHeight: pathEnd.latestBlock.Height,
		retryCount:          retryCount,
		assembled:           assembled,
	}
}

func (pathEnd *pathEndRuntime) trackProcessingConnectionMessage(message connectionIBCMessage, assembled bool) {
	action := message.action
	connectionKey := connectionInfoConnectionKey(message.info).Counterparty()
	msgProcessCache, ok := pathEnd.connectionProcessingCache[action]
	if !ok {
		msgProcessCache = make(connectionKeySendCache)
		pathEnd.connectionProcessingCache[action] = msgProcessCache
	}

	retryCount := uint64(0)

	if inProgress, ok := msgProcessCache[connectionKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgProcessCache[connectionKey] = processingMessage{
		lastProcessedHeight: pathEnd.latestBlock.Height,
		retryCount:          retryCount,
		assembled:           assembled,
	}
}

func (pathEnd *pathEndRuntime) trackProcessingChannelMessage(message channelIBCMessage, assembled bool) {
	action := message.action
	channelKey := channelInfoChannelKey(message.info).Counterparty()
	msgProcessCache, ok := pathEnd.channelProcessingCache[action]
	if !ok {
		msgProcessCache = make(channelKeySendCache)
		pathEnd.channelProcessingCache[action] = msgProcessCache
	}

	retryCount := uint64(0)

	if inProgress, ok := msgProcessCache[channelKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgProcessCache[channelKey] = processingMessage{
		lastProcessedHeight: pathEnd.latestBlock.Height,
		retryCount:          retryCount,
		assembled:           assembled,
	}
}
