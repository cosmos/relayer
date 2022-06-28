package processor

import "github.com/cosmos/relayer/v2/relayer/provider"

// packetIBCMessage holds a packet message's action and sequence along with it,
// useful for sending packets around internal to the PathProcessor.
type packetIBCMessage struct {
	channelKey ChannelKey
	action     string
	sequence   uint64
	message    provider.RelayerMessage
}

// channelIBCMessage holds a channel handshake message's action and channel along with it,
// useful for sending messages around internal to the PathProcessor.
type channelIBCMessage struct {
	action     string
	channelKey ChannelKey
	message    provider.RelayerMessage
}

// connectionIBCMessage holds a connection handshake message's action and connection along with it,
// useful for sending messages around internal to the PathProcessor.
type connectionIBCMessage struct {
	action        string
	connectionKey ConnectionKey
	message       provider.RelayerMessage
}

// inProgressSend tracks the send state of an IBC message
type inProgressSend struct {
	sendHeight uint64
	retryCount uint64
	message    provider.RelayerMessage
}

type packetSendCache map[ChannelKey]packetChannelMessageCache
type packetChannelMessageCache map[string]packetMessageSendCache
type packetMessageSendCache map[uint64]inProgressSend

func (c packetChannelMessageCache) deleteCachedMessages(toDelete ...map[string][]uint64) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, sequence := range toDeleteMessages {
				delete(c[message], sequence)
			}
		}
	}
}

type channelSendCache map[string]channelKeySendCache
type channelKeySendCache map[ChannelKey]inProgressSend

func (c channelSendCache) deleteCachedMessages(toDelete ...map[string][]ChannelKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, channel := range toDeleteMessages {
				delete(c[message], channel)
			}
		}
	}
}

type connectionSendCache map[string]connectionKeySendCache
type connectionKeySendCache map[ConnectionKey]inProgressSend

func (c connectionSendCache) deleteCachedMessages(toDelete ...map[string][]ConnectionKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, connection := range toDeleteMessages {
				delete(c[message], connection)
			}
		}
	}
}

// contains MsgRecvPacket from counterparty
// entire packet flow
type pathEndPacketFlowMessages struct {
	Src                   *pathEndRuntime
	Dst                   *pathEndRuntime
	ChannelKey            ChannelKey
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
	SrcMessages []packetIBCMessage
	DstMessages []packetIBCMessage

	ToDeleteSrc map[string][]uint64
	ToDeleteDst map[string][]uint64
}

type pathEndChannelHandshakeResponse struct {
	SrcMessages []channelIBCMessage
	DstMessages []channelIBCMessage

	ToDeleteSrc map[string][]ChannelKey
	ToDeleteDst map[string][]ChannelKey
}

type pathEndConnectionHandshakeResponse struct {
	SrcMessages []connectionIBCMessage
	DstMessages []connectionIBCMessage

	ToDeleteSrc map[string][]ConnectionKey
	ToDeleteDst map[string][]ConnectionKey
}
