package processor

import (
	"context"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
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

	msgUpdateClient, err := pp.assembleMsgUpdateClient(src, dst)
	if err != nil {
		return fmt.Errorf("error prepending MsgUpdateClient, chain_id: %s, client_id: %s, %w",
			dst.info.ChainID,
			dst.info.ClientID,
			err,
		)
	}

	messages := []provider.RelayerMessage{msgUpdateClient}

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
