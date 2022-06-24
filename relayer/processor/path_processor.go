package processor

import (
	"context"
	"errors"
	"fmt"

	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// durationErrorRetry determines how long to wait before retrying
	// in the case of failure to send transactions with IBC messages.
	durationErrorRetry = 5 * time.Second

	ibcHeadersToCache = 10
)

// PathProcessor is a process that handles incoming IBC messages from a pair of chains.
// It determines what messages need to be relayed, and sends them.
type PathProcessor struct {
	log *zap.Logger

	pathEnd1 *pathEndRuntime
	pathEnd2 *pathEndRuntime

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

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
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

func NewPathProcessor(log *zap.Logger, pathEnd1 PathEnd, pathEnd2 PathEnd) *PathProcessor {
	return &PathProcessor{
		log: log,
		pathEnd1: &pathEndRuntime{
			info:                 pathEnd1,
			incomingCacheData:    make(chan ChainProcessorCacheData, 100),
			connectionStateCache: make(ConnectionStateCache),
			channelStateCache:    make(ChannelStateCache),
			messageCache:         NewIBCMessagesCache(),
			ibcHeaderCache:       make(IBCHeaderCache),
		},
		pathEnd2: &pathEndRuntime{
			info:                 pathEnd2,
			incomingCacheData:    make(chan ChainProcessorCacheData, 100),
			connectionStateCache: make(ConnectionStateCache),
			channelStateCache:    make(ChannelStateCache),
			messageCache:         NewIBCMessagesCache(),
			ibcHeaderCache:       make(IBCHeaderCache),
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

// RelevantClientID returns the relevant client ID or panics
func (pp *PathProcessor) RelevantClientID(chainID string) string {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ClientID
	}
	if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ClientID
	}
	panic(fmt.Errorf("no relevant client ID for chain ID: %s", chainID))
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
type pathEndPacketFlowMessages struct {
	Src                   *pathEndRuntime
	Dst                   *pathEndRuntime
	SrcMsgTransfer        PacketSequenceCache
	DstMsgRecvPacket      PacketSequenceCache
	SrcMsgAcknowledgement PacketSequenceCache
	SrcMsgTimeout         PacketSequenceCache
	SrcMsgTimeoutOnClose  PacketSequenceCache
}

type pathEndConnectionHandshakeMessages struct {
	Src                         *pathEndRuntime
	Dst                         *pathEndRuntime
	SrcMsgConnectionOpenInit    ConnectionMessageCache
	DstMsgConnectionOpenTry     ConnectionMessageCache
	SrcMsgConnectionOpenAck     ConnectionMessageCache
	DstMsgConnectionOpenConfirm ConnectionMessageCache
}

type pathEndChannelHandshakeMessages struct {
	Src                      *pathEndRuntime
	Dst                      *pathEndRuntime
	SrcMsgChannelOpenInit    ChannelMessageCache
	DstMsgChannelOpenTry     ChannelMessageCache
	SrcMsgChannelOpenAck     ChannelMessageCache
	DstMsgChannelOpenConfirm ChannelMessageCache
}

type pathEndChannelCloseMessages struct {
	Src                       *pathEndRuntime
	Dst                       *pathEndRuntime
	SrcMsgChannelCloseInit    ChannelMessageCache
	DstMsgChannelCloseConfirm ChannelMessageCache
}

type pathEndPacketFlowResponse struct {
	SrcMessages []provider.RelayerMessage
	DstMessages []provider.RelayerMessage

	ToDeleteSrc map[string][]uint64
	ToDeleteDst map[string][]uint64
}

type pathEndChannelHandshakeResponse struct {
	SrcMessages []provider.RelayerMessage
	DstMessages []provider.RelayerMessage

	ToDeleteSrc map[string][]ChannelKey
	ToDeleteDst map[string][]ChannelKey
}

type pathEndConnectionHandshakeResponse struct {
	SrcMessages []provider.RelayerMessage
	DstMessages []provider.RelayerMessage

	ToDeleteSrc map[string][]ConnectionKey
	ToDeleteDst map[string][]ConnectionKey
}

// assembleIBCMessage constructs the applicable IBC message using the requested function.
// These functions may do things like make queries in order to assemble a complete IBC message.
func (pp *PathProcessor) assembleIBCMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	message string,
	partialMessage provider.RelayerMessage,
	assembleMessage func(ctx context.Context, msgRecvPacket provider.RelayerMessage, signer string, latest provider.LatestBlock) (provider.RelayerMessage, error),
	messages *[]provider.RelayerMessage,
) error {
	signer, err := dst.chainProvider.Address()
	if err != nil {
		return fmt.Errorf("error getting signer address for {%s}: %w", dst.info.ChainID, err)
	}
	assembled, err := assembleMessage(ctx, partialMessage, signer, src.latestBlock)
	if err != nil {
		return fmt.Errorf("error assembling %s for {%s}: %w", message, dst.info.ChainID, err)
	}
	*messages = append(*messages, assembled)
	return nil
}

func (pp *PathProcessor) appendPacketOrTimeout(ctx context.Context, src, dst *pathEndRuntime, sequence uint64, msgRecvPacket provider.RelayerMessage, res *pathEndPacketFlowResponse) {
	if err := dst.chainProvider.ValidatePacket(msgRecvPacket, dst.latestBlock); err != nil {
		var timeoutHeightErr *provider.TimeoutHeightError
		var timeoutTimestampErr *provider.TimeoutTimestampError
		var timeoutOnCloseErr *provider.TimeoutOnCloseError

		// if timeouts were detected, need to generate msgs for them for src
		switch {
		case errors.As(err, &timeoutHeightErr) || errors.As(err, &timeoutTimestampErr):
			if err := pp.assembleIBCMessage(ctx, dst, src, MsgTimeout, msgRecvPacket, dst.chainProvider.MsgTimeout, &res.SrcMessages); err != nil {
				pp.log.Error("Error assembling MsgTimeout",
					zap.Uint64("sequence", sequence),
					zap.String("chain_id", src.info.ChainID),
					zap.Error(err),
				)
			}
		case errors.As(err, &timeoutOnCloseErr):
			if err := pp.assembleIBCMessage(ctx, dst, src, MsgTimeoutOnClose, msgRecvPacket, dst.chainProvider.MsgTimeoutOnClose, &res.SrcMessages); err != nil {
				pp.log.Error("Error assembling MsgTimeoutOnClose",
					zap.Uint64("sequence", sequence),
					zap.String("chain_id", src.info.ChainID),
					zap.Error(err),
				)
			}
		default:
			pp.log.Error("Packet is invalid",
				zap.String("chain_id", src.info.ChainID),
				zap.Error(err),
			)
		}
		return
	}
	if err := pp.assembleIBCMessage(ctx, src, dst, MsgRecvPacket, msgRecvPacket, src.chainProvider.MsgRecvPacket, &res.DstMessages); err != nil {
		pp.log.Error("Error assembling MsgRecvPacket",
			zap.Uint64("sequence", sequence),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.Error(err),
		)
	}
}

func (pp *PathProcessor) appendAcknowledgement(ctx context.Context, src, dst *pathEndRuntime, sequence uint64, msgAcknowledgement provider.RelayerMessage, res *pathEndPacketFlowResponse) {
	if err := pp.assembleIBCMessage(ctx, src, dst, MsgAcknowledgement, msgAcknowledgement, src.chainProvider.MsgAcknowledgement, &res.DstMessages); err != nil {
		pp.log.Error("Error assembling MsgAcknowledgement",
			zap.Uint64("sequence", sequence),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.Error(err),
		)
	}
}

func (m *pathEndConnectionHandshakeResponse) appendSrc(connectionKey ConnectionKey, msg provider.RelayerMessage) {
	m.SrcMessages = append(m.SrcMessages, msg)
}

func (m *pathEndConnectionHandshakeResponse) appendDst(connectionKey ConnectionKey, msg provider.RelayerMessage) {
	m.DstMessages = append(m.DstMessages, msg)
}

func (m *pathEndChannelHandshakeResponse) appendSrc(channelKey ChannelKey, msg provider.RelayerMessage) {
	m.SrcMessages = append(m.SrcMessages, msg)
}

func (m *pathEndChannelHandshakeResponse) appendDst(channelKey ChannelKey, msg provider.RelayerMessage) {
	m.DstMessages = append(m.DstMessages, msg)
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(ctx context.Context, pathEndPacketFlowMessages pathEndPacketFlowMessages, wg *sync.WaitGroup, res *pathEndPacketFlowResponse) {
	defer wg.Done()
	res.SrcMessages = nil
	res.DstMessages = nil
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
				pp.appendAcknowledgement(ctx, pathEndPacketFlowMessages.Dst, pathEndPacketFlowMessages.Src, msgRecvSeq, msgAcknowledgement, res)
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay from src to dst
		pp.appendPacketOrTimeout(ctx, pathEndPacketFlowMessages.Src, pathEndPacketFlowMessages.Dst, transferSeq, msgTransfer, res)
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

func (pp *PathProcessor) getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEndConnectionHandshakeMessages pathEndConnectionHandshakeMessages, wg *sync.WaitGroup, res *pathEndConnectionHandshakeResponse) {
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
			res.appendDst(openInitKey, openInitMsg)
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
			res.appendSrc(openInitKey, foundOpenTry)
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
			res.appendDst(openInitKey, foundOpenAck)
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

func (pp *PathProcessor) getUnrelayedChannelHandshakeMessagesAndToDelete(pathEndChannelHandshakeMessages pathEndChannelHandshakeMessages, wg *sync.WaitGroup, res *pathEndChannelHandshakeResponse) {
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
			res.appendDst(openInitKey, openInitMsg)
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
			res.appendSrc(openInitKey, foundOpenTry)
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
			res.appendDst(openInitKey, foundOpenAck)
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

func (pp *PathProcessor) prependMsgUpdateClient(src, dst *pathEndRuntime, messages *[]provider.RelayerMessage) error {
	// If the client state trusted height is not equal to the client trusted state height, we cannot send a MsgUpdateClient
	// until another block is observed. But we can send the rest without the MsgUpdateClient.
	if !dst.clientTrustedState.ClientState.ConsensusHeight.EQ(dst.clientState.ConsensusHeight) {
		return fmt.Errorf("observed client trusted height: %d does not equal latest client state height: %d",
			dst.clientTrustedState.ClientState.ConsensusHeight.RevisionHeight, dst.clientState.ConsensusHeight.RevisionHeight)
	}

	msgUpdateClientHeader, err := src.chainProvider.MsgUpdateClientHeader(src.latestHeader, dst.clientTrustedState.ClientState.ConsensusHeight, dst.clientTrustedState.IBCHeader)
	if err != nil {
		return fmt.Errorf("error assembling new client header: %w", err)
	}

	msgUpdateClient, err := dst.chainProvider.MsgUpdateClient(dst.info.ClientID, msgUpdateClientHeader)
	if err != nil {
		return fmt.Errorf("error assembling MsgUpdateClient: %w", err)
	}

	*messages = append([]provider.RelayerMessage{msgUpdateClient}, *messages...)

	return nil
}

func (pp *PathProcessor) sendMessages(src, dst *pathEndRuntime, messages []provider.RelayerMessage) error {
	if len(messages) == 0 {
		return nil
	}

	if err := pp.prependMsgUpdateClient(src, dst, &messages); err != nil {
		return fmt.Errorf("error prepending MsgUpdateClient, chain_id: %s, client_id: %s, %w",
			dst.info.ChainID,
			dst.info.ClientID,
			err,
		)
	}

	pp.log.Debug("will send", zap.Any("messages", messages))

	return nil
}

// updateClientTrustedState will combine the counterparty chains trusted IBC header
// with the latest client state, which will be used for constructing MsgUpdateClient messages.
func (pp *PathProcessor) updateClientTrustedState(src *pathEndRuntime, dst *pathEndRuntime) {
	if src.clientTrustedState.ClientState.ConsensusHeight.GTE(src.clientState.ConsensusHeight) {
		// current height already trusted
		return
	}
	// need to assemble new trusted state
	ibcHeader, ok := dst.ibcHeaderCache[src.clientState.ConsensusHeight.RevisionHeight+1]
	if !ok {
		pp.log.Warn("No IBC header for client trusted height",
			zap.String("chain_id", src.info.ChainID),
			zap.String("client_id", src.info.ClientID),
			zap.Uint64("height", src.clientState.ConsensusHeight.RevisionHeight+1),
		)
		return
	}
	src.clientTrustedState = provider.ClientTrustedState{
		ClientState: src.clientState,
		IBCHeader:   ibcHeader,
	}
}

// messages from both pathEnds are needed in order to determine what needs to be relayed for a single pathEnd
func (pp *PathProcessor) processLatestMessages(ctx context.Context) error {
	// Update trusted client state for both pathends
	pp.updateClientTrustedState(pp.pathEnd1, pp.pathEnd2)
	pp.updateClientTrustedState(pp.pathEnd2, pp.pathEnd1)

	channelPairs := pp.channelPairs()

	// process the packet flows for both packends to determine what needs to be relayed
	pathEnd1ProcessRes := make([]*pathEndPacketFlowResponse, len(channelPairs))
	pathEnd2ProcessRes := make([]*pathEndPacketFlowResponse, len(channelPairs))

	var pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes pathEndConnectionHandshakeResponse
	var pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes pathEndChannelHandshakeResponse

	var wg sync.WaitGroup

	pathEnd1ConnectionHandshakeMessages := pathEndConnectionHandshakeMessages{
		Src:                         pp.pathEnd1,
		Dst:                         pp.pathEnd2,
		SrcMsgConnectionOpenInit:    pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenConfirm],
	}
	pathEnd2ConnectionHandshakeMessages := pathEndConnectionHandshakeMessages{
		Src:                         pp.pathEnd2,
		Dst:                         pp.pathEnd1,
		SrcMsgConnectionOpenInit:    pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd2.messageCache.ConnectionHandshake[MsgConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd1.messageCache.ConnectionHandshake[MsgConnectionOpenConfirm],
	}
	wg.Add(2)
	go pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd1ConnectionHandshakeMessages, &wg, &pathEnd1ConnectionHandshakeRes)
	go pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd2ConnectionHandshakeMessages, &wg, &pathEnd2ConnectionHandshakeRes)

	pathEnd1ChannelHandshakeMessages := pathEndChannelHandshakeMessages{
		Src:                      pp.pathEnd1,
		Dst:                      pp.pathEnd2,
		SrcMsgChannelOpenInit:    pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenConfirm],
	}
	pathEnd2ChannelHandshakeMessages := pathEndChannelHandshakeMessages{
		Src:                      pp.pathEnd2,
		Dst:                      pp.pathEnd1,
		SrcMsgChannelOpenInit:    pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd2.messageCache.ChannelHandshake[MsgChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd1.messageCache.ChannelHandshake[MsgChannelOpenConfirm],
	}
	wg.Add(2)
	go pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd1ChannelHandshakeMessages, &wg, &pathEnd1ChannelHandshakeRes)
	go pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd2ChannelHandshakeMessages, &wg, &pathEnd2ChannelHandshakeRes)

	for i, pair := range channelPairs {
		pathEnd1PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                   pp.pathEnd1,
			Dst:                   pp.pathEnd2,
			SrcMsgTransfer:        pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgAcknowledgement],
			SrcMsgTimeout:         pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTimeout],
			SrcMsgTimeoutOnClose:  pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTimeoutOnClose],
		}
		pathEnd2PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                   pp.pathEnd2,
			Dst:                   pp.pathEnd1,
			SrcMsgTransfer:        pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgAcknowledgement],
			SrcMsgTimeout:         pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTimeout],
			SrcMsgTimeoutOnClose:  pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTimeoutOnClose],
		}

		pathEnd1ProcessRes[i] = new(pathEndPacketFlowResponse)
		pathEnd2ProcessRes[i] = new(pathEndPacketFlowResponse)

		wg.Add(2)
		go pp.getUnrelayedPacketsAndAcksAndToDelete(ctx, pathEnd1PacketFlowMessages, &wg, pathEnd1ProcessRes[i])
		go pp.getUnrelayedPacketsAndAcksAndToDelete(ctx, pathEnd2PacketFlowMessages, &wg, pathEnd2ProcessRes[i])
	}
	wg.Wait()

	// concatenate applicable messages for pathend
	var pathEnd1Messages, pathEnd2Messages []provider.RelayerMessage

	pathEnd1Messages = append(pathEnd1Messages, pathEnd1ConnectionHandshakeRes.SrcMessages...)
	pathEnd1Messages = append(pathEnd1Messages, pathEnd2ConnectionHandshakeRes.DstMessages...)

	pathEnd2Messages = append(pathEnd2Messages, pathEnd2ConnectionHandshakeRes.SrcMessages...)
	pathEnd2Messages = append(pathEnd2Messages, pathEnd1ConnectionHandshakeRes.DstMessages...)

	pp.pathEnd1.messageCache.ConnectionHandshake.DeleteCachedMessages(pathEnd1ConnectionHandshakeRes.ToDeleteSrc, pathEnd2ConnectionHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ConnectionHandshake.DeleteCachedMessages(pathEnd2ConnectionHandshakeRes.ToDeleteSrc, pathEnd1ConnectionHandshakeRes.ToDeleteDst)

	pathEnd1Messages = append(pathEnd1Messages, pathEnd1ChannelHandshakeRes.SrcMessages...)
	pathEnd1Messages = append(pathEnd1Messages, pathEnd2ChannelHandshakeRes.DstMessages...)

	pathEnd2Messages = append(pathEnd2Messages, pathEnd2ChannelHandshakeRes.SrcMessages...)
	pathEnd2Messages = append(pathEnd2Messages, pathEnd1ChannelHandshakeRes.DstMessages...)

	pp.pathEnd1.messageCache.ChannelHandshake.DeleteCachedMessages(pathEnd1ChannelHandshakeRes.ToDeleteSrc, pathEnd2ChannelHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ChannelHandshake.DeleteCachedMessages(pathEnd2ChannelHandshakeRes.ToDeleteSrc, pathEnd1ChannelHandshakeRes.ToDeleteDst)

	for i := 0; i < len(channelPairs); i++ {
		pathEnd1Messages = append(pathEnd1Messages, pathEnd2ProcessRes[i].DstMessages...)
		pathEnd1Messages = append(pathEnd1Messages, pathEnd1ProcessRes[i].SrcMessages...)

		pathEnd2Messages = append(pathEnd2Messages, pathEnd1ProcessRes[i].DstMessages...)
		pathEnd2Messages = append(pathEnd2Messages, pathEnd2ProcessRes[i].SrcMessages...)

		pp.pathEnd1.messageCache.PacketFlow[channelPairs[i].pathEnd1ChannelKey].DeleteCachedMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.messageCache.PacketFlow[channelPairs[i].pathEnd2ChannelKey].DeleteCachedMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)
	}

	// now send messages in parallel
	var eg errgroup.Group
	eg.Go(func() error {
		return pp.sendMessages(pp.pathEnd1, pp.pathEnd2, pathEnd2Messages)
	})
	eg.Go(func() error {
		return pp.sendMessages(pp.pathEnd2, pp.pathEnd1, pathEnd1Messages)
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
		if err := pp.processLatestMessages(ctx); err != nil {
			// in case of IBC message send errors, schedule retry after durationErrorRetry
			time.AfterFunc(durationErrorRetry, pp.ProcessBacklogIfReady)
		}
	}
}
