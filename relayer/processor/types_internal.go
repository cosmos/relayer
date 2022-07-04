package processor

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// packetIBCMessage holds a packet message's action along with its details,
// useful for sending packets around internal to the PathProcessor.
type packetIBCMessage struct {
	info   provider.PacketInfo
	action string
}

// channelKey returns channel key for new message by this action
// based on prior action.
func (p packetIBCMessage) channelKey() ChannelKey {
	return PacketInfoChannelKey(p.action, p.info)
}

// channelIBCMessage holds a channel handshake message's action along with its details,
// useful for sending messages around internal to the PathProcessor.
type channelIBCMessage struct {
	action string
	info   provider.ChannelInfo
}

// connectionIBCMessage holds a connection handshake message's action along with its details,
// useful for sending messages around internal to the PathProcessor.
type connectionIBCMessage struct {
	action string
	info   provider.ConnectionInfo
}

// processingMessage tracks the send state of an IBC message
type processingMessage struct {
	assembled           bool
	lastProcessedHeight uint64
	retryCount          uint64
}

type packetProcessingCache map[ChannelKey]packetChannelMessageCache
type packetChannelMessageCache map[string]packetMessageSendCache
type packetMessageSendCache map[uint64]processingMessage

func (c packetChannelMessageCache) deleteCachedMessages(toDelete ...map[string][]uint64) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, sequence := range toDeleteMessages {
				delete(c[message], sequence)
			}
		}
	}
}

type channelProcessingCache map[string]channelKeySendCache
type channelKeySendCache map[ChannelKey]processingMessage

func (c channelProcessingCache) deleteCachedMessages(toDelete ...map[string][]ChannelKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, channel := range toDeleteMessages {
				delete(c[message], channel)
			}
		}
	}
}

type connectionProcessingCache map[string]connectionKeySendCache
type connectionKeySendCache map[ConnectionKey]processingMessage

func (c connectionProcessingCache) deleteCachedMessages(toDelete ...map[string][]ConnectionKey) {
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

func packetInfoChannelKey(p provider.PacketInfo) ChannelKey {
	return ChannelKey{
		ChannelID:             p.SourceChannel,
		PortID:                p.SourcePort,
		CounterpartyChannelID: p.DestinationChannel,
		CounterpartyPortID:    p.DestinationPort,
	}
}

func connectionInfoConnectionKey(c provider.ConnectionInfo) ConnectionKey {
	return ConnectionKey{
		ClientID:                 c.ClientID,
		ConnectionID:             c.ConnectionID,
		CounterpartyClientID:     c.CounterpartyClientID,
		CounterpartyConnectionID: c.CounterpartyConnectionID,
	}
}

func channelInfoChannelKey(c provider.ChannelInfo) ChannelKey {
	return ChannelKey{
		ChannelID:             c.ChannelID,
		PortID:                c.PortID,
		CounterpartyChannelID: c.CounterpartyChannelID,
		CounterpartyPortID:    c.CounterpartyPortID,
	}
}
