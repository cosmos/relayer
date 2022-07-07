package processor

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// assembleIBCMessage constructs the applicable IBC message using the requested function.
// These functions may do things like make queries in order to assemble a complete IBC message.
func (pp *PathProcessor) assemblePacketIBCMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	partialMessage packetIBCMessage,
	assembleMessage func(ctx context.Context, msgRecvPacket provider.PacketInfo, signer string, latest provider.LatestBlock) (provider.RelayerMessage, error),
) (provider.RelayerMessage, error) {
	signer, err := dst.chainProvider.Address()
	if err != nil {
		return nil, fmt.Errorf("error getting signer address for {%s}: %w", dst.info.ChainID, err)
	}
	assembled, err := assembleMessage(ctx, partialMessage.info, signer, src.latestBlock)
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
					action: MsgAcknowledgement,
					info:   msgAcknowledgement,
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
					action: MsgTimeout,
					info:   msgTransfer,
				}
				if pathEndPacketFlowMessages.Src.shouldSendPacketMessage(timeoutMsg, pathEndPacketFlowMessages.Dst) {
					res.SrcMessages = append(res.SrcMessages, timeoutMsg)
				}
			case errors.As(err, &timeoutOnCloseErr):
				timeoutOnCloseMsg := packetIBCMessage{
					action: MsgTimeoutOnClose,
					info:   msgTransfer,
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
			action: MsgRecvPacket,
			info:   msgTransfer,
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
		var foundOpenTry *provider.ConnectionInfo
		for openTryKey, openTryMsg := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenTry {
			if openInitKey == openTryKey {
				foundOpenTry = &openTryMsg
				break
			}
		}
		if foundOpenTry == nil {
			// need to send an open try to dst
			msgOpenTry := connectionIBCMessage{
				action: MsgConnectionOpenTry,
				info:   openInitMsg,
			}
			if pathEndConnectionHandshakeMessages.Dst.shouldSendConnectionMessage(msgOpenTry, pathEndConnectionHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenTry)
			}
			continue ConnectionHandshakeLoop
		}
		var foundOpenAck *provider.ConnectionInfo
		for openAckKey, openAckMsg := range pathEndConnectionHandshakeMessages.SrcMsgConnectionOpenAck {
			if openInitKey == openAckKey {
				foundOpenAck = &openAckMsg
				break
			}
		}
		if foundOpenAck == nil {
			// need to send an open ack to src
			msgOpenAck := connectionIBCMessage{
				action: MsgConnectionOpenAck,
				info:   *foundOpenTry,
			}
			if pathEndConnectionHandshakeMessages.Src.shouldSendConnectionMessage(msgOpenAck, pathEndConnectionHandshakeMessages.Dst) {
				res.SrcMessages = append(res.SrcMessages, msgOpenAck)
			}
			continue ConnectionHandshakeLoop
		}
		var foundOpenConfirm *provider.ConnectionInfo
		for openConfirmKey, openConfirmMsg := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenConfirm {
			if openInitKey == openConfirmKey {
				foundOpenConfirm = &openConfirmMsg
				break
			}
		}
		if foundOpenConfirm == nil {
			// need to send an open confirm to dst
			msgOpenConfirm := connectionIBCMessage{
				action: MsgConnectionOpenConfirm,
				info:   *foundOpenAck,
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
		var foundOpenTry *provider.ChannelInfo
		for openTryKey, openTryMsg := range pathEndChannelHandshakeMessages.DstMsgChannelOpenTry {
			if openInitKey == openTryKey {
				foundOpenTry = &openTryMsg
				break
			}
		}
		if foundOpenTry == nil {
			// need to send an open try to dst
			msgOpenTry := channelIBCMessage{
				action: MsgChannelOpenTry,
				info:   openInitMsg,
			}
			if pathEndChannelHandshakeMessages.Dst.shouldSendChannelMessage(msgOpenTry, pathEndChannelHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenTry)
			}
			continue ChannelHandshakeLoop
		}
		var foundOpenAck *provider.ChannelInfo
		for openAckKey, openAckMsg := range pathEndChannelHandshakeMessages.SrcMsgChannelOpenAck {
			if openInitKey == openAckKey {
				foundOpenAck = &openAckMsg
				break
			}
		}
		if foundOpenAck == nil {
			// need to send an open ack to src
			msgOpenAck := channelIBCMessage{
				action: MsgChannelOpenAck,
				info:   *foundOpenTry,
			}
			if pathEndChannelHandshakeMessages.Src.shouldSendChannelMessage(msgOpenAck, pathEndChannelHandshakeMessages.Dst) {
				res.SrcMessages = append(res.SrcMessages, msgOpenAck)
			}
			continue ChannelHandshakeLoop
		}
		var foundOpenConfirm *provider.ChannelInfo
		for openConfirmKey, openConfirmMsg := range pathEndChannelHandshakeMessages.DstMsgChannelOpenConfirm {
			if openInitKey == openConfirmKey {
				foundOpenConfirm = &openConfirmMsg
				break
			}
		}
		if foundOpenConfirm == nil {
			// need to send an open confirm to dst
			msgOpenConfirm := channelIBCMessage{
				action: MsgChannelOpenConfirm,
				info:   *foundOpenAck,
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
func (pp *PathProcessor) processLatestMessages(ctx context.Context, messageLifecycle MessageLifecycle) error {
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

	if messageLifecycle != nil && !pp.sentInitialMsg {
		switch m := messageLifecycle.(type) {
		case *PacketMessageLifecycle:
			if m.Initial == nil {
				break
			}
			channelKey, err := PacketInfoChannelKey(m.Initial.Action, m.Initial.Info)
			if err != nil {
				pp.log.Error("Unexpected error checking packet message",
					zap.String("action", m.Termination.Action),
					zap.Any("channel", channelKey),
					zap.Error(err),
				)
				break
			}
			if !pp.IsRelayedChannel(m.Initial.ChainID, channelKey) {
				break
			}
			if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
				pathEnd1PacketMessages = append(pathEnd1PacketMessages, packetIBCMessage{
					action: m.Initial.Action,
					info:   m.Initial.Info,
				})
			} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
				pathEnd2PacketMessages = append(pathEnd2PacketMessages, packetIBCMessage{
					action: m.Initial.Action,
					info:   m.Initial.Info,
				})
			}
		case *ConnectionMessageLifecycle:
			if m.Initial == nil {
				break
			}
			if !pp.IsRelevantConnection(m.Initial.ChainID, m.Initial.Info.ConnID) {
				break
			}
			if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
				pathEnd1ConnectionMessages = append(pathEnd1ConnectionMessages, connectionIBCMessage{
					action: m.Initial.Action,
					info:   m.Initial.Info,
				})
			} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
				pathEnd2ConnectionMessages = append(pathEnd2ConnectionMessages, connectionIBCMessage{
					action: m.Initial.Action,
					info:   m.Initial.Info,
				})
			}
		case *ChannelMessageLifecycle:
			if m.Initial == nil {
				break
			}
			if !pp.IsRelevantChannel(m.Initial.ChainID, m.Initial.Info.ChannelID) {
				break
			}
			if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
				pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, channelIBCMessage{
					action: m.Initial.Action,
					info:   m.Initial.Info,
				})
			} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
				pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, channelIBCMessage{
					action: m.Initial.Action,
					info:   m.Initial.Info,
				})
			}
		}

		pp.sentInitialMsg = true
	}

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

func (pp *PathProcessor) assembleAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	packetMessages []packetIBCMessage,
	connectionMessages []connectionIBCMessage,
	channelMessages []channelIBCMessage,
) error {
	if len(packetMessages) == 0 && len(connectionMessages) == 0 && len(channelMessages) == 0 {
		return nil
	}
	var outgoingMessages []provider.RelayerMessage
	msgUpdateClient, err := pp.assembleMsgUpdateClient(ctx, src, dst)
	if err != nil {
		return err
	}
	outgoingMessages = append(outgoingMessages, msgUpdateClient)

	var sentPackageMessages []packetIBCMessage

	for _, msg := range packetMessages {
		var assembleMessage func(context.Context, provider.PacketInfo, string, provider.LatestBlock) (provider.RelayerMessage, error)
		switch msg.action {
		case MsgRecvPacket:
			assembleMessage = src.chainProvider.MsgRecvPacket
		case MsgAcknowledgement:
			assembleMessage = src.chainProvider.MsgAcknowledgement
		case MsgTimeout:
			assembleMessage = src.chainProvider.MsgTimeout
		case MsgTimeoutOnClose:
			assembleMessage = src.chainProvider.MsgTimeoutOnClose
		default:
			pp.log.Error("Unexepected packet message action for message assembly",
				zap.String("action", msg.action),
			)
			continue
		}

		message, err := pp.assemblePacketIBCMessage(ctx, src, dst, msg, assembleMessage)
		dst.trackProcessingPacketMessage(msg, err == nil)
		if err != nil {
			pp.log.Error("Error assembling packet message", zap.Error(err))
			continue
		}
		sentPackageMessages = append(sentPackageMessages, msg)
		outgoingMessages = append(outgoingMessages, message)
	}

	// TODO handle connection and channel handshake messages

	_, txSuccess, err := dst.chainProvider.SendMessages(ctx, outgoingMessages)
	if err != nil {
		return fmt.Errorf("error sending messages: %w", err)
	}
	if !txSuccess {
		return errors.New("error sending messages, transaction was not successful")
	}

	return nil
}

func (pp *PathProcessor) channelMessagesToSend(pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes pathEndChannelHandshakeResponse) ([]channelIBCMessage, []channelIBCMessage) {
	pathEnd1ChannelSrcLen := len(pathEnd1ChannelHandshakeRes.SrcMessages)
	pathEnd1ChannelDstLen := len(pathEnd1ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelDstLen := len(pathEnd2ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelSrcLen := len(pathEnd2ChannelHandshakeRes.SrcMessages)
	pathEnd1ChannelMessages := make([]channelIBCMessage, 0, pathEnd1ChannelSrcLen+pathEnd2ChannelDstLen)
	pathEnd2ChannelMessages := make([]channelIBCMessage, 0, pathEnd2ChannelSrcLen+pathEnd1ChannelDstLen)

	// pathEnd1 channel messages come from pathEnd1 src and pathEnd2 dst
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd2ChannelHandshakeRes.DstMessages...)
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd1ChannelHandshakeRes.SrcMessages...)

	// pathEnd2 channel messages come from pathEnd2 src and pathEnd1 dst
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd1ChannelHandshakeRes.DstMessages...)
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd2ChannelHandshakeRes.SrcMessages...)

	pp.pathEnd1.messageCache.ChannelHandshake.DeleteMessages(pathEnd1ChannelHandshakeRes.ToDeleteSrc, pathEnd2ChannelHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ChannelHandshake.DeleteMessages(pathEnd2ChannelHandshakeRes.ToDeleteSrc, pathEnd1ChannelHandshakeRes.ToDeleteDst)

	return pathEnd1ChannelMessages, pathEnd2ChannelMessages
}

func (pp *PathProcessor) connectionMessagesToSend(pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes pathEndConnectionHandshakeResponse) ([]connectionIBCMessage, []connectionIBCMessage) {
	pathEnd1ConnectionSrcLen := len(pathEnd1ConnectionHandshakeRes.SrcMessages)
	pathEnd1ConnectionDstLen := len(pathEnd1ConnectionHandshakeRes.DstMessages)
	pathEnd2ConnectionDstLen := len(pathEnd2ConnectionHandshakeRes.DstMessages)
	pathEnd2ConnectionSrcLen := len(pathEnd2ConnectionHandshakeRes.SrcMessages)
	pathEnd1ConnectionMessages := make([]connectionIBCMessage, 0, pathEnd1ConnectionSrcLen+pathEnd2ConnectionDstLen)
	pathEnd2ConnectionMessages := make([]connectionIBCMessage, 0, pathEnd2ConnectionSrcLen+pathEnd1ConnectionDstLen)

	// pathEnd1 connection messages come from pathEnd1 src and pathEnd2 dst
	pathEnd1ConnectionMessages = append(pathEnd1ConnectionMessages, pathEnd2ConnectionHandshakeRes.DstMessages...)
	pathEnd1ConnectionMessages = append(pathEnd1ConnectionMessages, pathEnd1ConnectionHandshakeRes.SrcMessages...)

	// pathEnd2 connection messages come from pathEnd2 src and pathEnd1 dst
	pathEnd2ConnectionMessages = append(pathEnd2ConnectionMessages, pathEnd1ConnectionHandshakeRes.DstMessages...)
	pathEnd2ConnectionMessages = append(pathEnd2ConnectionMessages, pathEnd2ConnectionHandshakeRes.SrcMessages...)

	pp.pathEnd1.messageCache.ConnectionHandshake.DeleteMessages(pathEnd1ConnectionHandshakeRes.ToDeleteSrc, pathEnd2ConnectionHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ConnectionHandshake.DeleteMessages(pathEnd2ConnectionHandshakeRes.ToDeleteSrc, pathEnd1ConnectionHandshakeRes.ToDeleteDst)
	return pathEnd1ConnectionMessages, pathEnd2ConnectionMessages
}

func (pp *PathProcessor) packetMessagesToSend(channelPairs []channelPair, pathEnd1ProcessRes []pathEndPacketFlowResponse, pathEnd2ProcessRes []pathEndPacketFlowResponse) ([]packetIBCMessage, []packetIBCMessage) {
	pathEnd1PacketLen := 0
	pathEnd2PacketLen := 0
	for i := 0; i < len(channelPairs); i++ {
		pathEnd1PacketLen += len(pathEnd2ProcessRes[i].DstMessages) + len(pathEnd1ProcessRes[i].SrcMessages)
		pathEnd2PacketLen += len(pathEnd1ProcessRes[i].DstMessages) + len(pathEnd2ProcessRes[i].SrcMessages)
	}

	pathEnd1PacketMessages := make([]packetIBCMessage, 0, pathEnd1PacketLen)
	pathEnd2PacketMessages := make([]packetIBCMessage, 0, pathEnd2PacketLen)

	for i, channelPair := range channelPairs {
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd2ProcessRes[i].DstMessages...)
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd1ProcessRes[i].SrcMessages...)

		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd1ProcessRes[i].DstMessages...)
		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd2ProcessRes[i].SrcMessages...)

		pp.pathEnd1.messageCache.PacketFlow[channelPair.pathEnd1ChannelKey].DeleteMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.messageCache.PacketFlow[channelPair.pathEnd2ChannelKey].DeleteMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)
	}

	return pathEnd1PacketMessages, pathEnd2PacketMessages
}
