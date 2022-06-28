package processor

import (
	"context"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// shouldSendPacketMessage determines if the packet flow message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendPacketMessage(message packetIBCMessage, counterparty *pathEndRuntime) bool {
	msgSendCache, ok := pathEnd.packetSendCache[message.channelKey]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	channelSendCache, ok := msgSendCache[message.action]
	if !ok {
		// in progress cache does not exist for this action, so can send
		return true
	}
	inProgress, ok := channelSendCache[message.sequence]
	if !ok {
		// in progress cache does not exist for this sequence, so can send.
		return true
	}
	if pathEnd.latestBlock.Height-inProgress.sendHeight < blocksToRetryPacketAfter {
		// this message was sent less than blocksToRetryPacketAfter ago, do not send again yet.
		return false
	}
	if inProgress.retryCount == maxMessageSendRetries {
		// giving up on sending this packet flow message
		// remove all retention of this connection handshake in pathEnd.messagesCache.PacketFlow and counterparty
		toDelete := make(map[string][]uint64)
		toDeleteCounterparty := make(map[string][]uint64)
		switch message.action {
		case MsgRecvPacket:
			toDelete[MsgRecvPacket] = []uint64{message.sequence}
			toDeleteCounterparty[MsgTransfer] = []uint64{message.sequence}
		case MsgAcknowledgement, MsgTimeout, MsgTimeoutOnClose:
			toDelete[message.action] = []uint64{message.sequence}
			toDeleteCounterparty[MsgRecvPacket] = []uint64{message.sequence}
			toDelete[MsgTransfer] = []uint64{message.sequence}
		}
		// delete in progress send for this specific message
		pathEnd.packetSendCache[message.channelKey].deleteCachedMessages(map[string][]uint64{message.action: []uint64{message.sequence}})
		// delete all packet flow retention history for this sequence
		pathEnd.messageCache.PacketFlow[message.channelKey].DeleteCachedMessages(toDelete)
		counterparty.messageCache.PacketFlow[message.channelKey].DeleteCachedMessages(toDeleteCounterparty)
		return false
	}

	return true
}

// shouldSendConnectionMessage determines if the connection handshake message should be sent now.
// It will also determine if the message needs to be given up on entirely and remove retention if so.
func (pathEnd *pathEndRuntime) shouldSendConnectionMessage(message connectionIBCMessage, counterparty *pathEndRuntime) bool {
	msgSendCache, ok := pathEnd.connectionSendCache[message.action]
	if !ok {
		// in progress cache does not exist for this action, so can send.
		return true
	}
	inProgress, ok := msgSendCache[message.connectionKey]
	if !ok {
		// in progress cache does not exist for this connection, so can send.
		return true
	}
	if pathEnd.latestBlock.Height-inProgress.sendHeight < blocksToRetryPacketAfter {
		// this message was sent less than blocksToRetryPacketAfter ago, do not send again yet.
		return false
	}
	if inProgress.retryCount == maxMessageSendRetries {
		// giving up on sending this connection handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ConnectionKey)
		toDeleteCounterparty := make(map[string][]ConnectionKey)
		switch message.action {
		case MsgConnectionOpenTry:
			toDeleteCounterparty[MsgConnectionOpenInit] = []ConnectionKey{message.connectionKey}
		case MsgConnectionOpenAck:
			toDeleteCounterparty[MsgConnectionOpenTry] = []ConnectionKey{message.connectionKey}
			toDelete[MsgConnectionOpenInit] = []ConnectionKey{message.connectionKey}
		case MsgConnectionOpenConfirm:
			toDeleteCounterparty[MsgConnectionOpenAck] = []ConnectionKey{message.connectionKey}
			toDelete[MsgConnectionOpenTry] = []ConnectionKey{message.connectionKey}
			toDeleteCounterparty[MsgConnectionOpenInit] = []ConnectionKey{message.connectionKey}
		}
		// delete in progress send for this specific message
		pathEnd.connectionSendCache.deleteCachedMessages(map[string][]ConnectionKey{message.action: []ConnectionKey{message.connectionKey}})
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
	msgSendCache, ok := pathEnd.channelSendCache[message.action]
	if !ok {
		// in progress cache does not exist for this action, so can send.
		return true
	}
	inProgress, ok := msgSendCache[message.channelKey]
	if !ok {
		// in progress cache does not exist for this channel, so can send.
		return true
	}
	if pathEnd.latestBlock.Height-inProgress.sendHeight < blocksToRetryPacketAfter {
		// this message was sent less than blocksToRetryPacketAfter ago, do not send again yet.
		return false
	}
	if inProgress.retryCount == maxMessageSendRetries {
		// giving up on sending this channel handshake message
		// remove all retention of this connection handshake in pathEnd.messagesCache.ConnectionHandshake and counterparty
		toDelete := make(map[string][]ChannelKey)
		toDeleteCounterparty := make(map[string][]ChannelKey)
		switch message.action {
		case MsgChannelOpenTry:
			toDeleteCounterparty[MsgChannelOpenInit] = []ChannelKey{message.channelKey}
		case MsgChannelOpenAck:
			toDeleteCounterparty[MsgChannelOpenTry] = []ChannelKey{message.channelKey}
			toDelete[MsgChannelOpenInit] = []ChannelKey{message.channelKey}
		case MsgChannelOpenConfirm:
			toDeleteCounterparty[MsgChannelOpenAck] = []ChannelKey{message.channelKey}
			toDelete[MsgChannelOpenTry] = []ChannelKey{message.channelKey}
			toDeleteCounterparty[MsgChannelOpenInit] = []ChannelKey{message.channelKey}
		}
		// delete in progress send for this specific message
		pathEnd.channelSendCache.deleteCachedMessages(map[string][]ChannelKey{message.action: []ChannelKey{message.channelKey}})
		// delete all connection handshake retention history for this channel
		pathEnd.messageCache.ChannelHandshake.DeleteCachedMessages(toDelete)
		counterparty.messageCache.ChannelHandshake.DeleteCachedMessages(toDeleteCounterparty)
		return false
	}

	return true
}

func (pathEnd *pathEndRuntime) trackSentPacketMessage(message packetIBCMessage) {
	msgSendCache, ok := pathEnd.packetSendCache[message.channelKey]
	if !ok {
		pathEnd.packetSendCache[message.channelKey] = make(packetChannelMessageCache)
	}
	channelSendCache, ok := msgSendCache[message.action]
	if !ok {
		msgSendCache[message.action] = make(packetMessageSendCache)
	}

	retryCount := uint64(0)

	if inProgress, ok := channelSendCache[message.sequence]; ok {
		retryCount = inProgress.retryCount + 1
	}

	channelSendCache[message.sequence] = inProgressSend{
		sendHeight: pathEnd.latestBlock.Height,
		message:    message.message,
		retryCount: retryCount,
	}
}

func (pathEnd *pathEndRuntime) trackSentConnectionMessage(message connectionIBCMessage) {
	msgSendCache, ok := pathEnd.connectionSendCache[message.action]
	if !ok {
		pathEnd.connectionSendCache[message.action] = make(connectionKeySendCache)
	}

	retryCount := uint64(0)

	if inProgress, ok := msgSendCache[message.connectionKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgSendCache[message.connectionKey] = inProgressSend{
		sendHeight: pathEnd.latestBlock.Height,
		message:    message.message,
		retryCount: retryCount,
	}
}

func (pathEnd *pathEndRuntime) trackSentChannelMessage(message channelIBCMessage) {
	msgSendCache, ok := pathEnd.channelSendCache[message.action]
	if !ok {
		pathEnd.channelSendCache[message.action] = make(channelKeySendCache)
	}

	retryCount := uint64(0)

	if inProgress, ok := msgSendCache[message.channelKey]; ok {
		retryCount = inProgress.retryCount + 1
	}

	msgSendCache[message.channelKey] = inProgressSend{
		sendHeight: pathEnd.latestBlock.Height,
		message:    message.message,
		retryCount: retryCount,
	}
}

func (pp *PathProcessor) sendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	packetMessages []packetIBCMessage,
	connectionMessages []connectionIBCMessage,
	channelMessages []channelIBCMessage,
) error {
	if len(packetMessages) == 0 && len(connectionMessages) == 0 && len(channelMessages) == 0 {
		return nil
	}

	var messages []provider.RelayerMessage

	for _, msg := range packetMessages {
		if !dst.shouldSendPacketMessage(msg, src) {
			continue
		}
		messages = append(messages, msg.message)
	}
	for _, msg := range connectionMessages {
		if !dst.shouldSendConnectionMessage(msg, src) {
			continue
		}
		messages = append(messages, msg.message)
	}
	for _, msg := range channelMessages {
		if !dst.shouldSendChannelMessage(msg, src) {
			continue
		}
		messages = append(messages, msg.message)
	}

	if len(messages) == 0 {
		pp.log.Debug("No messages to send after filtering", zap.String("chain_id", dst.info.ChainID))
		return nil
	}

	msgUpdateClient, err := pp.assembleMsgUpdateClient(ctx, src, dst)
	if err != nil {
		pp.log.Debug("Error prepending MsgUpdateClient",
			zap.String("chain_id", dst.info.ChainID),
			zap.String("client_id", dst.info.ClientID),
			zap.Error(err),
		)
		return err
	}

	messages = append([]provider.RelayerMessage{msgUpdateClient}, messages...)

	_, txSuccess, err := dst.chainProvider.SendMessages(ctx, messages)
	if err != nil {
		return fmt.Errorf("error sending messages to chain_id: %s, %v", dst.info.ChainID, err)
	}
	if !txSuccess {
		return fmt.Errorf("error sending messages to chain_id: %s, transaction was not successful", dst.info.ChainID)
	}

	for _, msg := range packetMessages {
		dst.trackSentPacketMessage(msg)
	}
	for _, msg := range connectionMessages {
		dst.trackSentConnectionMessage(msg)
	}
	for _, msg := range channelMessages {
		dst.trackSentChannelMessage(msg)
	}

	return nil
}

func (pp *PathProcessor) channelMessagesToSend(pathEnd1ChannelHandshakeRes, pathEnd2ChannelHandshakeRes pathEndChannelHandshakeResponse) ([]channelIBCMessage, []channelIBCMessage) {
	pathEnd1ChannelSrcLen := len(pathEnd1ChannelHandshakeRes.SrcMessages)
	pathEnd1ChannelDstLen := len(pathEnd1ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelDstLen := len(pathEnd2ChannelHandshakeRes.DstMessages)
	pathEnd2ChannelSrcLen := len(pathEnd2ChannelHandshakeRes.SrcMessages)
	pathEnd1ChannelMessages := make([]channelIBCMessage, pathEnd1ChannelSrcLen+pathEnd2ChannelDstLen)
	pathEnd2ChannelMessages := make([]channelIBCMessage, pathEnd2ChannelSrcLen+pathEnd1ChannelDstLen)

	// pathEnd1 channel messages come from pathEnd1 src and pathEnd2 dst
	for i, msg := range pathEnd1ChannelHandshakeRes.SrcMessages {
		pathEnd1ChannelMessages[i] = msg
	}
	for i, msg := range pathEnd2ChannelHandshakeRes.DstMessages {
		pathEnd1ChannelMessages[i+pathEnd1ChannelSrcLen] = msg
	}

	// pathEnd2 channel messages come from pathEnd2 src and pathEnd1 dst
	for i, msg := range pathEnd2ChannelHandshakeRes.SrcMessages {
		pathEnd2ChannelMessages[i] = msg
	}
	for i, msg := range pathEnd1ChannelHandshakeRes.DstMessages {
		pathEnd2ChannelMessages[i+pathEnd2ChannelSrcLen] = msg
	}

	pp.pathEnd1.messageCache.ChannelHandshake.DeleteCachedMessages(pathEnd1ChannelHandshakeRes.ToDeleteSrc, pathEnd2ChannelHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ChannelHandshake.DeleteCachedMessages(pathEnd2ChannelHandshakeRes.ToDeleteSrc, pathEnd1ChannelHandshakeRes.ToDeleteDst)

	return pathEnd1ChannelMessages, pathEnd2ChannelMessages
}

func (pp *PathProcessor) connectionMessagesToSend(pathEnd1ConnectionHandshakeRes, pathEnd2ConnectionHandshakeRes pathEndConnectionHandshakeResponse) ([]connectionIBCMessage, []connectionIBCMessage) {
	pathEnd1ConnectionSrcLen := len(pathEnd1ConnectionHandshakeRes.SrcMessages)
	pathEnd1ConnectionDstLen := len(pathEnd1ConnectionHandshakeRes.DstMessages)
	pathEnd2ConnectionDstLen := len(pathEnd2ConnectionHandshakeRes.DstMessages)
	pathEnd2ConnectionSrcLen := len(pathEnd2ConnectionHandshakeRes.SrcMessages)
	pathEnd1ConnectionMessages := make([]connectionIBCMessage, pathEnd1ConnectionSrcLen+pathEnd2ConnectionDstLen)
	pathEnd2ConnectionMessages := make([]connectionIBCMessage, pathEnd2ConnectionSrcLen+pathEnd1ConnectionDstLen)

	// pathEnd1 connection messages come from pathEnd1 src and pathEnd2 dst
	for i, msg := range pathEnd1ConnectionHandshakeRes.SrcMessages {
		pathEnd1ConnectionMessages[i] = msg
	}
	for i, msg := range pathEnd2ConnectionHandshakeRes.DstMessages {
		pathEnd1ConnectionMessages[i+pathEnd1ConnectionSrcLen] = msg
	}

	// pathEnd2 connection messages come from pathEnd2 src and pathEnd1 dst
	for i, msg := range pathEnd2ConnectionHandshakeRes.SrcMessages {
		pathEnd2ConnectionMessages[i] = msg
	}
	for i, msg := range pathEnd1ConnectionHandshakeRes.DstMessages {
		pathEnd2ConnectionMessages[i+pathEnd2ConnectionSrcLen] = msg
	}

	pp.pathEnd1.messageCache.ConnectionHandshake.DeleteCachedMessages(pathEnd1ConnectionHandshakeRes.ToDeleteSrc, pathEnd2ConnectionHandshakeRes.ToDeleteDst)
	pp.pathEnd2.messageCache.ConnectionHandshake.DeleteCachedMessages(pathEnd2ConnectionHandshakeRes.ToDeleteSrc, pathEnd1ConnectionHandshakeRes.ToDeleteDst)
	return pathEnd1ConnectionMessages, pathEnd2ConnectionMessages
}

func (pp *PathProcessor) packetMessagesToSend(channelPairs []channelPair, pathEnd1ProcessRes []*pathEndPacketFlowResponse, pathEnd2ProcessRes []*pathEndPacketFlowResponse) ([]packetIBCMessage, []packetIBCMessage) {
	pathEnd1PacketLen := 0
	pathEnd2PacketLen := 0
	for i := 0; i < len(channelPairs); i++ {
		pathEnd1PacketLen += len(pathEnd2ProcessRes[i].DstMessages) + len(pathEnd1ProcessRes[i].SrcMessages)
		pathEnd2PacketLen += len(pathEnd1ProcessRes[i].DstMessages) + len(pathEnd2ProcessRes[i].SrcMessages)
	}

	pathEnd1PacketMessages := make([]packetIBCMessage, pathEnd1PacketLen)
	pathEnd2PacketMessages := make([]packetIBCMessage, pathEnd2PacketLen)

	pathEnd1PacketItr := 0
	pathEnd2PacketItr := 0

	for i := 0; i < len(channelPairs); i++ {
		for _, msg := range pathEnd2ProcessRes[i].DstMessages {
			pathEnd1PacketMessages[pathEnd1PacketItr] = msg
			pathEnd1PacketItr++
		}
		for _, msg := range pathEnd1ProcessRes[i].SrcMessages {
			pathEnd1PacketMessages[pathEnd1PacketItr] = msg
			pathEnd1PacketItr++
		}

		for _, msg := range pathEnd1ProcessRes[i].DstMessages {
			pathEnd2PacketMessages[pathEnd2PacketItr] = msg
			pathEnd2PacketItr++
		}
		for _, msg := range pathEnd2ProcessRes[i].SrcMessages {
			pathEnd2PacketMessages[pathEnd2PacketItr] = msg
			pathEnd2PacketItr++
		}

		pp.pathEnd1.messageCache.PacketFlow[channelPairs[i].pathEnd1ChannelKey].DeleteCachedMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.messageCache.PacketFlow[channelPairs[i].pathEnd2ChannelKey].DeleteCachedMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)
	}

	return pathEnd1PacketMessages, pathEnd2PacketMessages
}
