package processor

import (
	"context"

	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// DurationErrorRetry determines how long to wait before retrying
	// in the case of failure to send transactions with IBC messages.
	DurationErrorRetry = 5 * time.Second
)

// PathProcessor is a process that handles incoming IBC messages from a pair of chains.
// It determines what messages need to be relayed, and sends them.
type PathProcessor struct {
	log *zap.Logger

	pathEnd1 *PathEndRuntime
	pathEnd2 *PathEndRuntime

	// Signals to retry.
	retryProcess chan struct{}
}

// PathProcessors is a slice of PathProcessor instances
type PathProcessors []*PathProcessor

func (p PathProcessors) IsRelayedChannel(k ChannelKey, chainID string) bool {
	for _, pp := range p {
		if pp.IsRelayedChannel(chainID, k) {
			return true
		}
	}
	return false
}

// PathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which will
// be processed during Run(ctx).
type PathEndRuntime struct {
	info PathEnd

	chainProvider provider.ChainProvider

	// cached data
	messageCache         IBCMessagesCache
	connectionStateCache ConnectionStateCache
	channelStateCache    ChannelStateCache

	// New messages and other data arriving from the handleNewMessagesForPathEnd method.
	incomingCacheData chan ChainProcessorCacheData

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
}

func (pathEnd *PathEndRuntime) MergeCacheData(d ChainProcessorCacheData) {
	// TODO make sure passes channel filter for pathEnd1 before calling this
	pathEnd.messageCache.Merge(d.IBCMessagesCache)             // Merge incoming packet IBC messages into the backlog
	pathEnd.connectionStateCache.Merge(d.ConnectionStateCache) // Update latest connection open state for chain
	pathEnd.channelStateCache.Merge(d.ChannelStateCache)       // Update latest channel open state for chain
	pathEnd.inSync = d.InSync
}

// IBCMessageWithSequence holds a packet's sequence along with it,
// useful for sending packets around internal to the PathProcessor.
type IBCMessageWithSequence struct {
	Sequence uint64
	Message  provider.RelayerMessage
}

// IBCMessageWithChannel holds a channel handshake message's channel along with it,
// useful for sending messages around internal to the PathProcessor.
type IBCMessageWithChannel struct {
	ChannelKey
	Message provider.RelayerMessage
}

// IBCMessageWithConnection holds a connection handshake message's connection along with it,
// useful for sending messages around internal to the PathProcessor.
type IBCMessageWithConnection struct {
	ConnectionKey
	Message provider.RelayerMessage
}

func NewPathProcessor(log *zap.Logger, pathEnd1 PathEnd, pathEnd2 PathEnd) *PathProcessor {
	return &PathProcessor{
		log: log,
		pathEnd1: &PathEndRuntime{
			info:                 pathEnd1,
			incomingCacheData:    make(chan ChainProcessorCacheData, 100),
			connectionStateCache: make(ConnectionStateCache),
			channelStateCache:    make(ChannelStateCache),
			messageCache:         NewIBCMessagesCache(),
		},
		pathEnd2: &PathEndRuntime{
			info:                 pathEnd2,
			incomingCacheData:    make(chan ChainProcessorCacheData, 100),
			connectionStateCache: make(ConnectionStateCache),
			channelStateCache:    make(ChannelStateCache),
			messageCache:         NewIBCMessagesCache(),
		},
		retryProcess: make(chan struct{}, 8),
	}
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd1Messages(channelKey ChannelKey, message string) PacketSequenceCache {
	return pp.pathEnd1.messageCache.PacketFlow[channelKey][message]
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd2Messages(channelKey ChannelKey, message string) PacketSequenceCache {
	return pp.pathEnd2.messageCache.PacketFlow[channelKey][message]
}

type channelPair struct {
	pathEnd1ChannelKey ChannelKey
	pathEnd2ChannelKey ChannelKey
}

type connectionPair struct {
	pathEnd1ConnectionKey ConnectionKey
	pathEnd2ConnectionKey ConnectionKey
}

func (pp *PathProcessor) channelPairs() []channelPair {
	// Channel keys are from pathEnd1's perspective
	channels := make(map[ChannelKey]bool)
	for k, open := range pp.pathEnd1.channelStateCache {
		channels[k] = open
	}
	for k, open := range pp.pathEnd2.channelStateCache {
		channels[k.Counterparty()] = open
	}
	pairs := make([]channelPair, len(channels))
	i := 0
	for k, open := range channels {
		if !open {
			continue
		}
		pairs[i] = channelPair{
			pathEnd1ChannelKey: k,
			pathEnd2ChannelKey: k.Counterparty(),
		}
		i++
	}
	return pairs
}

// Path Processors are constructed before ChainProcessors, so reference needs to be added afterwards
// This can be done inside the ChainProcessor constructor for simplification
func (pp *PathProcessor) SetChainProviderIfApplicable(chainProvider provider.ChainProvider) bool {
	if chainProvider == nil {
		return false
	}
	if pp.pathEnd1.info.ChainID == chainProvider.ChainId() {
		pp.pathEnd1.chainProvider = chainProvider
		return true
	} else if pp.pathEnd2.info.ChainID == chainProvider.ChainId() {
		pp.pathEnd2.chainProvider = chainProvider
		return true
	}
	return false
}

func (pp *PathProcessor) IsRelayedChannel(chainID string, channelKey ChannelKey) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ShouldRelayChannel(channelKey)
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ShouldRelayChannel(channelKey)
	}
	return false
}

func (pp *PathProcessor) IsRelevantClient(chainID string, clientID string) bool {
	if pp.pathEnd1.info.ChainID == chainID && pp.pathEnd1.info.ClientID == clientID {
		return true
	} else if pp.pathEnd2.info.ChainID == chainID && pp.pathEnd2.info.ClientID == clientID {
		return true
	}
	return false
}

func (pp *PathProcessor) IsRelevantConnection(chainID string, connectionID string) bool {
	if pp.pathEnd1.info.ChainID == chainID && pp.pathEnd1.info.ConnectionID == connectionID {
		return true
	} else if pp.pathEnd2.info.ChainID == chainID && pp.pathEnd2.info.ConnectionID == connectionID {
		return true
	}
	return false
}

// ProcessBacklogIfReady gives ChainProcessors a way to trigger the path processor process
// as soon as they are in sync for the first time, even if they do not have new messages.
func (pp *PathProcessor) ProcessBacklogIfReady() {
	select {
	case pp.retryProcess <- struct{}{}:
		// All good.
	default:
		// Log that the channel is saturated;
		// something is wrong if we are retrying this quickly.
		pp.log.Info("Failed to enqueue path processor retry")
	}
}

// ChainProcessors call this method when they have new IBC messages
func (pp *PathProcessor) HandleNewData(chainID string, cacheData ChainProcessorCacheData) {
	if pp.pathEnd1.info.ChainID == chainID {
		pp.pathEnd1.incomingCacheData <- cacheData
	} else if pp.pathEnd2.info.ChainID == chainID {
		pp.pathEnd2.incomingCacheData <- cacheData
	}
}

// contains MsgRecvPacket from counterparty
// entire packet flow
type PathEndPacketFlowMessages struct {
	SrcMsgTransfer        PacketSequenceCache
	DstMsgRecvPacket      PacketSequenceCache
	SrcMsgAcknowledgement PacketSequenceCache
	SrcMsgTimeout         PacketSequenceCache
	SrcMsgTimeoutOnClose  PacketSequenceCache
}

type PathEndConnectionHandshakeMessages struct {
	SrcMsgConnectionOpenInit    ConnectionMessageCache
	DstMsgConnectionOpenTry     ConnectionMessageCache
	SrcMsgConnectionOpenAck     ConnectionMessageCache
	DstMsgConnectionOpenConfirm ConnectionMessageCache
}

type PathEndChannelHandshakeMessages struct {
	SrcMsgChannelOpenInit    ChannelMessageCache
	DstMsgChannelOpenTry     ChannelMessageCache
	SrcMsgChannelOpenAck     ChannelMessageCache
	DstMsgChannelOpenConfirm ChannelMessageCache
}

type PathEndChannelCloseMessages struct {
	SrcMsgChannelCloseInit    ChannelMessageCache
	DstMsgChannelCloseConfirm ChannelMessageCache
}

type PathEndProcessedResponse struct {
	UnrelayedPackets          []IBCMessageWithSequence
	UnrelayedAcknowledgements []IBCMessageWithSequence

	ToDeleteSrc map[string][]uint64
	ToDeleteDst map[string][]uint64
}

type PathEndChannelHandshakeResponse struct {
	SrcMessages []IBCMessageWithChannel
	DstMessages []IBCMessageWithChannel

	ToDeleteSrc map[string][]ChannelKey
	ToDeleteDst map[string][]ChannelKey
}

type PathEndConnectionHandshakeResponse struct {
	SrcMessages []IBCMessageWithConnection
	DstMessages []IBCMessageWithConnection

	ToDeleteSrc map[string][]ConnectionKey
	ToDeleteDst map[string][]ConnectionKey
}

func (m *PathEndProcessedResponse) appendPacket(sequence uint64, msgRecvPacket provider.RelayerMessage) {
	m.UnrelayedPackets = append(m.UnrelayedPackets, IBCMessageWithSequence{Sequence: sequence, Message: msgRecvPacket})
}

func (m *PathEndProcessedResponse) appendAcknowledgement(sequence uint64, msgAcknowledgement provider.RelayerMessage) {
	m.UnrelayedAcknowledgements = append(m.UnrelayedAcknowledgements, IBCMessageWithSequence{Sequence: sequence, Message: msgAcknowledgement})
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(pathEndPacketFlowMessages PathEndPacketFlowMessages, wg *sync.WaitGroup, res *PathEndProcessedResponse) {
	defer wg.Done()
	res.UnrelayedPackets = nil
	res.UnrelayedAcknowledgements = nil
	res.ToDeleteSrc = make(map[string][]uint64)
	res.ToDeleteDst = make(map[string][]uint64)

MsgTransferLoop:
	for transferSeq, msgTransfer := range pathEndPacketFlowMessages.SrcMsgTransfer {
		for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
			if transferSeq == ackSeq {
				// we have an ack for this packet, so packet flow is complete
				// remove all retention of this sequence number
				res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], transferSeq)
				res.ToDeleteDst[MsgRecvPacket] = append(res.ToDeleteDst[MsgRecvPacket], transferSeq)
				res.ToDeleteSrc[MsgAcknowledgement] = append(res.ToDeleteSrc[MsgAcknowledgement], transferSeq)
				continue MsgTransferLoop
			}
		}
		for timeoutSeq := range pathEndPacketFlowMessages.SrcMsgTimeout {
			if transferSeq == timeoutSeq {
				// we have a timeout for this packet, so packet flow is complete
				// remove all retention of this sequence number
				res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], transferSeq)
				res.ToDeleteSrc[MsgTimeout] = append(res.ToDeleteSrc[MsgTimeout], transferSeq)
				continue MsgTransferLoop
			}
		}
		for timeoutOnCloseSeq := range pathEndPacketFlowMessages.SrcMsgTimeoutOnClose {
			if transferSeq == timeoutOnCloseSeq {
				// we have a timeout for this packet, so packet flow is complete
				// remove all retention of this sequence number
				res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], transferSeq)
				res.ToDeleteSrc[MsgTimeoutOnClose] = append(res.ToDeleteSrc[MsgTimeoutOnClose], transferSeq)
				continue MsgTransferLoop
			}
		}
		for msgRecvSeq, msgAcknowledgement := range pathEndPacketFlowMessages.DstMsgRecvPacket {
			if transferSeq == msgRecvSeq {
				// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
				res.appendAcknowledgement(msgRecvSeq, msgAcknowledgement)
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay from src to dst
		res.appendPacket(transferSeq, msgTransfer)
	}

	// now iterate through packet-flow-complete messages and remove any leftover messages if the MsgTransfer or MsgRecvPacket was in a previous block that we did not query
	for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
		res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], ackSeq)
		res.ToDeleteDst[MsgRecvPacket] = append(res.ToDeleteDst[MsgRecvPacket], ackSeq)
		res.ToDeleteSrc[MsgAcknowledgement] = append(res.ToDeleteSrc[MsgAcknowledgement], ackSeq)
	}
	for timeoutSeq := range pathEndPacketFlowMessages.SrcMsgTimeout {
		res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], timeoutSeq)
		res.ToDeleteSrc[MsgTimeout] = append(res.ToDeleteSrc[MsgTimeout], timeoutSeq)
	}
	for timeoutOnCloseSeq := range pathEndPacketFlowMessages.SrcMsgTimeoutOnClose {
		res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], timeoutOnCloseSeq)
		res.ToDeleteSrc[MsgTimeoutOnClose] = append(res.ToDeleteSrc[MsgTimeoutOnClose], timeoutOnCloseSeq)
	}
}

func (pp *PathProcessor) getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEndConnectionHandshakeMessages PathEndConnectionHandshakeMessages, wg *sync.WaitGroup, res *PathEndConnectionHandshakeResponse) {
	defer wg.Done()
	res.SrcMessages = nil
	res.DstMessages = nil
	res.ToDeleteSrc = make(map[string][]ConnectionKey)
	res.ToDeleteDst = make(map[string][]ConnectionKey)

ConnectionHandshakeLoop:
	for openInitKey, openInitMsg := range pathEndConnectionHandshakeMessages.SrcMsgConnectionOpenInit {
		var foundOpenTry provider.RelayerMessage
		for openTryKey, openTryMsg := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenTry {
			if openInitKey == openTryKey {
				foundOpenTry = openTryMsg
				break
			}
		}
		if foundOpenTry == nil {
			// need to send an open try to dst
			res.DstMessages = append(res.DstMessages, IBCMessageWithConnection{
				ConnectionKey: openInitKey,
				Message:       openInitMsg,
			})
			continue ConnectionHandshakeLoop
		}
		var foundOpenAck provider.RelayerMessage
		for openAckKey, openAckMsg := range pathEndConnectionHandshakeMessages.SrcMsgConnectionOpenAck {
			if openInitKey == openAckKey {
				foundOpenAck = openAckMsg
				break
			}
		}
		if foundOpenAck == nil {
			// need to send an open ack to src
			res.SrcMessages = append(res.SrcMessages, IBCMessageWithConnection{
				ConnectionKey: openInitKey,
				Message:       foundOpenTry,
			})
			continue ConnectionHandshakeLoop
		}
		var foundOpenConfirm provider.RelayerMessage
		for openConfirmKey, openConfirmMsg := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenConfirm {
			if openInitKey == openConfirmKey {
				foundOpenConfirm = openConfirmMsg
				break
			}
		}
		if foundOpenConfirm == nil {
			// need to send an open confirm to dst
			res.DstMessages = append(res.DstMessages, IBCMessageWithConnection{
				ConnectionKey: openInitKey,
				Message:       foundOpenAck,
			})
			continue ConnectionHandshakeLoop
		}
		// handshake is complete for this connection, remove all retention.
		res.ToDeleteSrc[MsgConnectionOpenInit] = append(res.ToDeleteSrc[MsgConnectionOpenInit], openInitKey)
		res.ToDeleteDst[MsgConnectionOpenTry] = append(res.ToDeleteDst[MsgConnectionOpenTry], openInitKey)
		res.ToDeleteSrc[MsgConnectionOpenAck] = append(res.ToDeleteSrc[MsgConnectionOpenAck], openInitKey)
		res.ToDeleteDst[MsgConnectionOpenConfirm] = append(res.ToDeleteDst[MsgConnectionOpenConfirm], openInitKey)
	}

	// now iterate through connection-handshake-complete messages and remove any leftover messages
	for openConfirmKey := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenConfirm {
		res.ToDeleteSrc[MsgConnectionOpenInit] = append(res.ToDeleteSrc[MsgConnectionOpenInit], openConfirmKey)
		res.ToDeleteDst[MsgConnectionOpenTry] = append(res.ToDeleteDst[MsgConnectionOpenTry], openConfirmKey)
		res.ToDeleteSrc[MsgConnectionOpenAck] = append(res.ToDeleteSrc[MsgConnectionOpenAck], openConfirmKey)
		res.ToDeleteDst[MsgConnectionOpenConfirm] = append(res.ToDeleteDst[MsgConnectionOpenConfirm], openConfirmKey)
	}
}

func (pp *PathProcessor) getUnrelayedChannelHandshakeMessagesAndToDelete(pathEndChannelHandshakeMessages PathEndChannelHandshakeMessages, wg *sync.WaitGroup, res *PathEndChannelHandshakeResponse) {
	defer wg.Done()
	res.SrcMessages = nil
	res.DstMessages = nil
	res.ToDeleteSrc = make(map[string][]ChannelKey)
	res.ToDeleteDst = make(map[string][]ChannelKey)

ChannelHandshakeLoop:
	for openInitKey, openInitMsg := range pathEndChannelHandshakeMessages.SrcMsgChannelOpenInit {
		var foundOpenTry provider.RelayerMessage
		for openTryKey, openTryMsg := range pathEndChannelHandshakeMessages.DstMsgChannelOpenTry {
			if openInitKey == openTryKey {
				foundOpenTry = openTryMsg
				break
			}
		}
		if foundOpenTry == nil {
			// need to send an open try to dst
			res.DstMessages = append(res.DstMessages, IBCMessageWithChannel{
				ChannelKey: openInitKey,
				Message:    openInitMsg,
			})
			continue ChannelHandshakeLoop
		}
		var foundOpenAck provider.RelayerMessage
		for openAckKey, openAckMsg := range pathEndChannelHandshakeMessages.SrcMsgChannelOpenAck {
			if openInitKey == openAckKey {
				foundOpenAck = openAckMsg
				break
			}
		}
		if foundOpenAck == nil {
			// need to send an open ack to src
			res.SrcMessages = append(res.SrcMessages, IBCMessageWithChannel{
				ChannelKey: openInitKey,
				Message:    foundOpenTry,
			})
			continue ChannelHandshakeLoop
		}
		var foundOpenConfirm provider.RelayerMessage
		for openConfirmKey, openConfirmMsg := range pathEndChannelHandshakeMessages.DstMsgChannelOpenConfirm {
			if openInitKey == openConfirmKey {
				foundOpenConfirm = openConfirmMsg
				break
			}
		}
		if foundOpenConfirm == nil {
			// need to send an open confirm to dst
			res.DstMessages = append(res.DstMessages, IBCMessageWithChannel{
				ChannelKey: openInitKey,
				Message:    foundOpenAck,
			})
			continue ChannelHandshakeLoop
		}
		// handshake is complete for this channel, remove all retention.
		res.ToDeleteSrc[MsgChannelOpenInit] = append(res.ToDeleteSrc[MsgChannelOpenInit], openInitKey)
		res.ToDeleteDst[MsgChannelOpenTry] = append(res.ToDeleteDst[MsgChannelOpenTry], openInitKey)
		res.ToDeleteSrc[MsgChannelOpenAck] = append(res.ToDeleteSrc[MsgChannelOpenAck], openInitKey)
		res.ToDeleteDst[MsgChannelOpenConfirm] = append(res.ToDeleteDst[MsgChannelOpenConfirm], openInitKey)
	}

	// now iterate through channel-handshake-complete messages and remove any leftover messages
	for openConfirmKey := range pathEndChannelHandshakeMessages.DstMsgChannelOpenConfirm {
		res.ToDeleteSrc[MsgChannelOpenInit] = append(res.ToDeleteSrc[MsgChannelOpenInit], openConfirmKey)
		res.ToDeleteDst[MsgChannelOpenTry] = append(res.ToDeleteDst[MsgChannelOpenTry], openConfirmKey)
		res.ToDeleteSrc[MsgChannelOpenAck] = append(res.ToDeleteSrc[MsgChannelOpenAck], openConfirmKey)
		res.ToDeleteDst[MsgChannelOpenConfirm] = append(res.ToDeleteDst[MsgChannelOpenConfirm], openConfirmKey)
	}
}

func (pp *PathProcessor) sendMessages(pathEnd *PathEndRuntime, packetMessages []IBCMessageWithSequence, connectionMessages []IBCMessageWithConnection, channelMessages []IBCMessageWithChannel) error {
	if len(packetMessages) == 0 && len(connectionMessages) == 0 && len(channelMessages) == 0 {
		return nil
	}
	// TODO construct MsgUpdateClient for this pathEnd, using the latest trusted IBC header from other pathEnd, prepend messages with the MsgUpdateClient, then send the messages to this pathEnd

	pp.log.Debug("will send", zap.Any("packet_messages", packetMessages), zap.Any("connection_messages", connectionMessages), zap.Any("channel_messages", channelMessages))

	return nil
}

// messages from both pathEnds are needed in order to determine what needs to be relayed for a single pathEnd
func (pp *PathProcessor) processLatestMessages() error {
	channelPairs := pp.channelPairs()

	// process the packet flows for both packends to determine what needs to be relayed
	pathEnd1ProcessRes := make([]*PathEndProcessedResponse, len(channelPairs))
	pathEnd2ProcessRes := make([]*PathEndProcessedResponse, len(channelPairs))

	var pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes PathEndConnectionHandshakeResponse
	var pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes PathEndChannelHandshakeResponse

	var wg sync.WaitGroup

	pathEnd1ConnectionHandshakeMessages := PathEndConnectionHandshakeMessages{
		SrcMsgConnectionOpenInit:    pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenConfirm],
	}
	pathEnd2ConnectionHandshakeMessages := PathEndConnectionHandshakeMessages{
		SrcMsgConnectionOpenInit:    pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenConfirm],
	}
	wg.Add(2)
	go pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd1ConnectionHandshakeMessages, &wg, &pathEnd1ConnectionHandshakeRes)
	go pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd2ConnectionHandshakeMessages, &wg, &pathEnd2ConnectionHandshakeRes)

	pathEnd1ChannelHandshakeMessages := PathEndChannelHandshakeMessages{
		SrcMsgChannelOpenInit:    pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenConfirm],
	}
	pathEnd2ChannelHandshakeMessages := PathEndChannelHandshakeMessages{
		SrcMsgChannelOpenInit:    pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenConfirm],
	}
	wg.Add(2)
	go pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd1ChannelHandshakeMessages, &wg, &pathEnd1ChannelHandshakeRes)
	go pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd2ChannelHandshakeMessages, &wg, &pathEnd2ChannelHandshakeRes)

	for i, pair := range channelPairs {
		pathEnd1PacketFlowMessages := PathEndPacketFlowMessages{
			SrcMsgTransfer:        pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgAcknowledgement],
			SrcMsgTimeout:         pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTimeout],
			SrcMsgTimeoutOnClose:  pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTimeoutOnClose],
		}
		pathEnd2PacketFlowMessages := PathEndPacketFlowMessages{
			SrcMsgTransfer:        pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgAcknowledgement],
			SrcMsgTimeout:         pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTimeout],
			SrcMsgTimeoutOnClose:  pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTimeoutOnClose],
		}

		pathEnd1ProcessRes[i] = new(PathEndProcessedResponse)
		pathEnd2ProcessRes[i] = new(PathEndProcessedResponse)

		wg.Add(2)
		go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd1PacketFlowMessages, &wg, pathEnd1ProcessRes[i])
		go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd2PacketFlowMessages, &wg, pathEnd2ProcessRes[i])
	}
	wg.Wait()

	// concatenate applicable messages for pathend
	var pathEnd1PacketMessages, pathEnd2PacketMessages []IBCMessageWithSequence
	var pathEnd1ConnectionMessages, pathEnd2ConnectionMessages []IBCMessageWithConnection
	var pathEnd1ChannelMessages, pathEnd2ChannelMessages []IBCMessageWithChannel

	pathEnd1ConnectionMessages = append(pathEnd1ConnectionMessages, pathEnd1ConnectionHandshakeRes.SrcMessages...)
	pathEnd1ConnectionMessages = append(pathEnd1ConnectionMessages, pathEnd2ConnectionHandshakeRes.DstMessages...)

	pathEnd2ConnectionMessages = append(pathEnd2ConnectionMessages, pathEnd2ConnectionHandshakeRes.SrcMessages...)
	pathEnd2ConnectionMessages = append(pathEnd2ConnectionMessages, pathEnd1ConnectionHandshakeRes.DstMessages...)

	pp.pathEnd1.messageCache.ConnectionHandshake.DeleteCachedMessages(pathEnd1ConnectionHandshakeRes.ToDeleteSrc, pathEnd2ConnectionHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ConnectionHandshake.DeleteCachedMessages(pathEnd2ConnectionHandshakeRes.ToDeleteSrc, pathEnd1ConnectionHandshakeRes.ToDeleteDst)

	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd1ChannelHandshakeRes.SrcMessages...)
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd2ChannelHandshakeRes.DstMessages...)

	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd2ChannelHandshakeRes.SrcMessages...)
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd1ChannelHandshakeRes.DstMessages...)

	pp.pathEnd1.messageCache.ChannelHandshake.DeleteCachedMessages(pathEnd1ChannelHandshakeRes.ToDeleteSrc, pathEnd2ChannelHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ChannelHandshake.DeleteCachedMessages(pathEnd2ChannelHandshakeRes.ToDeleteSrc, pathEnd1ChannelHandshakeRes.ToDeleteDst)

	for i := 0; i < len(channelPairs); i++ {
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd2ProcessRes[i].UnrelayedPackets...)
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd1ProcessRes[i].UnrelayedAcknowledgements...)

		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd1ProcessRes[i].UnrelayedPackets...)
		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd2ProcessRes[i].UnrelayedAcknowledgements...)

		pp.pathEnd1.messageCache.PacketFlow[channelPairs[i].pathEnd1ChannelKey].DeleteCachedMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.messageCache.PacketFlow[channelPairs[i].pathEnd2ChannelKey].DeleteCachedMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)
	}

	// now send messages in parallel
	var eg errgroup.Group
	eg.Go(func() error {
		return pp.sendMessages(pp.pathEnd1, pathEnd1PacketMessages, pathEnd1ConnectionMessages, pathEnd1ChannelMessages)
	})
	eg.Go(func() error {
		return pp.sendMessages(pp.pathEnd2, pathEnd2PacketMessages, pathEnd2ConnectionMessages, pathEnd2ChannelMessages)
	})
	return eg.Wait()
}

// Run executes the main path process.
func (pp *PathProcessor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case d := <-pp.pathEnd1.incomingCacheData:
			// we have new data from ChainProcessor for pathEnd1
			pp.pathEnd1.MergeCacheData(d)

		case d := <-pp.pathEnd2.incomingCacheData:
			// we have new data from ChainProcessor for pathEnd2
			pp.pathEnd2.MergeCacheData(d)

		case <-pp.retryProcess:
			// No new data to merge in, just retry handling.
		}

		if !pp.pathEnd1.inSync || !pp.pathEnd2.inSync {
			continue
		}

		// process latest message cache state from both pathEnds
		if err := pp.processLatestMessages(); err != nil {
			// in case of IBC message send errors, schedule retry after DurationErrorRetry
			time.AfterFunc(DurationErrorRetry, pp.ProcessBacklogIfReady)
		}
	}
}
