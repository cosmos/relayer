package processor

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// pathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which are
// processed during Run(ctx).
type pathEndRuntime struct {
	log *zap.Logger

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

	// Cache processing messages for the different IBC message types
	// to avoid retrying message processing too soon after
	// attempting to assemble or send a message.
	packetProcessingCache     packetProcessingCache
	connectionProcessingCache connectionProcessingCache
	channelProcessingCache    channelProcessingCache

	connectionMessageSubscribers map[string][]func(provider.ConnectionInfo)
	channelMessageSubscribers    map[string][]func(provider.ChannelInfo)
	packetMessageSubscribers     map[string][]func(provider.PacketInfo)

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
}

func newPathEndRuntime(log *zap.Logger, pathEnd PathEnd) *pathEndRuntime {
	return &pathEndRuntime{
		info:                         pathEnd,
		log:                          log.With(zap.String("chain_id", pathEnd.ChainID), zap.String("client_id", pathEnd.ClientID)),
		incomingCacheData:            make(chan ChainProcessorCacheData, 100),
		connectionStateCache:         make(ConnectionStateCache),
		channelStateCache:            make(ChannelStateCache),
		messageCache:                 NewIBCMessagesCache(),
		ibcHeaderCache:               make(IBCHeaderCache),
		packetProcessingCache:        make(packetProcessingCache),
		connectionProcessingCache:    make(connectionProcessingCache),
		channelProcessingCache:       make(channelProcessingCache),
		connectionMessageSubscribers: make(map[string][]func(provider.ConnectionInfo)),
		channelMessageSubscribers:    make(map[string][]func(provider.ChannelInfo)),
		packetMessageSubscribers:     make(map[string][]func(provider.PacketInfo)),
	}
}

func (pathEnd *pathEndRuntime) isRelevantConnection(connectionID string) bool {
	for k := range pathEnd.connectionStateCache {
		if k.ConnectionID == connectionID {
			return true
		}
	}
	return false
}

func (pathEnd *pathEndRuntime) isRelevantChannel(channelID string) bool {
	for k := range pathEnd.channelStateCache {
		if k.ChannelID == channelID {
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
		for k, ci := range cmc {
			if pathEnd.isRelevantConnection(k.ConnectionID) {
				// can complete connection handshakes on this client
				// since PathProcessor holds reference to the counterparty chain pathEndRuntime.
				newCmc[k] = ci
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
		for k, ci := range cmc {
			if pathEnd.isRelevantChannel(k.ChannelID) {
				// can complete channel handshakes on this client
				// since PathProcessor holds reference to the counterparty chain pathEndRuntime.
				newCmc[k] = ci
			}
		}
		if len(newCmc) == 0 {
			continue
		}
		channelHandshakeMessages[action] = newCmc
	}
	pathEnd.messageCache.ChannelHandshake.Merge(channelHandshakeMessages)
}

func (pathEnd *pathEndRuntime) handleCallbacks(c IBCMessagesCache) {
	if len(pathEnd.connectionMessageSubscribers) > 0 {
		for action, m := range c.ConnectionHandshake {
			subscribers, ok := pathEnd.connectionMessageSubscribers[action]
			if !ok {
				continue
			}
			for _, ci := range m {
				for _, subscriber := range subscribers {
					subscriber(ci)
				}
			}
		}
	}
	if len(pathEnd.channelMessageSubscribers) > 0 {
		for action, m := range c.ChannelHandshake {
			subscribers, ok := pathEnd.channelMessageSubscribers[action]
			if !ok {
				continue
			}
			for _, ci := range m {
				for _, subscriber := range subscribers {
					subscriber(ci)
				}
			}
		}
	}
	if len(pathEnd.packetMessageSubscribers) > 0 {
		for _, m := range c.PacketFlow {
			for action, s := range m {
				subscribers, ok := pathEnd.packetMessageSubscribers[action]
				if !ok {
					continue
				}
				for _, pi := range s {
					for _, subscriber := range subscribers {
						subscriber(pi)
					}
				}
			}
		}
	}
}

func (pathEnd *pathEndRuntime) shouldTerminate(ibcMessagesCache IBCMessagesCache, messageLifecycle MessageLifecycle) bool {
	if messageLifecycle == nil {
		return false
	}
	switch m := messageLifecycle.(type) {
	case *PacketMessageLifecycle:
		if m.Termination.ChainID != pathEnd.info.ChainID {
			return false
		}
		actionCache, ok := ibcMessagesCache.PacketFlow[PacketInfoChannelKey(m.Termination.Action, m.Termination.Info)]
		if !ok {
			return false
		}
		sequenceCache, ok := actionCache[m.Termination.Action]
		if !ok {
			return false
		}
		_, ok = sequenceCache[m.Termination.Info.Sequence]
		if !ok {
			return false
		}
		// stop path processor, condition fulfilled
		pathEnd.log.Info("Found termination condition for packet flow")
		return true
	case *ChannelMessageLifecycle:
		if m.Termination.ChainID != pathEnd.info.ChainID {
			return false
		}
		cache, ok := ibcMessagesCache.ChannelHandshake[m.Termination.Action]
		if !ok {
			return false
		}
		// check against m.Termination.Info
		foundChannelID := m.Termination.Info.ChannelID == ""
		foundPortID := m.Termination.Info.PortID == ""
		foundCounterpartyChannelID := m.Termination.Info.CounterpartyChannelID == ""
		foundCounterpartyPortID := m.Termination.Info.CounterpartyPortID == ""
		for _, ci := range cache {
			if !foundChannelID && ci.ChannelID == m.Termination.Info.ChannelID {
				foundChannelID = true
			}
			if !foundPortID && ci.PortID == m.Termination.Info.PortID {
				foundPortID = true
			}
			if !foundCounterpartyChannelID && ci.CounterpartyChannelID == m.Termination.Info.CounterpartyChannelID {
				foundCounterpartyChannelID = true
			}
			if !foundCounterpartyPortID && ci.CounterpartyPortID == m.Termination.Info.CounterpartyPortID {
				foundCounterpartyPortID = true
			}
		}
		if foundChannelID && foundPortID && foundCounterpartyChannelID && foundCounterpartyPortID {
			pathEnd.log.Info("Found termination condition for channel handshake")
			return true
		}
	case *ConnectionMessageLifecycle:
		if m.Termination.ChainID != pathEnd.info.ChainID {
			return false
		}
		cache, ok := ibcMessagesCache.ConnectionHandshake[m.Termination.Action]
		if !ok {
			return false
		}
		// check against m.Termination.Info
		foundClientID := m.Termination.Info.ClientID == ""
		foundConnectionID := m.Termination.Info.ConnectionID == ""
		foundCounterpartyClientID := m.Termination.Info.CounterpartyClientID == ""
		foundCounterpartyConnectionID := m.Termination.Info.CounterpartyConnectionID == ""
		for _, ci := range cache {
			pathEnd.log.Info("Connection handshake termination candidate",
				zap.String("termination_client_id", m.Termination.Info.ClientID),
				zap.String("observed_client_id", ci.ClientID),
				zap.String("termination_counterparty_client_id", m.Termination.Info.CounterpartyClientID),
				zap.String("observed_counterparty_client_id", ci.CounterpartyClientID),
			)
			if !foundClientID && ci.ClientID == m.Termination.Info.ClientID {
				foundClientID = true
			}
			if !foundConnectionID && ci.ConnectionID == m.Termination.Info.ConnectionID {
				foundConnectionID = true
			}
			if !foundCounterpartyClientID && ci.CounterpartyClientID == m.Termination.Info.CounterpartyClientID {
				foundCounterpartyClientID = true
			}
			if !foundCounterpartyConnectionID && ci.CounterpartyConnectionID == m.Termination.Info.CounterpartyConnectionID {
				foundCounterpartyConnectionID = true
			}
		}
		if foundClientID && foundConnectionID && foundCounterpartyClientID && foundCounterpartyConnectionID {
			pathEnd.log.Info("Found termination condition for connection handshake")
			return true
		} else {
			pathEnd.log.Info("Did not find termination condition for connection handshake",
				zap.Bool("foundClientID", foundClientID),
				zap.Bool("foundConnectionID", foundConnectionID),
				zap.Bool("foundCounterpartyClientID", foundCounterpartyClientID),
				zap.Bool("foundCounterpartyConnectionID", foundCounterpartyConnectionID),
			)
		}
	}
	return false
}

func (pathEnd *pathEndRuntime) MergeCacheData(ctx context.Context, ctxCancel func(), d ChainProcessorCacheData, messageLifecycle MessageLifecycle) {
	pathEnd.inSync = d.InSync
	pathEnd.latestBlock = d.LatestBlock
	pathEnd.latestHeader = d.LatestHeader
	pathEnd.clientState = d.ClientState

	pathEnd.handleCallbacks(d.IBCMessagesCache)

	if pathEnd.shouldTerminate(d.IBCMessagesCache, messageLifecycle) {
		ctxCancel()
		return
	}

	pathEnd.connectionStateCache.Merge(d.ConnectionStateCache) // Update latest connection open state for chain
	pathEnd.channelStateCache.Merge(d.ChannelStateCache)       // Update latest channel open state for chain

	pathEnd.mergeMessageCache(d.IBCMessagesCache) // Merge incoming packet IBC messages into the backlog

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

		// TODO delete all retention of this connection handshake

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

		// TODO delete all retention of this connection handshake

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
