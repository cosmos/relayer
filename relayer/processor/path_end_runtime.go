package processor

import (
	"context"
	"time"

	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
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

	// Cache in progress sends for the different IBC message types
	// to avoid retrying a message immediately after it is sent.
	packetProcessing  packetProcessingCache
	connProcessing    connectionProcessingCache
	channelProcessing channelProcessingCache

	// Message subscriber callbacks
	connSubscribers map[string][]func(provider.ConnectionInfo)

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool

	metrics *PrometheusMetrics
}

func newPathEndRuntime(log *zap.Logger, pathEnd PathEnd, metrics *PrometheusMetrics) *pathEndRuntime {
	return &pathEndRuntime{
		log: log.With(
			zap.String("path_name", pathEnd.PathName),
			zap.String("chain_id", pathEnd.ChainID),
			zap.String("client_id", pathEnd.ClientID),
		),
		info:                 pathEnd,
		incomingCacheData:    make(chan ChainProcessorCacheData, 100),
		connectionStateCache: make(ConnectionStateCache),
		channelStateCache:    make(ChannelStateCache),
		messageCache:         NewIBCMessagesCache(),
		ibcHeaderCache:       make(IBCHeaderCache),
		packetProcessing:     make(packetProcessingCache),
		connProcessing:       make(connectionProcessingCache),
		channelProcessing:    make(channelProcessingCache),
		connSubscribers:      make(map[string][]func(provider.ConnectionInfo)),
		metrics:              metrics,
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
// inSync indicates whether both involved ChainProcessors are in sync or not. When true, the observed packets
// metrics will be counted so that observed vs relayed packets can be compared.
func (pathEnd *pathEndRuntime) mergeMessageCache(messageCache IBCMessagesCache, counterpartyChainID string, inSync bool) {
	packetMessages := make(ChannelPacketMessagesCache)
	connectionHandshakeMessages := make(ConnectionMessagesCache)
	channelHandshakeMessages := make(ChannelMessagesCache)

	for ch, pmc := range messageCache.PacketFlow {
		if pathEnd.info.ShouldRelayChannel(ChainChannelKey{ChainID: pathEnd.info.ChainID, CounterpartyChainID: counterpartyChainID, ChannelKey: ch}) {
			if inSync && pathEnd.metrics != nil {
				for eventType, pCache := range pmc {
					pathEnd.metrics.AddPacketsObserved(pathEnd.info.PathName, pathEnd.info.ChainID, ch.ChannelID, ch.PortID, eventType, len(pCache))
				}
			}
			packetMessages[ch] = pmc
		}
	}
	pathEnd.messageCache.PacketFlow.Merge(packetMessages)

	for eventType, cmc := range messageCache.ConnectionHandshake {
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
		connectionHandshakeMessages[eventType] = newCmc
	}
	pathEnd.messageCache.ConnectionHandshake.Merge(connectionHandshakeMessages)

	for eventType, cmc := range messageCache.ChannelHandshake {
		newCmc := make(ChannelMessageCache)
		for k, ci := range cmc {
			if !pathEnd.isRelevantChannel(k.ChannelID) {
				continue
			}
			// can complete channel handshakes on this client
			// since PathProcessor holds reference to the counterparty chain pathEndRuntime.
			if eventType == chantypes.EventTypeChannelOpenInit {
				// CounterpartyConnectionID is needed to construct MsgChannelOpenTry.
				for k := range pathEnd.connectionStateCache {
					if k.ConnectionID == ci.ConnID {
						ci.CounterpartyConnID = k.CounterpartyConnID
						break
					}
				}
			}
			newCmc[k] = ci
		}
		if len(newCmc) == 0 {
			continue
		}

		channelHandshakeMessages[eventType] = newCmc
	}
	pathEnd.messageCache.ChannelHandshake.Merge(channelHandshakeMessages)
}

func (pathEnd *pathEndRuntime) handleCallbacks(c IBCMessagesCache) {
	if len(pathEnd.connSubscribers) > 0 {
		for eventType, m := range c.ConnectionHandshake {
			subscribers, ok := pathEnd.connSubscribers[eventType]
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
}

func (pathEnd *pathEndRuntime) shouldTerminate(ibcMessagesCache IBCMessagesCache, messageLifecycle MessageLifecycle) bool {
	if messageLifecycle == nil {
		return false
	}
	switch m := messageLifecycle.(type) {
	case *PacketMessageLifecycle:
		if m.Termination == nil || m.Termination.ChainID != pathEnd.info.ChainID {
			return false
		}
		channelKey, err := PacketInfoChannelKey(m.Termination.EventType, m.Termination.Info)
		if err != nil {
			pathEnd.log.Error("Unexpected error checking packet message",
				zap.String("event_type", m.Termination.EventType),
				zap.Inline(channelKey),
				zap.Error(err),
			)
			return false
		}
		eventTypeCache, ok := ibcMessagesCache.PacketFlow[channelKey]
		if !ok {
			return false
		}
		sequenceCache, ok := eventTypeCache[m.Termination.EventType]
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
		if m.Termination == nil || m.Termination.ChainID != pathEnd.info.ChainID {
			return false
		}
		cache, ok := ibcMessagesCache.ChannelHandshake[m.Termination.EventType]
		if !ok {
			return false
		}
		// check against m.Termination.Info
		foundChannelID := m.Termination.Info.ChannelID == ""
		foundPortID := m.Termination.Info.PortID == ""
		foundCounterpartyChannelID := m.Termination.Info.CounterpartyChannelID == ""
		foundCounterpartyPortID := m.Termination.Info.CounterpartyPortID == ""
		for _, ci := range cache {
			if ci.ChannelID == m.Termination.Info.ChannelID {
				foundChannelID = true
			}
			if ci.PortID == m.Termination.Info.PortID {
				foundPortID = true
			}
			if ci.CounterpartyChannelID == m.Termination.Info.CounterpartyChannelID {
				foundCounterpartyChannelID = true
			}
			if ci.CounterpartyPortID == m.Termination.Info.CounterpartyPortID {
				foundCounterpartyPortID = true
			}
		}
		if foundChannelID && foundPortID && foundCounterpartyChannelID && foundCounterpartyPortID {
			pathEnd.log.Info("Found termination condition for channel handshake")
			return true
		}
	case *ConnectionMessageLifecycle:
		if m.Termination == nil || m.Termination.ChainID != pathEnd.info.ChainID {
			return false
		}
		cache, ok := ibcMessagesCache.ConnectionHandshake[m.Termination.EventType]
		if !ok {
			return false
		}
		// check against m.Termination.Info
		foundClientID := m.Termination.Info.ClientID == ""
		foundConnectionID := m.Termination.Info.ConnID == ""
		foundCounterpartyClientID := m.Termination.Info.CounterpartyClientID == ""
		foundCounterpartyConnectionID := m.Termination.Info.CounterpartyConnID == ""
		for _, ci := range cache {
			pathEnd.log.Info("Connection handshake termination candidate",
				zap.String("termination_client_id", m.Termination.Info.ClientID),
				zap.String("observed_client_id", ci.ClientID),
				zap.String("termination_counterparty_client_id", m.Termination.Info.CounterpartyClientID),
				zap.String("observed_counterparty_client_id", ci.CounterpartyClientID),
			)
			if ci.ClientID == m.Termination.Info.ClientID {
				foundClientID = true
			}
			if ci.ConnID == m.Termination.Info.ConnID {
				foundConnectionID = true
			}
			if ci.CounterpartyClientID == m.Termination.Info.CounterpartyClientID {
				foundCounterpartyClientID = true
			}
			if ci.CounterpartyConnID == m.Termination.Info.CounterpartyConnID {
				foundCounterpartyConnectionID = true
			}
		}
		if foundClientID && foundConnectionID && foundCounterpartyClientID && foundCounterpartyConnectionID {
			pathEnd.log.Info("Found termination condition for connection handshake")
			return true
		}
	}
	return false
}

func (pathEnd *pathEndRuntime) mergeCacheData(ctx context.Context, cancel func(), d ChainProcessorCacheData, counterpartyChainID string, counterpartyInSync bool, messageLifecycle MessageLifecycle, counterParty *pathEndRuntime) {
	pathEnd.inSync = d.InSync
	pathEnd.latestBlock = d.LatestBlock
	pathEnd.latestHeader = d.LatestHeader
	pathEnd.clientState = d.ClientState
	if d.ClientState.ConsensusHeight != pathEnd.clientState.ConsensusHeight {
		pathEnd.clientState = d.ClientState
		ibcHeader, ok := counterParty.ibcHeaderCache[d.ClientState.ConsensusHeight.RevisionHeight]
		if ok {
			pathEnd.clientState.ConsensusTime = time.Unix(0, int64(ibcHeader.ConsensusState().GetTimestamp()))
		}
	}

	pathEnd.handleCallbacks(d.IBCMessagesCache)

	if pathEnd.shouldTerminate(d.IBCMessagesCache, messageLifecycle) {
		cancel()
		return
	}

	pathEnd.connectionStateCache = d.ConnectionStateCache // Update latest connection open state for chain
	pathEnd.channelStateCache = d.ChannelStateCache       // Update latest channel open state for chain

	pathEnd.mergeMessageCache(d.IBCMessagesCache, counterpartyChainID, pathEnd.inSync && counterpartyInSync) // Merge incoming packet IBC messages into the backlog

	pathEnd.ibcHeaderCache.Merge(d.IBCHeaderCache)  // Update latest IBC header state
	pathEnd.ibcHeaderCache.Prune(ibcHeadersToCache) // Only keep most recent IBC headers
}

// shouldSendPacketMessage determines if the packet flow message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendPacketMessage(message packetIBCMessage, counterparty *pathEndRuntime) bool {
	eventType := message.eventType
	sequence := message.info.Sequence
	k, err := message.channelKey()
	if err != nil {
		pathEnd.log.Error("Unexpected error checking if should send packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", sequence),
			zap.Inline(k),
			zap.Error(err),
		)
		return false
	}
	if message.info.Height >= counterparty.latestBlock.Height {
		pathEnd.log.Debug("Waiting to relay packet message until counterparty height has incremented",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", sequence),
			zap.Inline(k),
		)
		return false
	}
	if !pathEnd.channelStateCache[k] {
		// channel is not open, do not send
		pathEnd.log.Warn("Refusing to relay packet message because channel is not open",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", sequence),
			zap.Inline(k),
		)
		return false
	}
	msgProcessCache, ok := pathEnd.packetProcessing[k]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	channelProcessingCache, ok := msgProcessCache[eventType]
	if !ok {
		// in progress cache does not exist for this eventType, so can send
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
	if inProgress.retryCount >= maxMessageSendRetries {
		pathEnd.log.Error("Giving up on sending packet message after max retries",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", sequence),
			zap.Inline(k),
			zap.Int("max_retries", maxMessageSendRetries),
		)
		// giving up on sending this packet flow message
		// remove all retention of this connection handshake in pathEnd.messagesCache.PacketFlow and counterparty
		toDelete := make(map[string][]uint64)
		toDeleteCounterparty := make(map[string][]uint64)
		switch eventType {
		case chantypes.EventTypeRecvPacket:
			toDelete[eventType] = []uint64{sequence}
			toDeleteCounterparty[chantypes.EventTypeSendPacket] = []uint64{sequence}
		case chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket, chantypes.EventTypeTimeoutPacketOnClose:
			toDelete[eventType] = []uint64{sequence}
			toDeleteCounterparty[chantypes.EventTypeRecvPacket] = []uint64{sequence}
			toDelete[chantypes.EventTypeSendPacket] = []uint64{sequence}
		}
		// delete in progress send for this specific message
		pathEnd.packetProcessing[k].deleteMessages(map[string][]uint64{eventType: []uint64{sequence}})
		// delete all packet flow retention history for this sequence
		pathEnd.messageCache.PacketFlow[k].DeleteMessages(toDelete)
		counterparty.messageCache.PacketFlow[k].DeleteMessages(toDeleteCounterparty)
		return false
	}

	return true
}

// shouldSendConnectionMessage determines if the connection handshake message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendConnectionMessage(message connectionIBCMessage, counterparty *pathEndRuntime) bool {
	eventType := message.eventType
	k := connectionInfoConnectionKey(message.info).Counterparty()
	if message.info.Height >= counterparty.latestBlock.Height {
		pathEnd.log.Debug("Waiting to relay connection message until counterparty height has incremented",
			zap.Inline(k),
			zap.String("event_type", eventType),
		)
		return false
	}
	msgProcessCache, ok := pathEnd.connProcessing[eventType]
	if !ok {
		// in progress cache does not exist for this eventType, so can send.
		return true
	}
	inProgress, ok := msgProcessCache[k]
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
	if inProgress.retryCount >= maxMessageSendRetries {
		pathEnd.log.Error("Giving up on sending connection message after max retries",
			zap.String("event_type", eventType),
		)
		// giving up on sending this connection handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ConnectionKey)
		toDeleteCounterparty := make(map[string][]ConnectionKey)
		counterpartyKey := k.Counterparty()
		switch eventType {
		case conntypes.EventTypeConnectionOpenInit:
			toDeleteCounterparty[conntypes.EventTypeConnectionOpenInit] = []ConnectionKey{counterpartyKey.MsgInitKey()}
		case conntypes.EventTypeConnectionOpenAck:
			toDeleteCounterparty[conntypes.EventTypeConnectionOpenTry] = []ConnectionKey{counterpartyKey}
			toDelete[conntypes.EventTypeConnectionOpenInit] = []ConnectionKey{k.MsgInitKey()}
		case conntypes.EventTypeConnectionOpenConfirm:
			toDeleteCounterparty[conntypes.EventTypeConnectionOpenAck] = []ConnectionKey{counterpartyKey}
			toDelete[conntypes.EventTypeConnectionOpenTry] = []ConnectionKey{k}
			toDeleteCounterparty[conntypes.EventTypeConnectionOpenInit] = []ConnectionKey{counterpartyKey.MsgInitKey()}
		}
		// delete in progress send for this specific message
		pathEnd.connProcessing.deleteMessages(map[string][]ConnectionKey{eventType: []ConnectionKey{k}})

		// delete all connection handshake retention history for this connection
		pathEnd.messageCache.ConnectionHandshake.DeleteMessages(toDelete)
		counterparty.messageCache.ConnectionHandshake.DeleteMessages(toDeleteCounterparty)

		return false
	}

	return true
}

// shouldSendConnectionMessage determines if the channel handshake message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendChannelMessage(message channelIBCMessage, counterparty *pathEndRuntime) bool {
	eventType := message.eventType
	channelKey := channelInfoChannelKey(message.info).Counterparty()
	if message.info.Height >= counterparty.latestBlock.Height {
		pathEnd.log.Debug("Waiting to relay channel message until counterparty height has incremented",
			zap.Inline(channelKey),
			zap.String("event_type", eventType),
		)
		return false
	}
	msgProcessCache, ok := pathEnd.channelProcessing[eventType]
	if !ok {
		// in progress cache does not exist for this eventType, so can send.
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
	if inProgress.retryCount >= maxMessageSendRetries {
		pathEnd.log.Error("Giving up on sending channel message after max retries",
			zap.String("event_type", eventType),
			zap.Int("max_retries", maxMessageSendRetries),
		)
		// giving up on sending this channel handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ChannelKey)
		toDeleteCounterparty := make(map[string][]ChannelKey)
		toDeletePacket := make(map[string][]uint64)
		toDeleteCounterpartyPacket := make(map[string][]uint64)

		counterpartyKey := channelKey.Counterparty()
		switch eventType {
		case chantypes.EventTypeChannelOpenTry:
			toDeleteCounterparty[chantypes.EventTypeChannelOpenInit] = []ChannelKey{counterpartyKey.MsgInitKey()}
		case chantypes.EventTypeChannelOpenAck:
			toDeleteCounterparty[chantypes.EventTypeChannelOpenTry] = []ChannelKey{counterpartyKey}
			toDelete[chantypes.EventTypeChannelOpenInit] = []ChannelKey{channelKey.MsgInitKey()}
		case chantypes.EventTypeChannelOpenConfirm:
			toDeleteCounterparty[chantypes.EventTypeChannelOpenAck] = []ChannelKey{counterpartyKey}
			toDelete[chantypes.EventTypeChannelOpenTry] = []ChannelKey{channelKey}
			toDeleteCounterparty[chantypes.EventTypeChannelOpenInit] = []ChannelKey{counterpartyKey.MsgInitKey()}
		case chantypes.EventTypeChannelCloseConfirm:
			toDeleteCounterparty[chantypes.EventTypeChannelCloseInit] = []ChannelKey{counterpartyKey}
			toDelete[chantypes.EventTypeChannelCloseConfirm] = []ChannelKey{channelKey}

			// Gather relevant send packet messages, for this channel key, that should be deleted if we
			// are operating on an ordered channel.
			if messageCache, ok := pathEnd.messageCache.PacketFlow[channelKey]; ok {
				if seqCache, ok := messageCache[chantypes.EventTypeSendPacket]; ok {
					for seq, packetInfo := range seqCache {
						if packetInfo.ChannelOrder == chantypes.ORDERED.String() {
							toDeletePacket[chantypes.EventTypeSendPacket] = append(toDeletePacket[chantypes.EventTypeSendPacket], seq)
						}
					}
				}
			}

			// Gather relevant timeout messages, for this counterparty channel key, that should be deleted if we
			// are operating on an ordered channel.
			if messageCache, ok := counterparty.messageCache.PacketFlow[counterpartyKey]; ok {
				if seqCache, ok := messageCache[chantypes.EventTypeTimeoutPacket]; ok {
					for seq, packetInfo := range seqCache {
						if packetInfo.ChannelOrder == chantypes.ORDERED.String() {
							toDeleteCounterpartyPacket[chantypes.EventTypeTimeoutPacket] = append(toDeleteCounterpartyPacket[chantypes.EventTypeTimeoutPacket], seq)
						}
					}
				}
			}
		}

		// delete in progress send for this specific message
		pathEnd.channelProcessing.deleteMessages(map[string][]ChannelKey{eventType: {channelKey}})
		pathEnd.messageCache.PacketFlow[channelKey].DeleteMessages(toDeletePacket)
		counterparty.messageCache.PacketFlow[counterpartyKey].DeleteMessages(toDeleteCounterpartyPacket)

		// delete all connection handshake retention history for this channel
		pathEnd.messageCache.ChannelHandshake.DeleteMessages(toDelete)
		counterparty.messageCache.ChannelHandshake.DeleteMessages(toDeleteCounterparty)

		return false
	}

	return true
}

func (pathEnd *pathEndRuntime) trackProcessingPacketMessage(t packetMessageToTrack) {
	eventType := t.msg.eventType
	sequence := t.msg.info.Sequence
	channelKey, err := t.msg.channelKey()
	if err != nil {
		pathEnd.log.Error("Unexpected error tracking processing packet",
			zap.Inline(channelKey),
			zap.String("event_type", eventType),
			zap.Uint64("sequence", sequence),
			zap.Error(err),
		)
		return
	}
	msgProcessCache, ok := pathEnd.packetProcessing[channelKey]
	if !ok {
		msgProcessCache = make(packetChannelMessageCache)
		pathEnd.packetProcessing[channelKey] = msgProcessCache
	}
	channelProcessingCache, ok := msgProcessCache[eventType]
	if !ok {
		channelProcessingCache = make(packetMessageSendCache)
		msgProcessCache[eventType] = channelProcessingCache
	}

	retryCount := uint64(0)

	if inProgress, ok := channelProcessingCache[sequence]; ok {
		retryCount = inProgress.retryCount + 1
	}

	channelProcessingCache[sequence] = processingMessage{
		lastProcessedHeight: pathEnd.latestBlock.Height,
		retryCount:          retryCount,
		assembled:           t.assembled,
	}
}

func (pathEnd *pathEndRuntime) trackProcessingConnectionMessage(t connectionMessageToTrack) {
	eventType := t.msg.eventType
	connectionKey := connectionInfoConnectionKey(t.msg.info).Counterparty()
	msgProcessCache, ok := pathEnd.connProcessing[eventType]
	if !ok {
		msgProcessCache = make(connectionKeySendCache)
		pathEnd.connProcessing[eventType] = msgProcessCache
	}

	retryCount := uint64(0)

	if inProgress, ok := msgProcessCache[connectionKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgProcessCache[connectionKey] = processingMessage{
		lastProcessedHeight: pathEnd.latestBlock.Height,
		retryCount:          retryCount,
		assembled:           t.assembled,
	}
}

func (pathEnd *pathEndRuntime) trackProcessingChannelMessage(t channelMessageToTrack) {
	eventType := t.msg.eventType
	channelKey := channelInfoChannelKey(t.msg.info).Counterparty()
	msgProcessCache, ok := pathEnd.channelProcessing[eventType]
	if !ok {
		msgProcessCache = make(channelKeySendCache)
		pathEnd.channelProcessing[eventType] = msgProcessCache
	}

	retryCount := uint64(0)

	if inProgress, ok := msgProcessCache[channelKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgProcessCache[channelKey] = processingMessage{
		lastProcessedHeight: pathEnd.latestBlock.Height,
		retryCount:          retryCount,
		assembled:           t.assembled,
	}
}
