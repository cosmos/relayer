package processor

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"

	"github.com/cosmos/relayer/v2/relayer/common"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// getMessagesToSend returns only the lowest sequence message (if it should be sent) for ordered channels,
// otherwise returns all which should be sent.
func (pp *PathProcessor) getMessagesToSend(
	msgs []packetIBCMessage,
	src, dst *pathEndRuntime,
) (srcMsgs []packetIBCMessage, dstMsgs []packetIBCMessage) {
	if len(msgs) == 0 {
		return
	}
	if msgs[0].info.ChannelOrder == chantypes.ORDERED.String() {
		// for packet messages on ordered channels, only handle the lowest sequence number now.
		sort.SliceStable(msgs, func(i, j int) bool {
			return msgs[i].info.Sequence < msgs[j].info.Sequence
		})
		firstMsg := msgs[0]
		switch firstMsg.eventType {
		case chantypes.EventTypeRecvPacket:
			if dst.shouldSendPacketMessage(firstMsg, src) {
				dstMsgs = append(dstMsgs, firstMsg)
			}
		case common.EventTimeoutRequest:
			if dst.shouldSendPacketMessage(firstMsg, src) {
				dstMsgs = append(dstMsgs, firstMsg)
			}
		default:
			if src.shouldSendPacketMessage(firstMsg, dst) {
				srcMsgs = append(srcMsgs, firstMsg)
			}
		}
		return srcMsgs, dstMsgs
	}

	// for unordered channels, can handle multiple simultaneous packets.
	for _, msg := range msgs {
		switch msg.eventType {
		case chantypes.EventTypeRecvPacket:
			if dst.shouldSendPacketMessage(msg, src) {
				dstMsgs = append(dstMsgs, msg)
			}
		case common.EventTimeoutRequest:
			if dst.shouldSendPacketMessage(msg, src) {
				dstMsgs = append(dstMsgs, msg)
			}
		default:
			if src.shouldSendPacketMessage(msg, dst) {
				srcMsgs = append(srcMsgs, msg)
			}
		}
	}
	return srcMsgs, dstMsgs
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(ctx context.Context, pathEndPacketFlowMessages pathEndPacketFlowMessages) pathEndPacketFlowResponse {
	res := pathEndPacketFlowResponse{
		ToDeleteSrc:        make(map[string][]uint64),
		ToDeleteDst:        make(map[string][]uint64),
		ToDeleteDstChannel: make(map[string][]ChannelKey),
	}

	var msgs []packetIBCMessage

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
						res.ToDeleteDst[common.EventTimeoutRequest] = append(res.ToDeleteDst[common.EventTimeoutRequest], transferSeq)

					}
				} else {
					// unordered channel, and we have a timeout for this packet, so packet flow is complete
					// remove all retention of this sequence number
					res.ToDeleteSrc[chantypes.EventTypeSendPacket] = append(res.ToDeleteSrc[chantypes.EventTypeSendPacket], transferSeq)
					res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket] = append(res.ToDeleteSrc[chantypes.EventTypeTimeoutPacket], transferSeq)
					res.ToDeleteDst[common.EventTimeoutRequest] = append(res.ToDeleteDst[common.EventTimeoutRequest], transferSeq)

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

		for requestTimeoutSeq, msgTimeoutRequest := range pathEndPacketFlowMessages.DstMsgRequestTimeout {
			if transferSeq == requestTimeoutSeq {
				timeoutMsg := packetIBCMessage{
					eventType: chantypes.EventTypeTimeoutPacket,
					info:      msgTimeoutRequest,
				}
				msgs = append(msgs, timeoutMsg)
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
				msgs = append(msgs, ackMsg)
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay either MsgRecvPacket from src to dst, or MsgTimeout/MsgTimeoutOnClose from dst to src
		if err := pathEndPacketFlowMessages.Dst.chainProvider.ValidatePacket(msgTransfer, pathEndPacketFlowMessages.Dst.latestBlock); err != nil {
			var timeoutHeightErr *provider.TimeoutHeightError
			var timeoutTimestampErr *provider.TimeoutTimestampError
			var timeoutOnCloseErr *provider.TimeoutOnCloseError

			if pathEndPacketFlowMessages.Dst.chainProvider.Type() == common.IconModule {
				switch {
				case errors.As(err, &timeoutHeightErr) || errors.As(err, &timeoutTimestampErr):
					timeoutRequestMsg := packetIBCMessage{
						eventType: common.EventTimeoutRequest,
						info:      msgTransfer,
					}
					msgs = append(msgs, timeoutRequestMsg)

				default:
					pp.log.Error("Packet is invalid",
						zap.String("chain_id", pathEndPacketFlowMessages.Src.info.ChainID),
						zap.Error(err),
					)
				}
				continue MsgTransferLoop

			}

			switch {
			case errors.As(err, &timeoutHeightErr) || errors.As(err, &timeoutTimestampErr):
				timeoutMsg := packetIBCMessage{
					eventType: chantypes.EventTypeTimeoutPacket,
					info:      msgTransfer,
				}
				msgs = append(msgs, timeoutMsg)
			case errors.As(err, &timeoutOnCloseErr):
				timeoutOnCloseMsg := packetIBCMessage{
					eventType: chantypes.EventTypeTimeoutPacketOnClose,
					info:      msgTransfer,
				}
				msgs = append(msgs, timeoutOnCloseMsg)
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
		msgs = append(msgs, recvPacketMsg)
	}

	res.SrcMessages, res.DstMessages = pp.getMessagesToSend(msgs, pathEndPacketFlowMessages.Src, pathEndPacketFlowMessages.Dst)

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
			res.ToDeleteDst[common.EventTimeoutRequest] = append(res.ToDeleteDst[common.EventTimeoutRequest], timeoutSeq)

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

func (pp *PathProcessor) getUnrelayedClientICQMessages(pathEnd *pathEndRuntime, queryMessages, responseMessages ClientICQMessageCache) (res []clientICQMessage) {
ClientICQLoop:
	for queryID, queryMsg := range queryMessages {
		for resQueryID := range responseMessages {
			if queryID == resQueryID {
				// done with this query, remove all retention.
				pathEnd.messageCache.ClientICQ.DeleteMessages(queryID)
				delete(pathEnd.clientICQProcessing, queryID)
				continue ClientICQLoop
			}
		}
		// query ID not found in response messages, check if should send queryMsg and send
		if pathEnd.shouldSendClientICQMessage(queryMsg) {
			res = append(res, clientICQMessage{
				info: queryMsg,
			})
		}
	}

	// now iterate through completion message and remove any leftover messages.
	for queryID := range responseMessages {
		pathEnd.messageCache.ClientICQ.DeleteMessages(queryID)
		delete(pathEnd.clientICQProcessing, queryID)
	}
	return res
}

//

// updateClientTrustedState combines the counterparty chains trusted IBC header
// with the latest client state, which will be used for constructing MsgUpdateClient messages.
func (pp *PathProcessor) updateClientTrustedState(src *pathEndRuntime, dst *pathEndRuntime) {
	if src.clientTrustedState.ClientState.ConsensusHeight.GTE(src.clientState.ConsensusHeight) {
		// current height already trusted
		return
	}

	if ClientIsIcon(src.clientState) {
		ibcheader, ok := nextIconIBCHeader(dst.ibcHeaderCache.Clone(), src.clientState.ConsensusHeight.RevisionHeight)
		if !ok {
			pp.log.Debug("No cached IBC header found for client next trusted height",
				zap.String("chain_id", src.info.ChainID),
				zap.String("client_id", src.info.ClientID),
				zap.Uint64("height", src.clientState.ConsensusHeight.RevisionHeight),
			)
			return

		}

		src.clientTrustedState = provider.ClientTrustedState{
			ClientState: src.clientState,
			IBCHeader:   ibcheader,
		}
		return
	}

	ibcHeader, ok := dst.ibcHeaderCache[src.clientState.ConsensusHeight.RevisionHeight+1]
	if !ok {
		if ibcHeaderCurrent, ok := dst.ibcHeaderCache[src.clientState.ConsensusHeight.RevisionHeight]; ok {
			if dst.clientTrustedState.IBCHeader != nil &&
				bytes.Equal(src.clientTrustedState.IBCHeader.NextValidatorsHash(), ibcHeaderCurrent.NextValidatorsHash()) {
				src.clientTrustedState = provider.ClientTrustedState{
					ClientState: src.clientState,
					IBCHeader:   ibcHeaderCurrent,
				}

				return
			}
		}
		pp.log.Debug("No cached IBC header for client trusted height",
			zap.String("chain_id", src.info.ChainID),
			zap.String("client_id", src.info.ClientID),
			zap.Uint64("height", src.clientState.ConsensusHeight.RevisionHeight),
		)
		return
	}

	src.clientTrustedState = provider.ClientTrustedState{
		ClientState: src.clientState,
		IBCHeader:   ibcHeader,
	}
}

func (pp *PathProcessor) appendInitialMessageIfNecessary(pathEnd1Messages, pathEnd2Messages *pathEndMessages) {
	if pp.messageLifecycle == nil || pp.sentInitialMsg {
		return
	}

	pp.sentInitialMsg = true
	switch m := pp.messageLifecycle.(type) {
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
func (pp *PathProcessor) processLatestMessages(ctx context.Context) error {
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
			DstMsgRequestTimeout:      pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][common.EventTimeoutRequest],
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
			DstMsgRequestTimeout:      pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][common.EventTimeoutRequest],
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

	pathEnd1ClientICQMessages := pp.getUnrelayedClientICQMessages(
		pp.pathEnd1,
		pp.pathEnd1.messageCache.ClientICQ[ClientICQTypeRequest],
		pp.pathEnd1.messageCache.ClientICQ[ClientICQTypeResponse],
	)
	pathEnd2ClientICQMessages := pp.getUnrelayedClientICQMessages(
		pp.pathEnd2,
		pp.pathEnd2.messageCache.ClientICQ[ClientICQTypeRequest],
		pp.pathEnd2.messageCache.ClientICQ[ClientICQTypeResponse],
	)

	pathEnd1Messages := pathEndMessages{
		connectionMessages: pathEnd1ConnectionMessages,
		channelMessages:    pathEnd1ChannelMessages,
		packetMessages:     pathEnd1PacketMessages,
		clientICQMessages:  pathEnd1ClientICQMessages,
	}

	pathEnd2Messages := pathEndMessages{
		connectionMessages: pathEnd2ConnectionMessages,
		channelMessages:    pathEnd2ChannelMessages,
		packetMessages:     pathEnd2PacketMessages,
		clientICQMessages:  pathEnd2ClientICQMessages,
	}

	pp.appendInitialMessageIfNecessary(&pathEnd1Messages, &pathEnd2Messages)

	// now assemble and send messages in parallel
	// if sending messages fails to one pathEnd, we don't need to halt sending to the other pathEnd.
	var eg errgroup.Group
	eg.Go(func() error {
		mp := newMessageProcessor(pp.log, pp.metrics, pp.memo, pp.clientUpdateThresholdTime)
		return mp.processMessages(ctx, pathEnd1Messages, pp.pathEnd2, pp.pathEnd1)
	})
	eg.Go(func() error {
		mp := newMessageProcessor(pp.log, pp.metrics, pp.memo, pp.clientUpdateThresholdTime)
		return mp.processMessages(ctx, pathEnd2Messages, pp.pathEnd1, pp.pathEnd2)
	})
	return eg.Wait()
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

func queryPacketCommitments(
	ctx context.Context,
	pathEnd *pathEndRuntime,
	k ChannelKey,
	commitments map[ChannelKey][]uint64,
	mu *sync.Mutex,
) func() error {
	return func() error {
		pathEnd.log.Debug("Flushing", zap.String("channel", k.ChannelID), zap.String("port", k.PortID))

		c, err := pathEnd.chainProvider.QueryPacketCommitments(ctx, pathEnd.latestBlock.Height, k.ChannelID, k.PortID)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		commitments[k] = make([]uint64, len(c.Commitments))
		for i, p := range c.Commitments {
			commitments[k][i] = p.Sequence
		}
		return nil
	}
}

func queuePendingRecvAndAcks(
	ctx context.Context,
	src, dst *pathEndRuntime,
	k ChannelKey,
	seqs []uint64,
	srcCache ChannelPacketMessagesCache,
	dstCache ChannelPacketMessagesCache,
	srcMu *sync.Mutex,
	dstMu *sync.Mutex,
) func() error {
	return func() error {
		if len(seqs) == 0 {
			src.log.Debug("Nothing to flush", zap.String("channel", k.ChannelID), zap.String("port", k.PortID))
			return nil
		}

		unrecv, err := dst.chainProvider.QueryUnreceivedPackets(ctx, dst.latestBlock.Height, k.CounterpartyChannelID, k.CounterpartyPortID, seqs)
		if err != nil {
			return err
		}

		if len(unrecv) > 0 {
			src.log.Debug("Will flush MsgRecvPacket", zap.String("channel", k.ChannelID), zap.String("port", k.PortID), zap.Uint64s("sequences", unrecv))
		} else {
			src.log.Debug("No MsgRecvPacket to flush", zap.String("channel", k.ChannelID), zap.String("port", k.PortID))
		}

		for _, seq := range unrecv {
			sendPacket, err := src.chainProvider.QuerySendPacket(ctx, k.ChannelID, k.PortID, seq)
			if err != nil {
				return err
			}
			srcMu.Lock()
			if _, ok := srcCache[k]; !ok {
				srcCache[k] = make(PacketMessagesCache)
			}
			if _, ok := srcCache[k][chantypes.EventTypeSendPacket]; !ok {
				srcCache[k][chantypes.EventTypeSendPacket] = make(PacketSequenceCache)
			}
			srcCache[k][chantypes.EventTypeSendPacket][seq] = sendPacket
			srcMu.Unlock()
		}

		var unacked []uint64

	SeqLoop:
		for _, seq := range seqs {
			for _, unrecvSeq := range unrecv {
				if seq == unrecvSeq {
					continue SeqLoop
				}
			}
			// does not exist in unrecv, so this is an ack that must be written
			unacked = append(unacked, seq)
		}

		if len(unacked) > 0 {
			src.log.Debug("Will flush MsgAcknowledgement", zap.String("channel", k.ChannelID), zap.String("port", k.PortID), zap.Uint64s("sequences", unrecv))
		} else {
			src.log.Debug("No MsgAcknowledgement to flush", zap.String("channel", k.ChannelID), zap.String("port", k.PortID))
		}

		for _, seq := range unacked {
			recvPacket, err := dst.chainProvider.QueryRecvPacket(ctx, k.CounterpartyChannelID, k.CounterpartyPortID, seq)
			if err != nil {
				return err
			}
			srcMu.Lock()
			if _, ok := srcCache[k]; !ok {
				srcCache[k] = make(PacketMessagesCache)
			}
			if _, ok := srcCache[k][chantypes.EventTypeSendPacket]; !ok {
				srcCache[k][chantypes.EventTypeSendPacket] = make(PacketSequenceCache)
			}
			srcCache[k][chantypes.EventTypeSendPacket][seq] = recvPacket
			srcMu.Unlock()

			dstMu.Lock()
			if _, ok := dstCache[k]; !ok {
				dstCache[k] = make(PacketMessagesCache)
			}
			if _, ok := dstCache[k][chantypes.EventTypeRecvPacket]; !ok {
				dstCache[k][chantypes.EventTypeRecvPacket] = make(PacketSequenceCache)
			}
			if _, ok := dstCache[k][chantypes.EventTypeWriteAck]; !ok {
				dstCache[k][chantypes.EventTypeWriteAck] = make(PacketSequenceCache)
			}
			dstCache[k][chantypes.EventTypeRecvPacket][seq] = recvPacket
			dstCache[k][chantypes.EventTypeWriteAck][seq] = recvPacket
			dstMu.Unlock()
		}
		return nil
	}
}

// flush runs queries to relay any pending messages which may have been
// in blocks before the height that the chain processors started querying.
func (pp *PathProcessor) flush(ctx context.Context) {
	var (
		commitments1                   = make(map[ChannelKey][]uint64)
		commitments2                   = make(map[ChannelKey][]uint64)
		commitments1Mu, commitments2Mu sync.Mutex

		pathEnd1Cache                    = NewIBCMessagesCache()
		pathEnd2Cache                    = NewIBCMessagesCache()
		pathEnd1CacheMu, pathEnd2CacheMu sync.Mutex
	)

	// Query remaining packet commitments on both chains
	var eg errgroup.Group
	for k := range pp.pathEnd1.channelStateCache {
		eg.Go(queryPacketCommitments(ctx, pp.pathEnd1, k, commitments1, &commitments1Mu))
	}
	for k := range pp.pathEnd2.channelStateCache {
		eg.Go(queryPacketCommitments(ctx, pp.pathEnd2, k, commitments2, &commitments2Mu))
	}

	if err := eg.Wait(); err != nil {
		pp.log.Error("Failed to query packet commitments", zap.Error(err))
	}

	// From remaining packet commitments, determine if:
	// 1. Packet commitment is on source, but MsgRecvPacket has not yet been relayed to destination
	// 2. Packet commitment is on source, and MsgRecvPacket has been relayed to destination, but MsgAcknowledgement has not been written to source to clear the packet commitment.
	// Based on above conditions, enqueue MsgRecvPacket and MsgAcknowledgement messages
	for k, seqs := range commitments1 {
		eg.Go(queuePendingRecvAndAcks(ctx, pp.pathEnd1, pp.pathEnd2, k, seqs, pathEnd1Cache.PacketFlow, pathEnd2Cache.PacketFlow, &pathEnd1CacheMu, &pathEnd2CacheMu))
	}

	for k, seqs := range commitments2 {
		eg.Go(queuePendingRecvAndAcks(ctx, pp.pathEnd2, pp.pathEnd1, k, seqs, pathEnd2Cache.PacketFlow, pathEnd1Cache.PacketFlow, &pathEnd2CacheMu, &pathEnd1CacheMu))
	}

	if err := eg.Wait(); err != nil {
		pp.log.Error("Failed to enqueue pending messages for flush", zap.Error(err))
	}

	pp.pathEnd1.mergeMessageCache(pathEnd1Cache, pp.pathEnd2.info.ChainID, pp.pathEnd2.inSync)
	pp.pathEnd2.mergeMessageCache(pathEnd2Cache, pp.pathEnd1.info.ChainID, pp.pathEnd1.inSync)
}

// shouldTerminateForFlushComplete will determine if the relayer should exit
// when FlushLifecycle is used. It will exit when all of the message caches are cleared.
func (pp *PathProcessor) shouldTerminateForFlushComplete(
	ctx context.Context, cancel func(),
) bool {
	if _, ok := pp.messageLifecycle.(*FlushLifecycle); !ok {
		return false
	}
	for _, packetMessagesCache := range pp.pathEnd1.messageCache.PacketFlow {
		for _, c := range packetMessagesCache {
			if len(c) > 0 {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd1.messageCache.ChannelHandshake {
		if len(c) > 0 {
			return false
		}
	}
	for _, c := range pp.pathEnd1.messageCache.ConnectionHandshake {
		if len(c) > 0 {
			return false
		}
	}
	for _, packetMessagesCache := range pp.pathEnd2.messageCache.PacketFlow {
		for _, c := range packetMessagesCache {
			if len(c) > 0 {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd2.messageCache.ChannelHandshake {
		if len(c) > 0 {
			return false
		}
	}
	for _, c := range pp.pathEnd2.messageCache.ConnectionHandshake {
		if len(c) > 0 {
			return false
		}
	}
	pp.log.Info("Found termination condition for flush, all caches cleared")
	return true
}
