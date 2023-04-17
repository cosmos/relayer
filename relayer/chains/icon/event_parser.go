package icon

import (
	"bytes"

	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"

	"go.uber.org/zap"
)

// EventType: EquivalentIBCEvent
// EventName: IconEventLogSignature
type ibcMessage struct {
	eventType string
	eventName string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, event types.EventLog)
}

type packetInfo provider.PacketInfo

func (pi *packetInfo) parseAttrs(log *zap.Logger, event types.EventLog) {
	eventName := GetEventLogSignature(event.Indexed)

	packetData := event.Indexed[1]
	var packet icon.Packet
	if err := proto.Unmarshal(packetData, &packet); err != nil {
		log.Error("failed to unmarshal packet")
	}
	pi.SourcePort = packet.SourcePort
	pi.SourceChannel = packet.SourceChannel
	pi.DestPort = packet.DestinationPort
	pi.DestChannel = packet.DestinationChannel
	pi.Sequence = packet.Sequence
	pi.Data = packet.Data
	if packet.TimeoutHeight != nil {
		pi.TimeoutHeight.RevisionHeight = packet.TimeoutHeight.RevisionHeight
		pi.TimeoutHeight.RevisionNumber = packet.TimeoutHeight.RevisionNumber
	} else {
		pi.TimeoutHeight.RevisionHeight = 200000 // TODO: should be removed
		pi.TimeoutHeight.RevisionNumber = 0      //  TODO: should be removed
	}
	pi.TimeoutTimestamp = packet.TimeoutTimestamp

	if bytes.Equal(eventName, MustConvertEventNameToBytes(EventTypeAcknowledgePacket)) {
		pi.Ack = []byte(event.Indexed[2])
	}

}

type channelInfo provider.ChannelInfo

func (ch *channelInfo) parseAttrs(log *zap.Logger, event types.EventLog) {

	ch.PortID = string(event.Indexed[1])
	ch.ChannelID = string(event.Indexed[2])

	protoChannel := event.Data[0]
	var channel icon.Channel

	if err := proto.Unmarshal(protoChannel, &channel); err != nil {
		log.Error("Error when unmarshalling the event log")
	}

	ch.CounterpartyChannelID = channel.Counterparty.GetChannelId()
	ch.CounterpartyPortID = channel.Counterparty.GetPortId()
	ch.ConnID = channel.ConnectionHops[0]
	ch.Version = channel.GetVersion()
}

type connectionInfo provider.ConnectionInfo

func (co *connectionInfo) parseAttrs(log *zap.Logger, event types.EventLog) {
	eventLog := parseEventName(log, event, 0)
	switch eventLog {
	case EventTypeConnectionOpenInit, EventTypeConnectionOpenTry:
		co.ClientID = string(event.Indexed[1][:])
		co.ConnID = string(event.Data[0][:])
		protoCounterparty := event.Data[1]

		var counterparty icon.Counterparty

		if err := proto.Unmarshal(protoCounterparty, &counterparty); err != nil {
			log.Error("Error decoding counterparty")
		}

		co.CounterpartyClientID = counterparty.GetClientId()
		co.CounterpartyConnID = counterparty.GetConnectionId()

	case EventTypeConnectionOpenAck, EventTypeConnectionOpenConfirm:
		co.ConnID = string(event.Indexed[1])
		protoConnection_ := event.Data[0][:]
		var connection icon.ConnectionEnd
		if err := proto.Unmarshal(protoConnection_, &connection); err != nil {
			log.Error("Error decoding counterparty")
		}

		co.ClientID = connection.GetClientId()
		co.CounterpartyClientID = connection.Counterparty.ClientId
		co.CounterpartyConnID = connection.Counterparty.ConnectionId
	}
}

type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
	header          []byte
}

func (c clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        c.clientID,
		ConsensusHeight: c.consensusHeight,
		Header:          c.header,
	}
}

func (cl *clientInfo) parseAttrs(log *zap.Logger, event types.EventLog) {
	clientId := event.Indexed[1]
	cl.clientID = string(clientId[:])
}

func parseEventName(log *zap.Logger, event types.EventLog, height uint64) string {
	return string(event.Indexed[0][:])
}

func parseIdentifier(event types.EventLog) string {
	return string(event.Indexed[1][:])
}
func parseIBCMessageFromEvent(
	log *zap.Logger,
	event types.EventLog,
	height uint64,
) *ibcMessage {
	eventName := string(event.Indexed[0][:])
	eventType := getEventTypeFromEventName(eventName)

	switch eventName {
	case EventTypeSendPacket, EventTypeRecvPacket, EventTypeAcknowledgePacket, EventTypeWriteAcknowledgement:
		//  EventTypeTimeoutPacket, EventTypeTimeoutPacketOnClose:

		info := &packetInfo{Height: height}
		info.parseAttrs(log, event)
		return &ibcMessage{
			eventType,
			eventName,
			info,
		}
	case EventTypeChannelOpenInit, EventTypeChannelOpenTry,
		EventTypeChannelOpenAck, EventTypeChannelOpenConfirm,
		EventTypeChannelCloseInit, EventTypeChannelCloseConfirm:

		ci := &channelInfo{Height: height}
		ci.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			eventName: eventName,
			info:      ci,
		}
	case EventTypeConnectionOpenInit, EventTypeConnectionOpenTry,
		EventTypeConnectionOpenAck, EventTypeConnectionOpenConfirm:
		ci := &connectionInfo{Height: height}
		ci.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			eventName: eventName,
			info:      ci,
		}
	case EventTypeCreateClient, EventTypeUpdateClient:

		ci := &clientInfo{}
		ci.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			eventName: eventName,
			info:      ci,
		}

	}
	return nil
}

func getEventTypeFromEventName(eventName string) string {
	return IconCosmosEventMap[eventName]
}

func GetEventLogSignature(indexed [][]byte) []byte {
	return indexed[0][:]
}
