package processor

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// preInitKey is used to declare intent to initialize a connection or channel handshake
// i.e. a MsgConnectionOpenInit or a MsgChannelOpenInit should be broadcasted to start
// the handshake if this key exists in the relevant cache.
const (
	preInitKey  = "pre_init"
	preCloseKey = "pre_close"
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
		default:
			if src.shouldSendPacketMessage(msg, dst) {
				srcMsgs = append(srcMsgs, msg)
			}
		}
	}
	return srcMsgs, dstMsgs
}

func (pp *PathProcessor) unrelayedPacketFlowMessages(
	ctx context.Context,
	pathEndPacketFlowMessages pathEndPacketFlowMessages,
) pathEndPacketFlowResponse {
	var (
		res                pathEndPacketFlowResponse
		msgs               []packetIBCMessage
		toDeleteSrc        = make(map[string][]uint64)
		toDeleteDst        = make(map[string][]uint64)
		toDeleteDstChannel = make(map[string][]ChannelKey)
	)

	k := pathEndPacketFlowMessages.ChannelKey

	deletePreInitIfMatches := func(info provider.PacketInfo) {
		cachedInfo, ok := pathEndPacketFlowMessages.SrcPreTransfer[0]
		if !ok {
			return
		}
		if !bytes.Equal(cachedInfo.Data, info.Data) {
			return
		}
		toDeleteSrc[preInitKey] = []uint64{0}
	}

	processRemovals := func() {
		pathEndPacketFlowMessages.Src.messageCache.PacketFlow[k].DeleteMessages(toDeleteSrc)
		pathEndPacketFlowMessages.Dst.messageCache.PacketFlow[k.Counterparty()].DeleteMessages(toDeleteDst)
		pathEndPacketFlowMessages.Dst.messageCache.ChannelHandshake.DeleteMessages(toDeleteDstChannel)
		pathEndPacketFlowMessages.Src.packetProcessing[k].deleteMessages(toDeleteSrc)
		pathEndPacketFlowMessages.Dst.packetProcessing[k.Counterparty()].deleteMessages(toDeleteDst)
		pathEndPacketFlowMessages.Dst.channelProcessing.deleteMessages(toDeleteDstChannel)
		toDeleteSrc = make(map[string][]uint64)
		toDeleteDst = make(map[string][]uint64)
		toDeleteDstChannel = make(map[string][]ChannelKey)
	}

	for seq, info := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
		// we have observed an ack on chain for this packet, so packet flow is complete
		// remove all retention of this sequence number
		deletePreInitIfMatches(info)
		toDeleteSrc[chantypes.EventTypeSendPacket] = append(toDeleteSrc[chantypes.EventTypeSendPacket], seq)
		toDeleteDst[chantypes.EventTypeRecvPacket] = append(toDeleteDst[chantypes.EventTypeRecvPacket], seq)
		toDeleteDst[chantypes.EventTypeWriteAck] = append(toDeleteDst[chantypes.EventTypeWriteAck], seq)
		toDeleteSrc[chantypes.EventTypeAcknowledgePacket] = append(toDeleteSrc[chantypes.EventTypeAcknowledgePacket], seq)
	}

	for seq, info := range pathEndPacketFlowMessages.SrcMsgTimeoutOnClose {
		// we have observed a timeout-on-close on chain for this packet, so packet flow is complete
		// remove all retention of this sequence number
		deletePreInitIfMatches(info)
		toDeleteSrc[chantypes.EventTypeSendPacket] = append(toDeleteSrc[chantypes.EventTypeSendPacket], seq)
		toDeleteDst[chantypes.EventTypeRecvPacket] = append(toDeleteDst[chantypes.EventTypeRecvPacket], seq)
		toDeleteDst[chantypes.EventTypeWriteAck] = append(toDeleteDst[chantypes.EventTypeWriteAck], seq)
		toDeleteSrc[chantypes.EventTypeAcknowledgePacket] = append(toDeleteSrc[chantypes.EventTypeAcknowledgePacket], seq)
	}

	for seq, info := range pathEndPacketFlowMessages.SrcMsgTimeout {
		deletePreInitIfMatches(info)
		toDeleteSrc[chantypes.EventTypeSendPacket] = append(toDeleteSrc[chantypes.EventTypeSendPacket], seq)
		toDeleteSrc[chantypes.EventTypeTimeoutPacket] = append(toDeleteSrc[chantypes.EventTypeTimeoutPacket], seq)
		if info.ChannelOrder == chantypes.ORDERED.String() {
			// Channel is now closed on src.
			// enqueue channel close init observation to be handled by channel close correlation
			if _, ok := pathEndPacketFlowMessages.Src.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit]; !ok {
				pathEndPacketFlowMessages.Src.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit] = make(ChannelMessageCache)
			}
			pathEndPacketFlowMessages.Src.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit][k] = provider.ChannelInfo{
				Height:                info.Height,
				PortID:                info.SourcePort,
				ChannelID:             info.SourceChannel,
				CounterpartyPortID:    info.DestPort,
				CounterpartyChannelID: info.DestChannel,
				Order:                 orderFromString(info.ChannelOrder),
			}
		}
	}

	processRemovals()

	for seq, info := range pathEndPacketFlowMessages.DstMsgRecvPacket {
		deletePreInitIfMatches(info)
		toDeleteSrc[chantypes.EventTypeSendPacket] = append(toDeleteSrc[chantypes.EventTypeSendPacket], seq)

		if len(info.Ack) == 0 {
			// have recv_packet but not write_acknowledgement yet. skip for now.
			continue
		}
		// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
		ackMsg := packetIBCMessage{
			eventType: chantypes.EventTypeAcknowledgePacket,
			info:      info,
		}
		msgs = append(msgs, ackMsg)
	}

	processRemovals()

	for _, info := range pathEndPacketFlowMessages.SrcMsgTransfer {
		deletePreInitIfMatches(info)

		// Packet is not yet relayed! need to relay either MsgRecvPacket from src to dst, or MsgTimeout/MsgTimeoutOnClose from dst to src
		if err := pathEndPacketFlowMessages.Dst.chainProvider.ValidatePacket(info, pathEndPacketFlowMessages.Dst.latestBlock); err != nil {
			var timeoutHeightErr *provider.TimeoutHeightError
			var timeoutTimestampErr *provider.TimeoutTimestampError
			var timeoutOnCloseErr *provider.TimeoutOnCloseError

			switch {
			case errors.As(err, &timeoutHeightErr) || errors.As(err, &timeoutTimestampErr):
				timeoutMsg := packetIBCMessage{
					eventType: chantypes.EventTypeTimeoutPacket,
					info:      info,
				}
				msgs = append(msgs, timeoutMsg)
			case errors.As(err, &timeoutOnCloseErr):
				timeoutOnCloseMsg := packetIBCMessage{
					eventType: chantypes.EventTypeTimeoutPacketOnClose,
					info:      info,
				}
				msgs = append(msgs, timeoutOnCloseMsg)
			default:
				pp.log.Error("Packet is invalid",
					zap.String("chain_id", pathEndPacketFlowMessages.Src.info.ChainID),
					zap.Error(err),
				)
			}
			continue
		}
		recvPacketMsg := packetIBCMessage{
			eventType: chantypes.EventTypeRecvPacket,
			info:      info,
		}
		msgs = append(msgs, recvPacketMsg)
	}

	processRemovals()

	for _, info := range pathEndPacketFlowMessages.SrcPreTransfer {
		msgTransfer := packetIBCMessage{
			eventType: chantypes.EventTypeSendPacket,
			info:      info,
		}
		msgs = append(msgs, msgTransfer)
	}

	res.SrcMessages, res.DstMessages = pp.getMessagesToSend(msgs, pathEndPacketFlowMessages.Src, pathEndPacketFlowMessages.Dst)

	return res
}

func (pp *PathProcessor) unrelayedConnectionHandshakeMessages(
	pathEndConnectionHandshakeMessages pathEndConnectionHandshakeMessages,
) pathEndConnectionHandshakeResponse {
	var (
		res         pathEndConnectionHandshakeResponse
		toDeleteSrc = make(map[string][]ConnectionKey)
		toDeleteDst = make(map[string][]ConnectionKey)
	)

	processRemovals := func() {
		pathEndConnectionHandshakeMessages.Src.messageCache.ConnectionHandshake.DeleteMessages(toDeleteSrc)
		pathEndConnectionHandshakeMessages.Dst.messageCache.ConnectionHandshake.DeleteMessages(toDeleteDst)
		pathEndConnectionHandshakeMessages.Src.connProcessing.deleteMessages(toDeleteSrc)
		pathEndConnectionHandshakeMessages.Dst.connProcessing.deleteMessages(toDeleteDst)
		toDeleteSrc = make(map[string][]ConnectionKey)
		toDeleteDst = make(map[string][]ConnectionKey)
	}

	for connKey := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenConfirm {
		// found open confirm, channel handshake complete. remove all retention

		counterpartyKey := connKey.Counterparty()
		toDeleteDst[conntypes.EventTypeConnectionOpenConfirm] = append(
			toDeleteDst[conntypes.EventTypeConnectionOpenConfirm],
			connKey,
		)
		toDeleteSrc[conntypes.EventTypeConnectionOpenAck] = append(
			toDeleteSrc[conntypes.EventTypeConnectionOpenAck],
			counterpartyKey,
		)
		toDeleteDst[conntypes.EventTypeConnectionOpenTry] = append(
			toDeleteDst[conntypes.EventTypeConnectionOpenTry],
			connKey,
		)

		// MsgConnectionOpenInit does not have CounterpartyConnectionID
		toDeleteSrc[conntypes.EventTypeConnectionOpenInit] = append(
			toDeleteSrc[conntypes.EventTypeConnectionOpenInit],
			counterpartyKey.MsgInitKey(),
		)
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], counterpartyKey.PreInitKey())
	}

	processRemovals()

	for connKey, info := range pathEndConnectionHandshakeMessages.SrcMsgConnectionOpenAck {
		// need to send an open confirm to dst
		msgOpenConfirm := connectionIBCMessage{
			eventType: conntypes.EventTypeConnectionOpenConfirm,
			info:      info,
		}

		if pathEndConnectionHandshakeMessages.Dst.shouldSendConnectionMessage(
			msgOpenConfirm,
			pathEndConnectionHandshakeMessages.Src,
		) {
			res.DstMessages = append(res.DstMessages, msgOpenConfirm)
		}

		toDeleteDst[conntypes.EventTypeConnectionOpenTry] = append(
			toDeleteDst[conntypes.EventTypeConnectionOpenTry], connKey.Counterparty(),
		)

		// MsgConnectionOpenInit does not have CounterpartyConnectionID
		toDeleteSrc[conntypes.EventTypeConnectionOpenInit] = append(
			toDeleteSrc[conntypes.EventTypeConnectionOpenInit], connKey.MsgInitKey(),
		)
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], connKey.PreInitKey())
	}

	processRemovals()

	for connKey, info := range pathEndConnectionHandshakeMessages.DstMsgConnectionOpenTry {
		// need to send an open ack to src
		msgOpenAck := connectionIBCMessage{
			eventType: conntypes.EventTypeConnectionOpenAck,
			info:      info,
		}
		if pathEndConnectionHandshakeMessages.Src.shouldSendConnectionMessage(
			msgOpenAck, pathEndConnectionHandshakeMessages.Dst,
		) {
			res.SrcMessages = append(res.SrcMessages, msgOpenAck)
		}

		counterpartyKey := connKey.Counterparty()

		// MsgConnectionOpenInit does not have CounterpartyConnectionID
		toDeleteSrc[conntypes.EventTypeConnectionOpenInit] = append(
			toDeleteSrc[conntypes.EventTypeConnectionOpenInit], counterpartyKey.MsgInitKey(),
		)
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], counterpartyKey.PreInitKey())
	}

	processRemovals()

	for connKey, info := range pathEndConnectionHandshakeMessages.SrcMsgConnectionOpenInit {
		// need to send an open try to dst
		msgOpenTry := connectionIBCMessage{
			eventType: conntypes.EventTypeConnectionOpenTry,
			info:      info,
		}
		if pathEndConnectionHandshakeMessages.Dst.shouldSendConnectionMessage(
			msgOpenTry, pathEndConnectionHandshakeMessages.Src,
		) {
			res.DstMessages = append(res.DstMessages, msgOpenTry)
		}

		// MsgConnectionOpenInit does not have CounterpartyConnectionID
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], connKey.PreInitKey())
	}

	processRemovals()

	for _, info := range pathEndConnectionHandshakeMessages.SrcMsgConnectionPreInit {
		// need to send an open init to src
		msgOpenInit := connectionIBCMessage{
			eventType: conntypes.EventTypeConnectionOpenInit,
			info:      info,
		}
		if pathEndConnectionHandshakeMessages.Src.shouldSendConnectionMessage(
			msgOpenInit, pathEndConnectionHandshakeMessages.Dst,
		) {
			res.SrcMessages = append(res.SrcMessages, msgOpenInit)
		}
	}

	return res
}

func (pp *PathProcessor) unrelayedChannelHandshakeMessages(
	pathEndChannelHandshakeMessages pathEndChannelHandshakeMessages,
) pathEndChannelHandshakeResponse {
	var (
		res         pathEndChannelHandshakeResponse
		toDeleteSrc = make(map[string][]ChannelKey)
		toDeleteDst = make(map[string][]ChannelKey)
	)
	processRemovals := func() {
		pathEndChannelHandshakeMessages.Src.messageCache.ChannelHandshake.DeleteMessages(toDeleteSrc)
		pathEndChannelHandshakeMessages.Dst.messageCache.ChannelHandshake.DeleteMessages(toDeleteDst)
		pathEndChannelHandshakeMessages.Src.channelProcessing.deleteMessages(toDeleteSrc)
		pathEndChannelHandshakeMessages.Dst.channelProcessing.deleteMessages(toDeleteDst)
		toDeleteSrc = make(map[string][]ChannelKey)
		toDeleteDst = make(map[string][]ChannelKey)
	}

	for chanKey := range pathEndChannelHandshakeMessages.DstMsgChannelOpenConfirm {
		// found open confirm, channel handshake complete. remove all retention

		counterpartyKey := chanKey.Counterparty()
		toDeleteDst[chantypes.EventTypeChannelOpenConfirm] = append(
			toDeleteDst[chantypes.EventTypeChannelOpenConfirm],
			chanKey,
		)
		toDeleteSrc[chantypes.EventTypeChannelOpenAck] = append(
			toDeleteSrc[chantypes.EventTypeChannelOpenAck],
			counterpartyKey,
		)
		toDeleteDst[chantypes.EventTypeChannelOpenTry] = append(
			toDeleteDst[chantypes.EventTypeChannelOpenTry],
			chanKey,
		)

		// MsgChannelOpenInit does not have CounterpartyChannelID
		toDeleteSrc[chantypes.EventTypeChannelOpenInit] = append(
			toDeleteSrc[chantypes.EventTypeChannelOpenInit],
			counterpartyKey.MsgInitKey(),
		)
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], counterpartyKey.PreInitKey())
	}

	processRemovals()

	for chanKey, info := range pathEndChannelHandshakeMessages.SrcMsgChannelOpenAck {
		// need to send an open confirm to dst
		msgOpenConfirm := channelIBCMessage{
			eventType: chantypes.EventTypeChannelOpenConfirm,
			info:      info,
		}

		if pathEndChannelHandshakeMessages.Dst.shouldSendChannelMessage(
			msgOpenConfirm,
			pathEndChannelHandshakeMessages.Src,
		) {
			res.DstMessages = append(res.DstMessages, msgOpenConfirm)
		}

		toDeleteDst[chantypes.EventTypeChannelOpenTry] = append(
			toDeleteDst[chantypes.EventTypeChannelOpenTry], chanKey.Counterparty(),
		)

		// MsgChannelOpenInit does not have CounterpartyChannelID
		toDeleteSrc[chantypes.EventTypeChannelOpenInit] = append(
			toDeleteSrc[chantypes.EventTypeChannelOpenInit], chanKey.MsgInitKey(),
		)
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], chanKey.PreInitKey())
	}

	processRemovals()

	for chanKey, info := range pathEndChannelHandshakeMessages.DstMsgChannelOpenTry {
		// need to send an open ack to src
		msgOpenAck := channelIBCMessage{
			eventType: chantypes.EventTypeChannelOpenAck,
			info:      info,
		}
		if pathEndChannelHandshakeMessages.Src.shouldSendChannelMessage(
			msgOpenAck, pathEndChannelHandshakeMessages.Dst,
		) {
			res.SrcMessages = append(res.SrcMessages, msgOpenAck)
		}

		counterpartyKey := chanKey.Counterparty()

		// MsgChannelOpenInit does not have CounterpartyChannelID
		toDeleteSrc[chantypes.EventTypeChannelOpenInit] = append(
			toDeleteSrc[chantypes.EventTypeChannelOpenInit], counterpartyKey.MsgInitKey(),
		)
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], counterpartyKey.PreInitKey())
	}

	processRemovals()

	for chanKey, info := range pathEndChannelHandshakeMessages.SrcMsgChannelOpenInit {
		// need to send an open try to dst
		msgOpenTry := channelIBCMessage{
			eventType: chantypes.EventTypeChannelOpenTry,
			info:      info,
		}
		if pathEndChannelHandshakeMessages.Dst.shouldSendChannelMessage(
			msgOpenTry, pathEndChannelHandshakeMessages.Src,
		) {
			res.DstMessages = append(res.DstMessages, msgOpenTry)
		}

		// MsgChannelOpenInit does not have CounterpartyChannelID
		toDeleteSrc[preInitKey] = append(toDeleteSrc[preInitKey], chanKey.PreInitKey())
	}

	processRemovals()

	for _, info := range pathEndChannelHandshakeMessages.SrcMsgChannelPreInit {
		// need to send an open init to src
		msgOpenInit := channelIBCMessage{
			eventType: chantypes.EventTypeChannelOpenInit,
			info:      info,
		}
		if pathEndChannelHandshakeMessages.Src.shouldSendChannelMessage(
			msgOpenInit, pathEndChannelHandshakeMessages.Dst,
		) {
			res.SrcMessages = append(res.SrcMessages, msgOpenInit)
		}
	}

	return res
}

func (pp *PathProcessor) unrelayedChannelCloseMessages(
	pathEndChannelCloseMessages pathEndChannelCloseMessages,
) pathEndChannelHandshakeResponse {
	var (
		res         pathEndChannelHandshakeResponse
		toDeleteSrc = make(map[string][]ChannelKey)
		toDeleteDst = make(map[string][]ChannelKey)
	)
	processRemovals := func() {
		pathEndChannelCloseMessages.Src.messageCache.ChannelHandshake.DeleteMessages(toDeleteSrc)
		pathEndChannelCloseMessages.Dst.messageCache.ChannelHandshake.DeleteMessages(toDeleteDst)
		pathEndChannelCloseMessages.Src.channelProcessing.deleteMessages(toDeleteSrc)
		pathEndChannelCloseMessages.Dst.channelProcessing.deleteMessages(toDeleteDst)
		toDeleteSrc = make(map[string][]ChannelKey)
		toDeleteDst = make(map[string][]ChannelKey)
	}

	for chanKey := range pathEndChannelCloseMessages.DstMsgChannelCloseConfirm {
		// found close confirm, channel handshake complete. remove all retention

		counterpartyKey := chanKey.Counterparty()
		toDeleteDst[chantypes.EventTypeChannelCloseConfirm] = append(
			toDeleteDst[chantypes.EventTypeChannelCloseConfirm],
			chanKey,
		)
		// MsgChannelCloseInit does not have CounterpartyChannelID // TODO: confirm this
		toDeleteSrc[chantypes.EventTypeChannelCloseInit] = append(
			toDeleteSrc[chantypes.EventTypeChannelCloseInit],
			counterpartyKey.MsgInitKey(),
		)
		// TODO: confirm chankey does not need modification
		toDeleteSrc[preCloseKey] = append(toDeleteSrc[preCloseKey], counterpartyKey)
	}

	processRemovals()

	for chanKey, info := range pathEndChannelCloseMessages.SrcMsgChannelCloseInit {
		// need to send a close confirm to dst
		msgCloseConfirm := channelIBCMessage{
			eventType: chantypes.EventTypeChannelCloseConfirm,
			info:      info,
		}
		if pathEndChannelCloseMessages.Dst.shouldSendChannelMessage(
			msgCloseConfirm, pathEndChannelCloseMessages.Src,
		) {
			res.DstMessages = append(res.DstMessages, msgCloseConfirm)
		}

		// TODO: confirm chankey does not need modification
		toDeleteSrc[preCloseKey] = append(toDeleteSrc[preCloseKey], chanKey)
	}

	processRemovals()

	for _, info := range pathEndChannelCloseMessages.SrcMsgChannelPreInit {
		// need to send a close init to src
		msgCloseInit := channelIBCMessage{
			eventType: chantypes.EventTypeChannelCloseInit,
			info:      info,
		}
		if pathEndChannelCloseMessages.Src.shouldSendChannelMessage(
			msgCloseInit, pathEndChannelCloseMessages.Dst,
		) {
			res.SrcMessages = append(res.SrcMessages, msgCloseInit)
		}
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
		if ibcHeaderCurrent, ok := dst.ibcHeaderCache[src.clientState.ConsensusHeight.RevisionHeight]; ok &&
			dst.clientTrustedState.IBCHeader != nil &&
			bytes.Equal(dst.clientTrustedState.IBCHeader.NextValidatorsHash(), ibcHeaderCurrent.NextValidatorsHash()) {
			src.clientTrustedState = provider.ClientTrustedState{
				ClientState: src.clientState,
				IBCHeader:   ibcHeaderCurrent,
			}
			return
		}
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

var observedEventTypeForDesiredMessage = map[string]string{
	conntypes.EventTypeConnectionOpenConfirm: conntypes.EventTypeConnectionOpenAck,
	conntypes.EventTypeConnectionOpenAck:     conntypes.EventTypeConnectionOpenTry,
	conntypes.EventTypeConnectionOpenTry:     conntypes.EventTypeConnectionOpenInit,
	conntypes.EventTypeConnectionOpenInit:    preInitKey,

	chantypes.EventTypeChannelOpenConfirm: chantypes.EventTypeChannelOpenAck,
	chantypes.EventTypeChannelOpenAck:     chantypes.EventTypeChannelOpenTry,
	chantypes.EventTypeChannelOpenTry:     chantypes.EventTypeChannelOpenInit,
	chantypes.EventTypeChannelOpenInit:    preInitKey,

	chantypes.EventTypeAcknowledgePacket: chantypes.EventTypeRecvPacket,
	chantypes.EventTypeRecvPacket:        chantypes.EventTypeSendPacket,
	chantypes.EventTypeSendPacket:        preInitKey,
}

func (pp *PathProcessor) queuePreInitMessages(cancel func()) {
	if pp.messageLifecycle == nil || pp.sentInitialMsg {
		return
	}

	switch m := pp.messageLifecycle.(type) {
	case *PacketMessageLifecycle:
		pp.sentInitialMsg = true
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
			cancel()
			return
		}
		if !pp.IsRelayedChannel(m.Initial.ChainID, channelKey) {
			return
		}
		eventType, ok := observedEventTypeForDesiredMessage[m.Initial.EventType]
		if !ok {
			pp.log.Error(
				"Failed to queue initial connection message, event type not handled",
				zap.String("event_type", m.Initial.EventType),
			)
			cancel()
			return
		}
		if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
			_, ok = pp.pathEnd1.messageCache.PacketFlow[channelKey][eventType]
			if !ok {
				pp.pathEnd1.messageCache.PacketFlow[channelKey][eventType] = make(PacketSequenceCache)
			}
			pp.pathEnd1.messageCache.PacketFlow[channelKey][eventType][0] = m.Initial.Info
		} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
			_, ok = pp.pathEnd2.messageCache.PacketFlow[channelKey][eventType]
			if !ok {
				pp.pathEnd2.messageCache.PacketFlow[channelKey][eventType] = make(PacketSequenceCache)
			}
			pp.pathEnd2.messageCache.PacketFlow[channelKey][eventType][0] = m.Initial.Info
		}
	case *ConnectionMessageLifecycle:
		pp.sentInitialMsg = true
		if m.Initial == nil {
			return
		}
		if !pp.IsRelevantClient(m.Initial.ChainID, m.Initial.Info.ClientID) {
			return
		}
		eventType, ok := observedEventTypeForDesiredMessage[m.Initial.EventType]
		if !ok {
			pp.log.Error(
				"Failed to queue initial connection message, event type not handled",
				zap.String("event_type", m.Initial.EventType),
			)
			cancel()
			return
		}
		connKey := ConnectionInfoConnectionKey(m.Initial.Info)
		if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
			_, ok = pp.pathEnd1.messageCache.ConnectionHandshake[eventType]
			if !ok {
				pp.pathEnd1.messageCache.ConnectionHandshake[eventType] = make(ConnectionMessageCache)
			}
			pp.pathEnd1.messageCache.ConnectionHandshake[eventType][connKey] = m.Initial.Info
		} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
			_, ok = pp.pathEnd2.messageCache.ConnectionHandshake[eventType]
			if !ok {
				pp.pathEnd2.messageCache.ConnectionHandshake[eventType] = make(ConnectionMessageCache)
			}
			pp.pathEnd2.messageCache.ConnectionHandshake[eventType][connKey] = m.Initial.Info
		}
	case *ChannelMessageLifecycle:
		pp.sentInitialMsg = true
		if m.Initial == nil {
			return
		}
		if !pp.IsRelevantConnection(m.Initial.ChainID, m.Initial.Info.ConnID) {
			return
		}
		eventType, ok := observedEventTypeForDesiredMessage[m.Initial.EventType]
		if !ok {
			pp.log.Error(
				"Failed to queue initial channel message, event type not handled",
				zap.String("event_type", m.Initial.EventType),
			)
			cancel()
			return
		}
		chanKey := ChannelInfoChannelKey(m.Initial.Info)
		if m.Initial.ChainID == pp.pathEnd1.info.ChainID {
			_, ok = pp.pathEnd1.messageCache.ChannelHandshake[eventType]
			if !ok {
				pp.pathEnd1.messageCache.ChannelHandshake[eventType] = make(ChannelMessageCache)
			}
			pp.pathEnd1.messageCache.ChannelHandshake[eventType][chanKey] = m.Initial.Info
		} else if m.Initial.ChainID == pp.pathEnd2.info.ChainID {
			_, ok = pp.pathEnd2.messageCache.ChannelHandshake[eventType]
			if !ok {
				pp.pathEnd2.messageCache.ChannelHandshake[eventType] = make(ChannelMessageCache)
			}
			pp.pathEnd2.messageCache.ChannelHandshake[eventType][chanKey] = m.Initial.Info
		}
	case *ChannelCloseLifecycle:
		pp.sentInitialMsg = true

		if !pp.IsRelevantConnection(pp.pathEnd1.info.ChainID, m.SrcConnID) {
			return
		}

		for k, open := range pp.pathEnd1.channelStateCache {
			if k.ChannelID == m.SrcChannelID && k.PortID == m.SrcPortID && k.CounterpartyChannelID != "" && k.CounterpartyPortID != "" {
				if open {
					// channel is still open on pathEnd1
					break
				}
				if counterpartyOpen, ok := pp.pathEnd2.channelStateCache[k.Counterparty()]; ok && !counterpartyOpen {
					pp.log.Info("Channel already closed on both sides")
					cancel()
					return
				}
				// queue channel close init on pathEnd1
				if _, ok := pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit]; !ok {
					pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit] = make(ChannelMessageCache)
				}
				pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit][k] = provider.ChannelInfo{
					PortID:                k.PortID,
					ChannelID:             k.ChannelID,
					CounterpartyPortID:    k.CounterpartyPortID,
					CounterpartyChannelID: k.CounterpartyChannelID,
					ConnID:                m.SrcConnID,
				}
				return
			}
		}

		for k, open := range pp.pathEnd2.channelStateCache {
			if k.CounterpartyChannelID == m.SrcChannelID && k.CounterpartyPortID == m.SrcPortID && k.ChannelID != "" && k.PortID != "" {
				if open {
					// channel is still open on pathEnd2
					break
				}
				if counterpartyChanState, ok := pp.pathEnd1.channelStateCache[k.Counterparty()]; ok && !counterpartyChanState {
					pp.log.Info("Channel already closed on both sides")
					cancel()
					return
				}
				// queue channel close init on pathEnd2
				if _, ok := pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit]; !ok {
					pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit] = make(ChannelMessageCache)
				}
				pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit][k] = provider.ChannelInfo{
					PortID:                k.PortID,
					ChannelID:             k.ChannelID,
					CounterpartyPortID:    k.CounterpartyPortID,
					CounterpartyChannelID: k.CounterpartyChannelID,
					ConnID:                m.DstConnID,
				}
			}
		}

		pp.log.Error("This channel is unable to be closed. Channel must already be closed on one chain.",
			zap.String("src_channel_id", m.SrcChannelID),
			zap.String("src_port_id", m.SrcPortID),
		)
		cancel()
	}
}

// messages from both pathEnds are needed in order to determine what needs to be relayed for a single pathEnd
func (pp *PathProcessor) processLatestMessages(ctx context.Context, cancel func()) error {
	// Update trusted client state for both pathends
	pp.updateClientTrustedState(pp.pathEnd1, pp.pathEnd2)
	pp.updateClientTrustedState(pp.pathEnd2, pp.pathEnd1)

	channelPairs := pp.channelPairs()

	pp.queuePreInitMessages(cancel)

	pathEnd1ConnectionHandshakeMessages := pathEndConnectionHandshakeMessages{
		Src:                         pp.pathEnd1,
		Dst:                         pp.pathEnd2,
		SrcMsgConnectionPreInit:     pp.pathEnd1.messageCache.ConnectionHandshake[preInitKey],
		SrcMsgConnectionOpenInit:    pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenConfirm],
	}
	pathEnd2ConnectionHandshakeMessages := pathEndConnectionHandshakeMessages{
		Src:                         pp.pathEnd2,
		Dst:                         pp.pathEnd1,
		SrcMsgConnectionPreInit:     pp.pathEnd2.messageCache.ConnectionHandshake[preInitKey],
		SrcMsgConnectionOpenInit:    pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenInit],
		DstMsgConnectionOpenTry:     pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenTry],
		SrcMsgConnectionOpenAck:     pp.pathEnd2.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenAck],
		DstMsgConnectionOpenConfirm: pp.pathEnd1.messageCache.ConnectionHandshake[conntypes.EventTypeConnectionOpenConfirm],
	}
	pathEnd1ConnectionHandshakeRes := pp.unrelayedConnectionHandshakeMessages(pathEnd1ConnectionHandshakeMessages)
	pathEnd2ConnectionHandshakeRes := pp.unrelayedConnectionHandshakeMessages(pathEnd2ConnectionHandshakeMessages)

	pathEnd1ChannelHandshakeMessages := pathEndChannelHandshakeMessages{
		Src:                      pp.pathEnd1,
		Dst:                      pp.pathEnd2,
		SrcMsgChannelPreInit:     pp.pathEnd1.messageCache.ChannelHandshake[preInitKey],
		SrcMsgChannelOpenInit:    pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenConfirm],
	}
	pathEnd2ChannelHandshakeMessages := pathEndChannelHandshakeMessages{
		Src:                      pp.pathEnd2,
		Dst:                      pp.pathEnd1,
		SrcMsgChannelPreInit:     pp.pathEnd2.messageCache.ChannelHandshake[preInitKey],
		SrcMsgChannelOpenInit:    pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenInit],
		DstMsgChannelOpenTry:     pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenTry],
		SrcMsgChannelOpenAck:     pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenAck],
		DstMsgChannelOpenConfirm: pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelOpenConfirm],
	}
	pathEnd1ChannelHandshakeRes := pp.unrelayedChannelHandshakeMessages(pathEnd1ChannelHandshakeMessages)
	pathEnd2ChannelHandshakeRes := pp.unrelayedChannelHandshakeMessages(pathEnd2ChannelHandshakeMessages)

	// process the packet flows for both path ends to determine what needs to be relayed
	pathEnd1ProcessRes := make([]pathEndPacketFlowResponse, len(channelPairs))
	pathEnd2ProcessRes := make([]pathEndPacketFlowResponse, len(channelPairs))

	for i, pair := range channelPairs {
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
			Src:                   pp.pathEnd1,
			Dst:                   pp.pathEnd2,
			ChannelKey:            pair.pathEnd1ChannelKey,
			SrcPreTransfer:        pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][preInitKey],
			SrcMsgTransfer:        pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeSendPacket],
			DstMsgRecvPacket:      pathEnd1DstMsgRecvPacket,
			SrcMsgAcknowledgement: pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeAcknowledgePacket],
			SrcMsgTimeout:         pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeTimeoutPacket],
			SrcMsgTimeoutOnClose:  pp.pathEnd1.messageCache.PacketFlow[pair.pathEnd1ChannelKey][chantypes.EventTypeTimeoutPacketOnClose],
		}
		pathEnd2PacketFlowMessages := pathEndPacketFlowMessages{
			Src:                   pp.pathEnd2,
			Dst:                   pp.pathEnd1,
			ChannelKey:            pair.pathEnd2ChannelKey,
			SrcPreTransfer:        pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd1ChannelKey][preInitKey],
			SrcMsgTransfer:        pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeSendPacket],
			DstMsgRecvPacket:      pathEnd2DstMsgRecvPacket,
			SrcMsgAcknowledgement: pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeAcknowledgePacket],
			SrcMsgTimeout:         pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeTimeoutPacket],
			SrcMsgTimeoutOnClose:  pp.pathEnd2.messageCache.PacketFlow[pair.pathEnd2ChannelKey][chantypes.EventTypeTimeoutPacketOnClose],
		}

		pathEnd1ProcessRes[i] = pp.unrelayedPacketFlowMessages(ctx, pathEnd1PacketFlowMessages)
		pathEnd2ProcessRes[i] = pp.unrelayedPacketFlowMessages(ctx, pathEnd2PacketFlowMessages)
	}

	pathEnd1ChannelCloseMessages := pathEndChannelCloseMessages{
		Src:                       pp.pathEnd1,
		Dst:                       pp.pathEnd2,
		SrcMsgChannelPreInit:      pp.pathEnd1.messageCache.ChannelHandshake[preCloseKey],
		SrcMsgChannelCloseInit:    pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit],
		DstMsgChannelCloseConfirm: pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseConfirm],
	}
	pathEnd2ChannelCloseMessages := pathEndChannelCloseMessages{
		Src:                       pp.pathEnd2,
		Dst:                       pp.pathEnd1,
		SrcMsgChannelPreInit:      pp.pathEnd2.messageCache.ChannelHandshake[preCloseKey],
		SrcMsgChannelCloseInit:    pp.pathEnd2.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseInit],
		DstMsgChannelCloseConfirm: pp.pathEnd1.messageCache.ChannelHandshake[chantypes.EventTypeChannelCloseConfirm],
	}
	pathEnd1ChannelCloseRes := pp.unrelayedChannelCloseMessages(pathEnd1ChannelCloseMessages)
	pathEnd2ChannelCloseRes := pp.unrelayedChannelCloseMessages(pathEnd2ChannelCloseMessages)

	// concatenate applicable messages for pathend
	pathEnd1ConnectionMessages, pathEnd2ConnectionMessages := pp.connectionMessagesToSend(pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes)
	pathEnd1ChannelMessages, pathEnd2ChannelMessages := pp.channelMessagesToSend(
		pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes,
		pathEnd1ChannelCloseRes, pathEnd2ChannelCloseRes,
	)

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

func (pp *PathProcessor) channelMessagesToSend(pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes, pathEnd1ChannelCloseRes, pathEnd2ChannelCloseRes pathEndChannelHandshakeResponse) ([]channelIBCMessage, []channelIBCMessage) {
	pathEnd1ChannelOpenSrcLen := len(pathEnd1ChannelHandshakeRes.SrcMessages)
	pathEnd1ChannelOpenDstLen := len(pathEnd1ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelOpenDstLen := len(pathEnd2ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelOpenSrcLen := len(pathEnd2ChannelHandshakeRes.SrcMessages)

	pathEnd1ChannelCloseSrcLen := len(pathEnd1ChannelHandshakeRes.SrcMessages)
	pathEnd1ChannelCloseDstLen := len(pathEnd1ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelCloseDstLen := len(pathEnd2ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelCloseSrcLen := len(pathEnd2ChannelHandshakeRes.SrcMessages)

	pathEnd1ChannelMessages := make([]channelIBCMessage, 0, pathEnd1ChannelOpenSrcLen+pathEnd2ChannelOpenDstLen+pathEnd1ChannelCloseSrcLen+pathEnd2ChannelCloseDstLen)
	pathEnd2ChannelMessages := make([]channelIBCMessage, 0, pathEnd2ChannelOpenSrcLen+pathEnd1ChannelOpenDstLen+pathEnd2ChannelCloseSrcLen+pathEnd1ChannelCloseDstLen)

	// pathEnd1 channel messages come from pathEnd1 src and pathEnd2 dst
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd2ChannelHandshakeRes.DstMessages...)
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd2ChannelCloseRes.DstMessages...)
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd1ChannelHandshakeRes.SrcMessages...)
	pathEnd1ChannelMessages = append(pathEnd1ChannelMessages, pathEnd1ChannelCloseRes.SrcMessages...)

	// pathEnd2 channel messages come from pathEnd2 src and pathEnd1 dst
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd1ChannelHandshakeRes.DstMessages...)
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd1ChannelCloseRes.DstMessages...)
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd2ChannelHandshakeRes.SrcMessages...)
	pathEnd2ChannelMessages = append(pathEnd2ChannelMessages, pathEnd2ChannelCloseRes.SrcMessages...)

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

	for i := range channelPairs {
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd2ProcessRes[i].DstMessages...)
		pathEnd1PacketMessages = append(pathEnd1PacketMessages, pathEnd1ProcessRes[i].SrcMessages...)

		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd1ProcessRes[i].DstMessages...)
		pathEnd2PacketMessages = append(pathEnd2PacketMessages, pathEnd2ProcessRes[i].SrcMessages...)

		pathEnd1ChannelMessage = append(pathEnd1ChannelMessage, pathEnd2ProcessRes[i].DstChannelMessage...)
		pathEnd2ChannelMessage = append(pathEnd2ChannelMessage, pathEnd1ProcessRes[i].DstChannelMessage...)
	}

	return pathEnd1PacketMessages, pathEnd2PacketMessages, pathEnd1ChannelMessage, pathEnd2ChannelMessage
}

func queryPacketCommitments(
	ctx context.Context,
	pathEnd *pathEndRuntime,
	k ChannelKey,
	commitments map[ChannelKey][]uint64,
	mu sync.Locker,
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
		sort.SliceStable(commitments[k], func(i, j int) bool {
			return commitments[k][i] < commitments[k][j]
		})
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
	srcMu sync.Locker,
	dstMu sync.Locker,
) func() error {
	return func() error {
		if len(seqs) == 0 {
			src.log.Debug("Nothing to flush", zap.String("channel", k.ChannelID), zap.String("port", k.PortID))
			return nil
		}

		dstChan, dstPort := k.CounterpartyChannelID, k.CounterpartyPortID

		unrecv, err := dst.chainProvider.QueryUnreceivedPackets(ctx, dst.latestBlock.Height, dstChan, dstPort, seqs)
		if err != nil {
			return err
		}

		dstHeight := int64(dst.latestBlock.Height)

		if len(unrecv) > 0 {
			channel, err := dst.chainProvider.QueryChannel(ctx, dstHeight, dstChan, dstPort)
			if err != nil {
				return err
			}

			if channel.Channel.Ordering == chantypes.ORDERED {
				nextSeqRecv, err := dst.chainProvider.QueryNextSeqRecv(ctx, dstHeight, dstChan, dstPort)
				if err != nil {
					return err
				}

				var newUnrecv []uint64

				for _, seq := range unrecv {
					if seq >= nextSeqRecv.NextSequenceReceive {
						newUnrecv = append(newUnrecv, seq)
						break
					}
				}

				unrecv = newUnrecv
			}
		}

		if len(unrecv) > 0 {
			src.log.Debug("Will flush MsgRecvPacket",
				zap.String("channel", k.ChannelID),
				zap.String("port", k.PortID),
				zap.Uint64s("sequences", unrecv),
			)
		} else {
			src.log.Debug("No MsgRecvPacket to flush",
				zap.String("channel", k.ChannelID),
				zap.String("port", k.PortID),
			)
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
			src.log.Debug("Will flush MsgAcknowledgement", zap.Object("channel", k), zap.Uint64s("sequences", unacked))
		} else {
			src.log.Debug("No MsgAcknowledgement to flush", zap.String("channel", k.ChannelID), zap.String("port", k.PortID))
		}

		for _, seq := range unacked {
			recvPacket, err := dst.chainProvider.QueryRecvPacket(ctx, k.CounterpartyChannelID, k.CounterpartyPortID, seq)
			if err != nil {
				return err
			}

			dstMu.Lock()

			ck := k.Counterparty()
			if _, ok := dstCache[ck]; !ok {
				dstCache[ck] = make(PacketMessagesCache)
			}
			if _, ok := dstCache[ck][chantypes.EventTypeRecvPacket]; !ok {
				dstCache[ck][chantypes.EventTypeRecvPacket] = make(PacketSequenceCache)
			}
			if _, ok := dstCache[ck][chantypes.EventTypeWriteAck]; !ok {
				dstCache[ck][chantypes.EventTypeWriteAck] = make(PacketSequenceCache)
			}
			dstCache[ck][chantypes.EventTypeRecvPacket][seq] = recvPacket
			dstCache[ck][chantypes.EventTypeWriteAck][seq] = recvPacket
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
	for k, open := range pp.pathEnd1.channelStateCache {
		if !open {
			continue
		}
		if !pp.pathEnd1.info.ShouldRelayChannel(ChainChannelKey{
			ChainID:             pp.pathEnd1.info.ChainID,
			CounterpartyChainID: pp.pathEnd2.info.ChainID,
			ChannelKey:          k,
		}) {
			continue
		}
		eg.Go(queryPacketCommitments(ctx, pp.pathEnd1, k, commitments1, &commitments1Mu))
	}
	for k, open := range pp.pathEnd2.channelStateCache {
		if !open {
			continue
		}
		if !pp.pathEnd2.info.ShouldRelayChannel(ChainChannelKey{
			ChainID:             pp.pathEnd2.info.ChainID,
			CounterpartyChainID: pp.pathEnd1.info.ChainID,
			ChannelKey:          k,
		}) {
			continue
		}
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
func (pp *PathProcessor) shouldTerminateForFlushComplete() bool {
	if _, ok := pp.messageLifecycle.(*FlushLifecycle); !ok {
		return false
	}
	for k, packetMessagesCache := range pp.pathEnd1.messageCache.PacketFlow {
		if open, ok := pp.pathEnd1.channelStateCache[k]; !ok || !open {
			continue
		}
		for _, c := range packetMessagesCache {
			if len(c) > 0 {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd1.messageCache.ChannelHandshake {
		for k := range pp.pathEnd1.channelStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd1.messageCache.ConnectionHandshake {
		for k := range pp.pathEnd1.connectionStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	for k, packetMessagesCache := range pp.pathEnd2.messageCache.PacketFlow {
		if open, ok := pp.pathEnd1.channelStateCache[k]; !ok || !open {
			continue
		}
		for _, c := range packetMessagesCache {
			if len(c) > 0 {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd2.messageCache.ChannelHandshake {
		for k := range pp.pathEnd1.channelStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd2.messageCache.ConnectionHandshake {
		for k := range pp.pathEnd1.connectionStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	pp.log.Info("Found termination condition for flush, all caches cleared")
	return true
}
