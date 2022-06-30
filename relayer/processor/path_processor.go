package processor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// durationErrorRetry determines how long to wait before retrying
	// in the case of failure to send transactions with IBC messages.
	durationErrorRetry       = 5 * time.Second
	blocksToRetryPacketAfter = 5
	maxMessageSendRetries    = 5

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

func NewPathProcessor(log *zap.Logger, pathEnd1 PathEnd, pathEnd2 PathEnd) *PathProcessor {
	return &PathProcessor{
		log:          log,
		pathEnd1:     newPathEndRuntime(pathEnd1),
		pathEnd2:     newPathEndRuntime(pathEnd2),
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
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ClientID == clientID
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ClientID == clientID
	}
	return false
}

func (pp *PathProcessor) IsRelevantConnection(chainID string, connectionID string) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.isRelevantConnection(connectionID)
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.isRelevantConnection(connectionID)
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

// assembleIBCMessage constructs the applicable IBC message using the requested function.
// These functions may do things like make queries in order to assemble a complete IBC message.
func (pp *PathProcessor) assemblePacketIBCMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	partialMessage packetIBCMessage,
	assembleMessage func(ctx context.Context, msgRecvPacket provider.RelayerMessage, signer string, latest provider.LatestBlock) (provider.RelayerMessage, error),
) (provider.RelayerMessage, error) {
	signer, err := dst.chainProvider.Address()
	if err != nil {
		return nil, fmt.Errorf("error getting signer address for {%s}: %w", dst.info.ChainID, err)
	}
	assembled, err := assembleMessage(ctx, partialMessage.message, signer, src.latestBlock)
	if err != nil {
		return nil, fmt.Errorf("error assembling %s for {%s}: %w", partialMessage.action, dst.info.ChainID, err)
	}

	return assembled, nil
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(ctx context.Context, pathEndPacketFlowMessages pathEndPacketFlowMessages) pathEndPacketFlowResponse {
	res := pathEndPacketFlowResponse{
		ToDeleteSrc: make(map[string][]uint64),
		ToDeleteDst: make(map[string][]uint64),
	}

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
				ackMsg := packetIBCMessage{
					channelKey: pathEndPacketFlowMessages.ChannelKey,
					action:     MsgAcknowledgement,
					sequence:   transferSeq,
					message:    msgAcknowledgement,
				}
				if pathEndPacketFlowMessages.Src.shouldSendPacketMessage(ackMsg, pathEndPacketFlowMessages.Dst) {
					res.SrcMessages = append(res.SrcMessages, ackMsg)
				}
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay either MsgRecvPacket from src to dst, or MsgTimeout/MsgTimeoutOnClose from dst to src
		if err := pathEndPacketFlowMessages.Dst.chainProvider.ValidatePacket(msgTransfer, pathEndPacketFlowMessages.Dst.latestBlock); err != nil {
			var timeoutHeightErr *provider.TimeoutHeightError
			var timeoutTimestampErr *provider.TimeoutTimestampError
			var timeoutOnCloseErr *provider.TimeoutOnCloseError

			switch {
			case errors.As(err, &timeoutHeightErr) || errors.As(err, &timeoutTimestampErr):
				timeoutMsg := packetIBCMessage{
					channelKey: pathEndPacketFlowMessages.ChannelKey,
					action:     MsgTimeout,
					sequence:   transferSeq,
					message:    msgTransfer,
				}
				if pathEndPacketFlowMessages.Src.shouldSendPacketMessage(timeoutMsg, pathEndPacketFlowMessages.Dst) {
					res.SrcMessages = append(res.SrcMessages, timeoutMsg)
				}
			case errors.As(err, &timeoutOnCloseErr):
				timeoutOnCloseMsg := packetIBCMessage{
					channelKey: pathEndPacketFlowMessages.ChannelKey,
					action:     MsgTimeoutOnClose,
					sequence:   transferSeq,
					message:    msgTransfer,
				}
				if pathEndPacketFlowMessages.Src.shouldSendPacketMessage(timeoutOnCloseMsg, pathEndPacketFlowMessages.Dst) {
					res.SrcMessages = append(res.SrcMessages, timeoutOnCloseMsg)
				}
			default:
				pp.log.Error("Packet is invalid",
					zap.String("chain_id", pathEndPacketFlowMessages.Src.info.ChainID),
					zap.Error(err),
				)
			}
			continue MsgTransferLoop
		}
		recvPacketMsg := packetIBCMessage{
			channelKey: pathEndPacketFlowMessages.ChannelKey.Counterparty(),
			action:     MsgRecvPacket,
			sequence:   transferSeq,
			message:    msgTransfer,
		}
		if pathEndPacketFlowMessages.Dst.shouldSendPacketMessage(recvPacketMsg, pathEndPacketFlowMessages.Src) {
			res.DstMessages = append(res.DstMessages, recvPacketMsg)
		}
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
	return res
}

func (pp *PathProcessor) getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEndConnectionHandshakeMessages pathEndConnectionHandshakeMessages) pathEndConnectionHandshakeResponse {
	res := pathEndConnectionHandshakeResponse{
		ToDeleteSrc: make(map[string][]ConnectionKey),
		ToDeleteDst: make(map[string][]ConnectionKey),
	}

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
			msgOpenTry := connectionIBCMessage{
				action:        MsgConnectionOpenTry,
				connectionKey: openInitKey.Counterparty(),
				message:       openInitMsg,
			}
			if pathEndConnectionHandshakeMessages.Dst.shouldSendConnectionMessage(msgOpenTry, pathEndConnectionHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenTry)
			}
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
			msgOpenAck := connectionIBCMessage{
				action:        MsgConnectionOpenAck,
				connectionKey: openInitKey,
				message:       foundOpenTry,
			}
			if pathEndConnectionHandshakeMessages.Src.shouldSendConnectionMessage(msgOpenAck, pathEndConnectionHandshakeMessages.Dst) {
				res.SrcMessages = append(res.SrcMessages, msgOpenAck)
			}
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
			msgOpenConfirm := connectionIBCMessage{
				action:        MsgConnectionOpenConfirm,
				connectionKey: openInitKey.Counterparty(),
				message:       foundOpenAck,
			}
			if pathEndConnectionHandshakeMessages.Dst.shouldSendConnectionMessage(msgOpenConfirm, pathEndConnectionHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenConfirm)
			}
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
	return res
}

func (pp *PathProcessor) getUnrelayedChannelHandshakeMessagesAndToDelete(pathEndChannelHandshakeMessages pathEndChannelHandshakeMessages) pathEndChannelHandshakeResponse {
	res := pathEndChannelHandshakeResponse{
		ToDeleteSrc: make(map[string][]ChannelKey),
		ToDeleteDst: make(map[string][]ChannelKey),
	}

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
			msgOpenTry := channelIBCMessage{
				action:     MsgChannelOpenTry,
				channelKey: openInitKey.Counterparty(),
				message:    openInitMsg,
			}
			if pathEndChannelHandshakeMessages.Dst.shouldSendChannelMessage(msgOpenTry, pathEndChannelHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenTry)
			}
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
			msgOpenAck := channelIBCMessage{
				action:     MsgChannelOpenAck,
				channelKey: openInitKey,
				message:    foundOpenTry,
			}
			if pathEndChannelHandshakeMessages.Src.shouldSendChannelMessage(msgOpenAck, pathEndChannelHandshakeMessages.Dst) {
				res.SrcMessages = append(res.SrcMessages, msgOpenAck)
			}
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
			msgOpenConfirm := channelIBCMessage{
				action:     MsgChannelOpenConfirm,
				channelKey: openInitKey.Counterparty(),
				message:    foundOpenAck,
			}
			if pathEndChannelHandshakeMessages.Dst.shouldSendChannelMessage(msgOpenConfirm, pathEndChannelHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenConfirm)
			}
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
	return res
}

// assembleMsgUpdateClient uses the ChainProvider from both pathEnds to assemble the client update header
// from the source and then assemble the update client message in the correct format for the destination.
func (pp *PathProcessor) assembleMsgUpdateClient(ctx context.Context, src, dst *pathEndRuntime) (provider.RelayerMessage, error) {
	clientID := dst.info.ClientID
	clientConsensusHeight := dst.clientState.ConsensusHeight
	trustedConsensusHeight := dst.clientTrustedState.ClientState.ConsensusHeight

	// If the client state height is not equal to the client trusted state height and the client state height is
	// the latest block, we cannot send a MsgUpdateClient until another block is observed on the counterparty.
	// If the client state height is in the past, beyond ibcHeadersToCache, then we need to query for it.
	if !trustedConsensusHeight.EQ(clientConsensusHeight) {
		if int64(clientConsensusHeight.RevisionHeight)-int64(trustedConsensusHeight.RevisionHeight) <= ibcHeadersToCache {
			return nil, fmt.Errorf("observed client trusted height: %d does not equal latest client state height: %d",
				trustedConsensusHeight.RevisionHeight, clientConsensusHeight.RevisionHeight)
		}
		header, err := src.chainProvider.IBCHeaderAtHeight(ctx, int64(clientConsensusHeight.RevisionHeight+1))
		if err != nil {
			return nil, fmt.Errorf("error getting IBC header at height: %d for chain_id: %s, %w", clientConsensusHeight.RevisionHeight+1, src.info.ChainID, err)
		}
		pp.log.Warn("Had to query for client trusted IBC header",
			zap.String("chain_id", src.info.ChainID),
			zap.String("counterparty_chain_id", dst.info.ChainID),
			zap.String("counterparty_client_id", clientID),
			zap.Uint64("height", clientConsensusHeight.RevisionHeight+1),
			zap.Uint64("latest_height", src.latestBlock.Height),
		)
		dst.clientTrustedState = provider.ClientTrustedState{
			ClientState: dst.clientState,
			IBCHeader:   header,
		}
		trustedConsensusHeight = clientConsensusHeight
	}

	if src.latestHeader.Height() == trustedConsensusHeight.RevisionHeight {
		return nil, fmt.Errorf("latest header height is equal to the client trusted height: %d, "+
			"need to wait for next block's header before we can assemble and send a new MsgUpdateClient",
			trustedConsensusHeight.RevisionHeight)
	}

	msgUpdateClientHeader, err := src.chainProvider.MsgUpdateClientHeader(src.latestHeader, trustedConsensusHeight, dst.clientTrustedState.IBCHeader)
	if err != nil {
		return nil, fmt.Errorf("error assembling new client header: %w", err)
	}

	msgUpdateClient, err := dst.chainProvider.MsgUpdateClient(clientID, msgUpdateClientHeader)
	if err != nil {
		return nil, fmt.Errorf("error assembling MsgUpdateClient: %w", err)
	}

	return msgUpdateClient, nil
}

// updateClientTrustedState combines the counterparty chains trusted IBC header
// with the latest client state, which will be used for constructing MsgUpdateClient messages.
func (pp *PathProcessor) updateClientTrustedState(src *pathEndRuntime, dst *pathEndRuntime) {
	if src.clientTrustedState.ClientState.ConsensusHeight.GTE(src.clientState.ConsensusHeight) {
		// current height already trusted
		return
	}
	// need to assemble new trusted state
	ibcHeader, ok := dst.ibcHeaderCache[src.clientState.ConsensusHeight.RevisionHeight+1]
	if !ok {
		pp.log.Warn("No cached IBC header for client trusted height",
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
	pathEnd1ConnectionHandshakeRes := pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd1ConnectionHandshakeMessages)
	pathEnd2ConnectionHandshakeRes := pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd2ConnectionHandshakeMessages)

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
	pathEnd1ChannelHandshakeRes := pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd1ChannelHandshakeMessages)
	pathEnd2ChannelHandshakeRes := pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd2ChannelHandshakeMessages)

	// process the packet flows for both path ends to determine what needs to be relayed
	pathEnd1ProcessRes := make([]pathEndPacketFlowResponse, len(channelPairs))
	pathEnd2ProcessRes := make([]pathEndPacketFlowResponse, len(channelPairs))

	for i, pair := range channelPairs {
		pathEnd1PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                   pp.pathEnd1,
			Dst:                   pp.pathEnd2,
			ChannelKey:            pair.pathEnd1ChannelKey,
			SrcMsgTransfer:        pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgAcknowledgement],
			SrcMsgTimeout:         pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTimeout],
			SrcMsgTimeoutOnClose:  pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgTimeoutOnClose],
		}
		pathEnd2PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                   pp.pathEnd2,
			Dst:                   pp.pathEnd1,
			ChannelKey:            pair.pathEnd2ChannelKey,
			SrcMsgTransfer:        pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgAcknowledgement],
			SrcMsgTimeout:         pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTimeout],
			SrcMsgTimeoutOnClose:  pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][MsgTimeoutOnClose],
		}

		pathEnd1ProcessRes[i] = pp.getUnrelayedPacketsAndAcksAndToDelete(ctx, pathEnd1PacketFlowMessages)
		pathEnd2ProcessRes[i] = pp.getUnrelayedPacketsAndAcksAndToDelete(ctx, pathEnd2PacketFlowMessages)
	}

	// concatenate applicable messages for pathend
	pathEnd1ConnectionMessages, pathEnd2ConnectionMessages := pp.connectionMessagesToSend(pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes)
	pathEnd1ChannelMessages, pathEnd2ChannelMessages := pp.channelMessagesToSend(pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes)
	pathEnd1PacketMessages, pathEnd2PacketMessages := pp.packetMessagesToSend(channelPairs, pathEnd1ProcessRes, pathEnd2ProcessRes)

	// now assemble and send messages in parallel
	// if sending messages fails to one pathEnd, we don't need to halt sending to the other pathEnd.
	var eg errgroup.Group
	eg.Go(func() error {
		if err := pp.assembleAndSendMessages(ctx, pp.pathEnd2, pp.pathEnd1, pathEnd1PacketMessages, pathEnd1ConnectionMessages, pathEnd1ChannelMessages); err != nil {
			pp.log.Error("Error sending messages",
				zap.String("src_chain_id", pp.pathEnd1.info.ChainID),
				zap.String("dst_chain_id", pp.pathEnd2.info.ChainID),
				zap.String("dst_client_id", pp.pathEnd2.info.ClientID),
				zap.Error(err),
			)
			return err
		}
		return nil
	})
	eg.Go(func() error {
		if err := pp.assembleAndSendMessages(ctx, pp.pathEnd1, pp.pathEnd2, pathEnd2PacketMessages, pathEnd2ConnectionMessages, pathEnd2ChannelMessages); err != nil {
			pp.log.Error("Error sending messages",
				zap.String("src_chain_id", pp.pathEnd2.info.ChainID),
				zap.String("dst_chain_id", pp.pathEnd1.info.ChainID),
				zap.String("dst_client_id", pp.pathEnd1.info.ClientID),
				zap.Error(err),
			)
			return err
		}
		return nil
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
