package processor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
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
		return nil, fmt.Errorf("error assembling %s for {%s}: %w", partialMessage.eventType, dst.info.ChainID, err)
	}

	return assembled, nil
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(ctx context.Context, pathEndPacketFlowMessages pathEndPacketFlowMessages) pathEndPacketFlowResponse {
	res := pathEndPacketFlowResponse{
		ToDeleteSrc:        make(map[string][]uint64),
		ToDeleteDst:        make(map[string][]uint64),
		ToDeleteDstChannel: make(map[string][]ChannelKey),
	}

	dstRecvPacketMsgs := make([]packetIBCMessage, 0)

MsgTransferLoop:
	for transferSeq, msgTransfer := range pathEndPacketFlowMessages.SrcMsgTransfer {
		for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
			if transferSeq == ackSeq {
				// we have an ack for this packet, so packet flow is complete
				// remove all retention of this sequence number
				res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], transferSeq)
				res.ToDeleteDst[chantypes.EventTypeRecvPacket] = append(res.ToDeleteDst[chantypes.EventTypeRecvPacket], transferSeq)
				res.ToDeleteDst[chantypes.EventTypeWriteAck] = append(res.ToDeleteDst[chantypes.EventTypeWriteAck], transferSeq)
				res.ToDeleteSrc[chantypes.EventTypeAcknowledgePacket] = append(res.ToDeleteSrc[chantypes.EventTypeAcknowledgePacket], transferSeq)
				continue MsgTransferLoop
			}
		}

		for timeoutSeq, msgTimeout := range pathEndPacketFlowMessages.SrcMsgTimeout {
			if transferSeq == timeoutSeq {
				if msgTimeout.ChannelOrder == chantypes.ORDERED.String() {
					// For ordered channel packets, flow is not done until channel-close-confirm is observed.
					if pathEndPacketFlowMessages.DstMsgChannelCloseConfirm == nil {
						// have not observed a channel-close-confirm yet for this channel, send it if ready.
						// will come back through here next block if not yet ready.
						closeChan := channelIBCMessage{
							eventType: chantypes.EventTypeChannelCloseConfirm,
							info: provider.ChannelInfo{
								Height:                msgTimeout.Height,
								PortID:                msgTimeout.SourcePort,
								ChannelID:             msgTimeout.SourceChannel,
								CounterpartyPortID:    msgTimeout.DestPort,
								CounterpartyChannelID: msgTimeout.DestChannel,
								Order:                 orderFromString(msgTimeout.ChannelOrder),
							},
						}

						if pathEndPacketFlowMessages.Dst.shouldSendChannelMessage(closeChan, pathEndPacketFlowMessages.Src) {
							res.DstChannelMessage = append(res.DstChannelMessage, closeChan)
						}
					} else {
						// ordered channel, and we have a channel close confirm, so packet-flow and channel-close-flow is complete.
						// remove all retention of this sequence number and this channel-close-confirm.
						res.ToDeleteDstChannel[chantypes.EventTypeChannelCloseConfirm] = append(res.ToDeleteDstChannel[chantypes.EventTypeChannelCloseConfirm], pathEndPacketFlowMessages.ChannelKey.Counterparty())
						res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], transferSeq)
						res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket] = append(res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket], transferSeq)
					}
				} else {
					// unordered channel, and we have a timeout for this packet, so packet flow is complete
					// remove all retention of this sequence number
					res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], transferSeq)
					res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket] = append(res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket], transferSeq)
				}
				continue MsgTransferLoop
			}
		}
		for timeoutOnCloseSeq := range pathEndPacketFlowMessages.SrcMsgTimeoutOnClose {
			if transferSeq == timeoutOnCloseSeq {
				// we have a timeout for this packet, so packet flow is complete
				// remove all retention of this sequence number
				res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], transferSeq)
				res.ToDeleteSrc[chantypes.EventTypeTimeoutPacketOnClose] = append(res.ToDeleteSrc[chantypes.EventTypeTimeoutPacketOnClose], transferSeq)
				continue MsgTransferLoop
			}
		}
		for msgRecvSeq, msgAcknowledgement := range pathEndPacketFlowMessages.DstMsgRecvPacket {
			if transferSeq == msgRecvSeq {
				if len(msgAcknowledgement.Ack) == 0 {
					// have recv_packet but not write_acknowledgement yet. skip for now.
					continue MsgTransferLoop
				}
				// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
				ackMsg := packetIBCMessage{
					eventType: chantypes.EventTypeAcknowledgePacket,
					info:      msgAcknowledgement,
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
					eventType: chantypes.EventTypeTimeoutPacket,
					info:      msgTransfer,
				}
				if pathEndPacketFlowMessages.Src.shouldSendPacketMessage(timeoutMsg, pathEndPacketFlowMessages.Dst) {
					res.SrcMessages = append(res.SrcMessages, timeoutMsg)
				}
			case errors.As(err, &timeoutOnCloseErr):
				timeoutOnCloseMsg := packetIBCMessage{
					eventType: chantypes.EventTypeTimeoutPacketOnClose,
					info:      msgTransfer,
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
			eventType: chantypes.EventTypeRecvPacket,
			info:      msgTransfer,
		}
		dstRecvPacketMsgs = append(dstRecvPacketMsgs, recvPacketMsg)
	}

	if len(dstRecvPacketMsgs) > 0 {
		sort.SliceStable(dstRecvPacketMsgs, func(i, j int) bool {
			return dstRecvPacketMsgs[i].info.Sequence < dstRecvPacketMsgs[j].info.Sequence
		})
		firstMsg := dstRecvPacketMsgs[0]
		if firstMsg.info.ChannelOrder == chantypes.ORDERED.String() {
			// for recv packet messages on ordered channels, only handle the lowest sequence number now.
			if pathEndPacketFlowMessages.Dst.shouldSendPacketMessage(firstMsg, pathEndPacketFlowMessages.Src) {
				res.DstMessages = append(res.DstMessages, firstMsg)
			}
		} else {
			// for unordered channels, can handle multiple simultaneous packets.
			for _, msg := range dstRecvPacketMsgs {
				if pathEndPacketFlowMessages.Dst.shouldSendPacketMessage(msg, pathEndPacketFlowMessages.Src) {
					res.DstMessages = append(res.DstMessages, msg)
				}
			}
		}
	}

	// now iterate through packet-flow-complete messages and remove any leftover messages if the MsgTransfer or MsgRecvPacket was in a previous block that we did not query
	for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
		res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], ackSeq)
		res.ToDeleteDst[chantypes.EventTypeRecvPacket] = append(res.ToDeleteDst[chantypes.EventTypeRecvPacket], ackSeq)
		res.ToDeleteDst[chantypes.EventTypeWriteAck] = append(res.ToDeleteDst[chantypes.EventTypeWriteAck], ackSeq)
		res.ToDeleteSrc[chantypes.EventTypeAcknowledgePacket] = append(res.ToDeleteSrc[chantypes.EventTypeAcknowledgePacket], ackSeq)
	}
	for timeoutSeq, msgTimeout := range pathEndPacketFlowMessages.SrcMsgTimeout {
		if msgTimeout.ChannelOrder != chantypes.ORDERED.String() {
			res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], timeoutSeq)
			res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket] = append(res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket], timeoutSeq)
		}
	}
	for timeoutOnCloseSeq := range pathEndPacketFlowMessages.SrcMsgTimeoutOnClose {
		res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], timeoutOnCloseSeq)
		res.ToDeleteSrc[chantypes.EventTypeTimeoutPacketOnClose] = append(res.ToDeleteSrc[chantypes.EventTypeTimeoutPacketOnClose], timeoutOnCloseSeq)
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
			// MsgConnectionOpenInit does not have counterparty connection ID, so check if everything
			// else matches for counterparty. If so, add counterparty connection ID for
			// the checks later on in this function.
			if openInitKey == openTryKey.Counterparty().MsgInitKey() {
				openInitKey.CounterpartyConnID = openTryKey.ConnectionID
				foundOpenTry = &openTryMsg
				break
			}
		}
		if foundOpenTry == nil {
			// need to send an open try to dst
			msgOpenTry := connectionIBCMessage{
				eventType: conntypes.EventTypeConnectionOpenTry,
				info:      openInitMsg,
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
				eventType: conntypes.EventTypeConnectionOpenAck,
				info:      *foundOpenTry,
			}
			if pathEndConnectionHandshakeMessages.Src.shouldSendConnectionMessage(msgOpenAck, pathEndConnectionHandshakeMessages.Dst) {
				res.SrcMessages = append(res.SrcMessages, msgOpenAck)
			}
			continue ConnectionHandshakeLoop
		}
		var foundOpenConfirm *provider.ConnectionInfo
		for openConfirmKey, openConfirmMsg := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenConfirm {
			if openInitKey == openConfirmKey.Counterparty() {
				foundOpenConfirm = &openConfirmMsg
				break
			}
		}
		if foundOpenConfirm == nil {
			// need to send an open confirm to dst
			msgOpenConfirm := connectionIBCMessage{
				eventType: conntypes.EventTypeConnectionOpenConfirm,
				info:      *foundOpenAck,
			}
			if pathEndConnectionHandshakeMessages.Dst.shouldSendConnectionMessage(msgOpenConfirm, pathEndConnectionHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenConfirm)
			}
			continue ConnectionHandshakeLoop
		}
		// handshake is complete for this connection, remove all retention.
		res.ToDeleteDst[conntypes.EventTypeConnectionOpenTry] = append(res.ToDeleteDst[conntypes.EventTypeConnectionOpenTry], openInitKey)
		res.ToDeleteSrc[conntypes.EventTypeConnectionOpenAck] = append(res.ToDeleteSrc[conntypes.EventTypeConnectionOpenAck], openInitKey)
		res.ToDeleteDst[conntypes.EventTypeConnectionOpenConfirm] = append(res.ToDeleteDst[conntypes.EventTypeConnectionOpenConfirm], openInitKey)

		// MsgConnectionOpenInit does not have CounterpartyConnectionID
		openInitKey.CounterpartyConnID = ""
		res.ToDeleteSrc[conntypes.EventTypeConnectionOpenInit] = append(res.ToDeleteSrc[conntypes.EventTypeConnectionOpenInit], openInitKey)
	}

	// now iterate through connection-handshake-complete messages and remove any leftover messages
	for openConfirmKey := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenConfirm {
		res.ToDeleteDst[conntypes.EventTypeConnectionOpenTry] = append(res.ToDeleteDst[conntypes.EventTypeConnectionOpenTry], openConfirmKey)
		res.ToDeleteSrc[conntypes.EventTypeConnectionOpenAck] = append(res.ToDeleteSrc[conntypes.EventTypeConnectionOpenAck], openConfirmKey)
		res.ToDeleteDst[conntypes.EventTypeConnectionOpenConfirm] = append(res.ToDeleteDst[conntypes.EventTypeConnectionOpenConfirm], openConfirmKey)

		// MsgConnectionOpenInit does not have CounterpartyConnectionID
		openConfirmKey.CounterpartyConnID = ""
		res.ToDeleteSrc[conntypes.EventTypeConnectionOpenInit] = append(res.ToDeleteSrc[conntypes.EventTypeConnectionOpenInit], openConfirmKey)
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
			// MsgChannelOpenInit does not have counterparty channel ID, so check if everything
			// else matches for counterparty. If so, add counterparty channel ID for
			// the checks later on in this function.
			if openInitKey == openTryKey.Counterparty().MsgInitKey() {
				openInitKey.CounterpartyChannelID = openTryMsg.ChannelID
				foundOpenTry = &openTryMsg
				break
			}
		}
		if foundOpenTry == nil {
			// need to send an open try to dst
			msgOpenTry := channelIBCMessage{
				eventType: chantypes.EventTypeChannelOpenTry,
				info:      openInitMsg,
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
				eventType: chantypes.EventTypeChannelOpenAck,
				info:      *foundOpenTry,
			}
			if pathEndChannelHandshakeMessages.Src.shouldSendChannelMessage(msgOpenAck, pathEndChannelHandshakeMessages.Dst) {
				res.SrcMessages = append(res.SrcMessages, msgOpenAck)
			}
			continue ChannelHandshakeLoop
		}
		var foundOpenConfirm *provider.ChannelInfo
		for openConfirmKey, openConfirmMsg := range pathEndChannelHandshakeMessages.DstMsgChannelOpenConfirm {
			if openInitKey == openConfirmKey.Counterparty() {
				foundOpenConfirm = &openConfirmMsg
				break
			}
		}
		if foundOpenConfirm == nil {
			// need to send an open confirm to dst
			msgOpenConfirm := channelIBCMessage{
				eventType: chantypes.EventTypeChannelOpenConfirm,
				info:      *foundOpenAck,
			}
			if pathEndChannelHandshakeMessages.Dst.shouldSendChannelMessage(msgOpenConfirm, pathEndChannelHandshakeMessages.Src) {
				res.DstMessages = append(res.DstMessages, msgOpenConfirm)
			}
			continue ChannelHandshakeLoop
		}
		// handshake is complete for this channel, remove all retention.
		res.ToDeleteDst[chantypes.EventTypeChannelOpenTry] = append(res.ToDeleteDst[chantypes.EventTypeChannelOpenTry], openInitKey)
		res.ToDeleteSrc[chantypes.EventTypeChannelOpenAck] = append(res.ToDeleteSrc[chantypes.EventTypeChannelOpenAck], openInitKey)
		res.ToDeleteDst[chantypes.EventTypeChannelOpenConfirm] = append(res.ToDeleteDst[chantypes.EventTypeChannelOpenConfirm], openInitKey)
		// MsgChannelOpenInit does not have CounterpartyChannelID
		res.ToDeleteSrc[chantypes.EventTypeChannelOpenInit] = append(res.ToDeleteSrc[chantypes.EventTypeChannelOpenInit], openInitKey.MsgInitKey())
	}

	// now iterate through channel-handshake-complete messages and remove any leftover messages
	for openConfirmKey := range pathEndChannelHandshakeMessages.DstMsgChannelOpenConfirm {
		res.ToDeleteDst[chantypes.EventTypeChannelOpenTry] = append(res.ToDeleteDst[chantypes.EventTypeChannelOpenTry], openConfirmKey)
		res.ToDeleteSrc[chantypes.EventTypeChannelOpenAck] = append(res.ToDeleteSrc[chantypes.EventTypeChannelOpenAck], openConfirmKey)
		res.ToDeleteDst[chantypes.EventTypeChannelOpenConfirm] = append(res.ToDeleteDst[chantypes.EventTypeChannelOpenConfirm], openConfirmKey)
		// MsgChannelOpenInit does not have CounterpartyChannelID
		res.ToDeleteSrc[chantypes.EventTypeChannelOpenInit] = append(res.ToDeleteSrc[chantypes.EventTypeChannelOpenInit], openConfirmKey.MsgInitKey())
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
		deltaConsensusHeight := int64(clientConsensusHeight.RevisionHeight) - int64(trustedConsensusHeight.RevisionHeight)
		if trustedConsensusHeight.RevisionHeight != 0 && deltaConsensusHeight <= clientConsensusHeightUpdateThresholdBlocks {
			return nil, fmt.Errorf("observed client trusted height: %d does not equal latest client state height: %d",
				trustedConsensusHeight.RevisionHeight, clientConsensusHeight.RevisionHeight)
		}
		header, err := src.chainProvider.QueryIBCHeader(ctx, int64(clientConsensusHeight.RevisionHeight+1))
		if err != nil {
			return nil, fmt.Errorf("error getting IBC header at height: %d for chain_id: %s, %w", clientConsensusHeight.RevisionHeight+1, src.info.ChainID, err)
		}
		pp.log.Debug("Had to query for client trusted IBC header",
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
		pp.log.Debug("No cached IBC header for client trusted height",
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

func (pp *PathProcessor) appendInitialMessageIfNecessary(msg MessageLifecycle, pathEnd1Messages, pathEnd2Messages *pathEndMessages) {
	if msg == nil || pp.sentInitialMsg {
		return
	}
	pp.sentInitialMsg = true
	switch m := msg.(type) {
	case *PacketMessageLifecycle:
		if m.Initial == nil {
			return
		}
		channelKey, err := PacketInfoChannelKey(m.Initial.EventType, m.Initial.Info)
		if err != nil {
			pp.log.Error("Unexpected error checking packet message",
				zap.String("event_type", m.Termination.EventType),
				zap.Inline(channelKey),
				zap.Error(err),
			)
			return
		}
		if !pp.IsRelayedChannel(m.Initial.ChainID, channelKey) {
			return
		}
		if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
			pathEnd1Messages.packetMessages = append(pathEnd1Messages.packetMessages, packetIBCMessage{
				eventType: m.Initial.EventType,
				info:      m.Initial.Info,
			})
		} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
			pathEnd2Messages.packetMessages = append(pathEnd2Messages.packetMessages, packetIBCMessage{
				eventType: m.Initial.EventType,
				info:      m.Initial.Info,
			})
		}
	case *ConnectionMessageLifecycle:
		if m.Initial == nil {
			return
		}
		if !pp.IsRelevantClient(m.Initial.ChainID, m.Initial.Info.ClientID) {
			return
		}
		if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
			pathEnd1Messages.connectionMessages = append(pathEnd1Messages.connectionMessages, connectionIBCMessage{
				eventType: m.Initial.EventType,
				info:      m.Initial.Info,
			})
		} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
			pathEnd2Messages.connectionMessages = append(pathEnd2Messages.connectionMessages, connectionIBCMessage{
				eventType: m.Initial.EventType,
				info:      m.Initial.Info,
			})
		}
	case *ChannelMessageLifecycle:
		if m.Initial == nil {
			return
		}
		if !pp.IsRelevantConnection(m.Initial.ChainID, m.Initial.Info.ConnID) {
			return
		}
		if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
			pathEnd1Messages.channelMessages = append(pathEnd1Messages.channelMessages, channelIBCMessage{
				eventType: m.Initial.EventType,
				info:      m.Initial.Info,
			})
		} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
			pathEnd2Messages.channelMessages = append(pathEnd2Messages.channelMessages, channelIBCMessage{
				eventType: m.Initial.EventType,
				info:      m.Initial.Info,
			})
		}
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
		SrcMsgConnectionOpenInit:    pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenConfirm],
	}
	pathEnd2ConnectionHandshakeMessages := pathEndConnectionHandshakeMessages{
		Src:                         pp.pathEnd2,
		Dst:                         pp.pathEnd1,
		SrcMsgConnectionOpenInit:    pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenConfirm],
	}
	pathEnd1ConnectionHandshakeRes := pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd1ConnectionHandshakeMessages)
	pathEnd2ConnectionHandshakeRes := pp.getUnrelayedConnectionHandshakeMessagesAndToDelete(pathEnd2ConnectionHandshakeMessages)

	pathEnd1ChannelHandshakeMessages := pathEndChannelHandshakeMessages{
		Src:                      pp.pathEnd1,
		Dst:                      pp.pathEnd2,
		SrcMsgChannelOpenInit:    pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenConfirm],
	}
	pathEnd2ChannelHandshakeMessages := pathEndChannelHandshakeMessages{
		Src:                      pp.pathEnd2,
		Dst:                      pp.pathEnd1,
		SrcMsgChannelOpenInit:    pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenConfirm],
	}
	pathEnd1ChannelHandshakeRes := pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd1ChannelHandshakeMessages)
	pathEnd2ChannelHandshakeRes := pp.getUnrelayedChannelHandshakeMessagesAndToDelete(pathEnd2ChannelHandshakeMessages)

	// process the packet flows for both path ends to determine what needs to be relayed
	pathEnd1ProcessRes := make([]pathEndPacketFlowResponse, len(channelPairs))
	pathEnd2ProcessRes := make([]pathEndPacketFlowResponse, len(channelPairs))

	for i, pair := range channelPairs {
		var pathEnd1ChannelCloseConfirm, pathEnd2ChannelCloseConfirm *provider.ChannelInfo

		if pathEnd1ChanCloseConfirmMsgs, ok := pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseConfirm]; ok {
			if pathEnd1ChannelCloseConfirmMsg, ok := pathEnd1ChanCloseConfirmMsgs[pair.pathEnd1ChannelKey]; ok {
				pathEnd1ChannelCloseConfirm = &pathEnd1ChannelCloseConfirmMsg
			}
		}

		if pathEnd2ChanCloseConfirmMsgs, ok := pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseConfirm]; ok {
			if pathEnd2ChannelCloseConfirmMsg, ok := pathEnd2ChanCloseConfirmMsgs[pair.pathEnd2ChannelKey]; ok {
				pathEnd2ChannelCloseConfirm = &pathEnd2ChannelCloseConfirmMsg
			}
		}

		// Append acks into recv packet info if present
		pathEnd1DstMsgRecvPacket := pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeRecvPacket]
		for seq, ackInfo := range pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeWriteAck] {
			if recvPacketInfo, ok := pathEnd1DstMsgRecvPacket[seq]; ok {
				recvPacketInfo.Ack = ackInfo.Ack
				pathEnd1DstMsgRecvPacket[seq] = recvPacketInfo
			}
		}

		pathEnd2DstMsgRecvPacket := pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeRecvPacket]
		for seq, ackInfo := range pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeWriteAck] {
			if recvPacketInfo, ok := pathEnd2DstMsgRecvPacket[seq]; ok {
				recvPacketInfo.Ack = ackInfo.Ack
				pathEnd2DstMsgRecvPacket[seq] = recvPacketInfo
			}
		}

		pathEnd1PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                       pp.pathEnd1,
			Dst:                       pp.pathEnd2,
			ChannelKey:                pair.pathEnd1ChannelKey,
			SrcMsgTransfer:            pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeSendPacket],
			DstMsgRecvPacket:          pathEnd1DstMsgRecvPacket,
			SrcMsgAcknowledgement:     pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeAcknowledgePacket],
			SrcMsgTimeout:             pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeTimeoutPacket],
			SrcMsgTimeoutOnClose:      pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeTimeoutPacketOnClose],
			DstMsgChannelCloseConfirm: pathEnd2ChannelCloseConfirm,
		}
		pathEnd2PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                       pp.pathEnd2,
			Dst:                       pp.pathEnd1,
			ChannelKey:                pair.pathEnd2ChannelKey,
			SrcMsgTransfer:            pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeSendPacket],
			DstMsgRecvPacket:          pathEnd2DstMsgRecvPacket,
			SrcMsgAcknowledgement:     pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeAcknowledgePacket],
			SrcMsgTimeout:             pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeTimeoutPacket],
			SrcMsgTimeoutOnClose:      pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeTimeoutPacketOnClose],
			DstMsgChannelCloseConfirm: pathEnd1ChannelCloseConfirm,
		}

		pathEnd1ProcessRes[i] = pp.getUnrelayedPacketsAndAcksAndToDelete(ctx, pathEnd1PacketFlowMessages)
		pathEnd2ProcessRes[i] = pp.getUnrelayedPacketsAndAcksAndToDelete(ctx, pathEnd2PacketFlowMessages)
	}

	// concatenate applicable messages for pathend
	pathEnd1ConnectionMessages, pathEnd2ConnectionMessages := pp.connectionMessagesToSend(pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes)
	pathEnd1ChannelMessages, pathEnd2ChannelMessages := pp.channelMessagesToSend(pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes)

	pathEnd1PacketMessages, pathEnd2PacketMessages, pathEnd1ChanCloseMessages, pathEnd2ChanCloseMessages := pp.packetMessagesToSend(channelPairs, pathEnd1ProcessRes, pathEnd2ProcessRes)
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd1ChanCloseMessages...)
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd2ChanCloseMessages...)

	pathEnd1Messages := pathEndMessages{
		connectionMessages: pathEnd1ConnectionMessages,
		channelMessages:    pathEnd1ChannelMessages,
		packetMessages:     pathEnd1PacketMessages,
	}

	pathEnd2Messages := pathEndMessages{
		connectionMessages: pathEnd2ConnectionMessages,
		channelMessages:    pathEnd2ChannelMessages,
		packetMessages:     pathEnd2PacketMessages,
	}

	pp.appendInitialMessageIfNecessary(messageLifecycle, &pathEnd1Messages, &pathEnd2Messages)

	// now assemble and send messages in parallel
	// if sending messages fails to one pathEnd, we don't need to halt sending to the other pathEnd.
	var eg errgroup.Group
	eg.Go(func() error {
		return pp.assembleAndSendMessages(ctx, pp.pathEnd2, pp.pathEnd1, pathEnd1Messages)
	})
	eg.Go(func() error {
		return pp.assembleAndSendMessages(ctx, pp.pathEnd1, pp.pathEnd2, pathEnd2Messages)
	})
	return eg.Wait()
}

func (pp *PathProcessor) assembleMessage(
	ctx context.Context,
	msg ibcMessage,
	src, dst *pathEndRuntime,
	om *outgoingMessages,
	i int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	var message provider.RelayerMessage
	var err error
	switch m := msg.(type) {
	case packetIBCMessage:
		message, err = pp.assemblePacketMessage(ctx, m, src, dst)
		om.pktMsgs[i] = packetMessageToTrack{
			msg:       m,
			assembled: err == nil,
		}
	case connectionIBCMessage:
		message, err = pp.assembleConnectionMessage(ctx, m, src, dst)
		om.connMsgs[i] = connectionMessageToTrack{
			msg:       m,
			assembled: err == nil,
		}
	case channelIBCMessage:
		message, err = pp.assembleChannelMessage(ctx, m, src, dst)
		om.chanMsgs[i] = channelMessageToTrack{
			msg:       m,
			assembled: err == nil,
		}
	}
	if err != nil {
		pp.log.Error("Error assembling channel message", zap.Error(err))
		return
	}
	om.Append(message)
}

func (pp *PathProcessor) assembleAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	messages pathEndMessages,
) error {
	var needsClientUpdate bool
	if len(messages.packetMessages) == 0 && len(messages.connectionMessages) == 0 && len(messages.channelMessages) == 0 {
		var consensusHeightTime time.Time
		if dst.clientState.ConsensusTime.IsZero() {
			h, err := src.chainProvider.QueryIBCHeader(ctx, int64(dst.clientState.ConsensusHeight.RevisionHeight))
			if err != nil {
				return fmt.Errorf("failed to get header height: %w", err)
			}
			consensusHeightTime = time.Unix(0, int64(h.ConsensusState().GetTimestamp()))
		} else {
			consensusHeightTime = dst.clientState.ConsensusTime
		}
		clientUpdateThresholdMs := pp.clientUpdateThresholdTime.Milliseconds()
		if (float64(dst.clientState.TrustingPeriod.Milliseconds())*2/3 < float64(time.Since(consensusHeightTime).Milliseconds())) ||
			(clientUpdateThresholdMs > 0 && time.Since(consensusHeightTime).Milliseconds() > clientUpdateThresholdMs) {
			needsClientUpdate = true
			pp.log.Info("Client close to expiration",
				zap.String("chain_id:", dst.info.ChainID),
				zap.String("client_id:", dst.info.ClientID),
				zap.Int64("trusting_period", dst.clientState.TrustingPeriod.Milliseconds()),
				zap.Int64("time_since_client_update", time.Since(consensusHeightTime).Milliseconds()),
				zap.Int64("client_threshold_time", pp.clientUpdateThresholdTime.Milliseconds()),
			)
		} else {
			return nil
		}
	}
	om := outgoingMessages{
		msgs: make(
			[]provider.RelayerMessage,
			0,
			len(messages.packetMessages)+len(messages.connectionMessages)+len(messages.channelMessages),
		),
		pktMsgs:  make([]packetMessageToTrack, len(messages.packetMessages)),
		connMsgs: make([]connectionMessageToTrack, len(messages.connectionMessages)),
		chanMsgs: make([]channelMessageToTrack, len(messages.channelMessages)),
	}
	msgUpdateClient, err := pp.assembleMsgUpdateClient(ctx, src, dst)
	if err != nil {
		return err
	}
	om.Append(msgUpdateClient)

	// Each assembleMessage call below will make a query on the source chain, so these operations can run in parallel.
	var wg sync.WaitGroup

	for i, msg := range messages.packetMessages {
		wg.Add(1)
		go pp.assembleMessage(ctx, msg, src, dst, &om, i, &wg)
	}

	for i, msg := range messages.connectionMessages {
		wg.Add(1)
		go pp.assembleMessage(ctx, msg, src, dst, &om, i, &wg)
	}

	for i, msg := range messages.channelMessages {
		wg.Add(1)
		go pp.assembleMessage(ctx, msg, src, dst, &om, i, &wg)
	}

	wg.Wait()

	if len(om.msgs) == 1 && !needsClientUpdate {
		// only msgUpdateClient, don't need to send
		return errors.New("all messages failed to assemble")
	}

	for _, m := range om.pktMsgs {
		dst.trackProcessingPacketMessage(m)
	}
	for _, m := range om.connMsgs {
		dst.trackProcessingConnectionMessage(m)
	}
	for _, m := range om.chanMsgs {
		dst.trackProcessingChannelMessage(m)
	}

	go pp.sendMessages(ctx, src, dst, &om, pp.memo)

	return nil
}

func (pp *PathProcessor) sendMessages(ctx context.Context, src, dst *pathEndRuntime, om *outgoingMessages, memo string) {
	ctx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	_, txSuccess, err := dst.chainProvider.SendMessages(ctx, om.msgs, pp.memo)
	if err != nil {
		if errors.Is(err, chantypes.ErrRedundantTx) {
			pp.log.Debug("Packet(s) already handled by another relayer",
				zap.String("src_chain_id", src.info.ChainID),
				zap.String("dst_chain_id", dst.info.ChainID),
				zap.String("src_client_id", src.info.ClientID),
				zap.String("dst_client_id", dst.info.ClientID),
				zap.Object("messages", om),
				zap.Error(err),
			)
			return
		}
		pp.log.Error("Error sending messages",
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Object("messages", om),
			zap.Error(err),
		)
		return
	}
	if !txSuccess {
		dst.log.Error("Error sending messages, transaction was not successful")
		return
	}

	if pp.metrics == nil {
		return
	}
	for _, m := range om.pktMsgs {
		var channel, port string
		if m.msg.eventType == chantypes.EventTypeRecvPacket {
			channel = m.msg.info.DestChannel
			port = m.msg.info.DestPort
		} else {
			channel = m.msg.info.SourceChannel
			port = m.msg.info.SourcePort
		}
		pp.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, m.msg.eventType)
	}
}

func (pp *PathProcessor) assemblePacketMessage(
	ctx context.Context,
	msg packetIBCMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var packetProof func(context.Context, provider.PacketInfo, uint64) (provider.PacketProof, error)
	var assembleMessage func(provider.PacketInfo, provider.PacketProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case chantypes.EventTypeRecvPacket:
		packetProof = src.chainProvider.PacketCommitment
		assembleMessage = dst.chainProvider.MsgRecvPacket
	case chantypes.EventTypeAcknowledgePacket:
		packetProof = src.chainProvider.PacketAcknowledgement
		assembleMessage = dst.chainProvider.MsgAcknowledgement
	case chantypes.EventTypeTimeoutPacket:
		if msg.info.ChannelOrder == chantypes.ORDERED.String() {
			packetProof = src.chainProvider.NextSeqRecv
		} else {
			packetProof = src.chainProvider.PacketReceipt
		}

		assembleMessage = dst.chainProvider.MsgTimeout
	case chantypes.EventTypeTimeoutPacketOnClose:
		if msg.info.ChannelOrder == chantypes.ORDERED.String() {
			packetProof = src.chainProvider.NextSeqRecv
		} else {
			packetProof = src.chainProvider.PacketReceipt
		}

		assembleMessage = dst.chainProvider.MsgTimeoutOnClose
	default:
		return nil, fmt.Errorf("unexepected packet message eventType for message assembly: %s", msg.eventType)
	}

	ctx, cancel := context.WithTimeout(ctx, packetProofQueryTimeout)
	defer cancel()

	var proof provider.PacketProof
	var err error
	proof, err = packetProof(ctx, msg.info, src.latestBlock.Height)
	if err != nil {
		return nil, fmt.Errorf("error querying packet proof: %w", err)
	}
	return assembleMessage(msg.info, proof)
}

func (pp *PathProcessor) assembleConnectionMessage(
	ctx context.Context,
	msg connectionIBCMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var connProof func(context.Context, provider.ConnectionInfo, uint64) (provider.ConnectionProof, error)
	var assembleMessage func(provider.ConnectionInfo, provider.ConnectionProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case conntypes.EventTypeConnectionOpenInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgConnectionOpenInit
	case conntypes.EventTypeConnectionOpenTry:
		connProof = src.chainProvider.ConnectionHandshakeProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenTry
	case conntypes.EventTypeConnectionOpenAck:
		connProof = src.chainProvider.ConnectionHandshakeProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenAck
	case conntypes.EventTypeConnectionOpenConfirm:
		connProof = src.chainProvider.ConnectionProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenConfirm
	default:
		return nil, fmt.Errorf("unexepected connection message eventType for message assembly: %s", msg.eventType)
	}
	var proof provider.ConnectionProof
	var err error
	if connProof != nil {
		proof, err = connProof(ctx, msg.info, src.latestBlock.Height)
		if err != nil {
			return nil, fmt.Errorf("error querying connection proof: %w", err)
		}
	}
	return assembleMessage(msg.info, proof)
}

func (pp *PathProcessor) assembleChannelMessage(
	ctx context.Context,
	msg channelIBCMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var chanProof func(context.Context, provider.ChannelInfo, uint64) (provider.ChannelProof, error)
	var assembleMessage func(provider.ChannelInfo, provider.ChannelProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case chantypes.EventTypeChannelOpenInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgChannelOpenInit
	case chantypes.EventTypeChannelOpenTry:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenTry
	case chantypes.EventTypeChannelOpenAck:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenAck
	case chantypes.EventTypeChannelOpenConfirm:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenConfirm
	case chantypes.EventTypeChannelCloseInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgChannelCloseInit
	case chantypes.EventTypeChannelCloseConfirm:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelCloseConfirm
	default:
		return nil, fmt.Errorf("unexepected channel message eventType for message assembly: %s", msg.eventType)
	}
	var proof provider.ChannelProof
	var err error
	if chanProof != nil {
		proof, err = chanProof(ctx, msg.info, src.latestBlock.Height)
		if err != nil {
			return nil, fmt.Errorf("error querying channel proof: %w", err)
		}
	}
	return assembleMessage(msg.info, proof)
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
	pp.pathEnd1.channelProcessing.deleteMessages(pathEnd1ChannelHandshakeRes.ToDeleteSrc, pathEnd2ChannelHandshakeRes.ToDeleteDst)
	pp.pathEnd2.channelProcessing.deleteMessages(pathEnd2ChannelHandshakeRes.ToDeleteSrc, pathEnd1ChannelHandshakeRes.ToDeleteDst)

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
	pp.pathEnd1.connProcessing.deleteMessages(pathEnd1ConnectionHandshakeRes.ToDeleteSrc, pathEnd2ConnectionHandshakeRes.ToDeleteDst)
	pp.pathEnd2.connProcessing.deleteMessages(pathEnd2ConnectionHandshakeRes.ToDeleteSrc, pathEnd1ConnectionHandshakeRes.ToDeleteDst)

	return pathEnd1ConnectionMessages, pathEnd2ConnectionMessages
}

func (pp *PathProcessor) packetMessagesToSend(
	channelPairs []channelPair,
	pathEnd1ProcessRes []pathEndPacketFlowResponse,
	pathEnd2ProcessRes []pathEndPacketFlowResponse,
) ([]packetIBCMessage, []packetIBCMessage, []channelIBCMessage, []channelIBCMessage) {
	pathEnd1PacketLen := 0
	pathEnd2PacketLen := 0
	pathEnd1ChannelLen := 0
	pathEnd2ChannelLen := 0

	for i := 0; i < len(channelPairs); i++ {
		pathEnd1PacketLen += len(pathEnd2ProcessRes[i].DstMessages) + len(pathEnd1ProcessRes[i].SrcMessages)
		pathEnd2PacketLen += len(pathEnd1ProcessRes[i].DstMessages) + len(pathEnd2ProcessRes[i].SrcMessages)
		pathEnd1ChannelLen += len(pathEnd2ProcessRes[i].DstChannelMessage)
		pathEnd2ChannelLen += len(pathEnd1ProcessRes[i].DstChannelMessage)
	}

	pathEnd1PacketMessages := make([]packetIBCMessage, 0, pathEnd1PacketLen)
	pathEnd2PacketMessages := make([]packetIBCMessage, 0, pathEnd2PacketLen)

	pathEnd1ChannelMessage := make([]channelIBCMessage, 0, pathEnd1ChannelLen)
	pathEnd2ChannelMessage := make([]channelIBCMessage, 0, pathEnd2ChannelLen)

	for i, channelPair := range channelPairs {
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd2ProcessRes[i].DstMessages...)
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd1ProcessRes[i].SrcMessages...)

		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd1ProcessRes[i].DstMessages...)
		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd2ProcessRes[i].SrcMessages...)

		pathEnd1ChannelMessage = append(pathEnd1ChannelMessage, pathEnd2ProcessRes[i].DstChannelMessage...)
		pathEnd2ChannelMessage = append(pathEnd2ChannelMessage, pathEnd1ProcessRes[i].DstChannelMessage...)

		pp.pathEnd1.messageCache.ChannelHandshake.DeleteMessages(pathEnd2ProcessRes[i].ToDeleteDstChannel)
		pp.pathEnd1.channelProcessing.deleteMessages(pathEnd2ProcessRes[i].ToDeleteDstChannel)

		pp.pathEnd2.messageCache.ChannelHandshake.DeleteMessages(pathEnd1ProcessRes[i].ToDeleteDstChannel)
		pp.pathEnd2.channelProcessing.deleteMessages(pathEnd1ProcessRes[i].ToDeleteDstChannel)

		pp.pathEnd1.messageCache.PacketFlow[channelPair.pathEnd1ChannelKey].DeleteMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.messageCache.PacketFlow[channelPair.pathEnd2ChannelKey].DeleteMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)

		pp.pathEnd1.packetProcessing[channelPair.pathEnd1ChannelKey].deleteMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.packetProcessing[channelPair.pathEnd2ChannelKey].deleteMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)
	}

	return pathEnd1PacketMessages, pathEnd2PacketMessages, pathEnd1ChannelMessage, pathEnd2ChannelMessage
}
