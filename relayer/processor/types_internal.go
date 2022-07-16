package processor

import (
	"sync"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

// pathEndMessages holds the different IBC messages that
// will attempt to be sent to the pathEnd.
type pathEndMessages struct {
	connectionMessages []connectionIBCMessage
	channelMessages    []channelIBCMessage
	packetMessages     []packetIBCMessage
}

type ibcMessage interface {
	ibcMessageIndicator()
}

// packetIBCMessage holds a packet message's eventType and sequence along with it,
// useful for sending packets around internal to the PathProcessor.
type packetIBCMessage struct {
	info      provider.PacketInfo
	eventType string
}

func (packetIBCMessage) ibcMessageIndicator() {}

// channelKey returns channel key for new message by this eventType
// based on prior eventType.
func (p packetIBCMessage) channelKey() (ChannelKey, error) {
	return PacketInfoChannelKey(p.eventType, p.info)
}

// channelIBCMessage holds a channel handshake message's eventType along with its details,
// useful for sending messages around internal to the PathProcessor.
type channelIBCMessage struct {
	eventType string
	info      provider.ChannelInfo
}

func (channelIBCMessage) ibcMessageIndicator() {}

// connectionIBCMessage holds a connection handshake message's eventType along with its details,
// useful for sending messages around internal to the PathProcessor.
type connectionIBCMessage struct {
	eventType string
	info      provider.ConnectionInfo
}

func (connectionIBCMessage) ibcMessageIndicator() {}

// processingMessage tracks the state of a IBC message currently being processed.
type processingMessage struct {
	assembled           bool
	lastProcessedHeight uint64
	retryCount          uint64
}

type packetProcessingCache map[ChannelKey]packetChannelMessageCache
type packetChannelMessageCache map[string]packetMessageSendCache
type packetMessageSendCache map[uint64]processingMessage

func (c packetChannelMessageCache) deleteMessages(toDelete ...map[string][]uint64) {
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

func (c channelProcessingCache) deleteMessages(toDelete ...map[string][]ChannelKey) {
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

func (c connectionProcessingCache) deleteMessages(toDelete ...map[string][]ConnectionKey) {
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
		CounterpartyChannelID: p.DestChannel,
		CounterpartyPortID:    p.DestPort,
	}
}

func connectionInfoConnectionKey(c provider.ConnectionInfo) ConnectionKey {
	return ConnectionKey{
		ClientID:             c.ClientID,
		ConnectionID:         c.ConnID,
		CounterpartyClientID: c.CounterpartyClientID,
		CounterpartyConnID:   c.CounterpartyConnID,
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

// outgoingMessages is a slice of relayer messages that can be
// appended to concurrently.
type outgoingMessages struct {
	mu       sync.Mutex
	msgs     []provider.RelayerMessage
	pktMsgs  []packetMessageToTrack
	connMsgs []connectionMessageToTrack
	chanMsgs []channelMessageToTrack
}

// Append acquires a lock on om's mutex and then appends msg.
// When there are no more possible concurrent calls to Append,
// it is safe to directly access om.msgs.
func (om *outgoingMessages) Append(msg provider.RelayerMessage) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.msgs = append(om.msgs, msg)
}

type packetMessageToTrack struct {
	msg       packetIBCMessage
	assembled bool
}

type connectionMessageToTrack struct {
	msg       connectionIBCMessage
	assembled bool
}

type channelMessageToTrack struct {
	msg       channelIBCMessage
	assembled bool
}
