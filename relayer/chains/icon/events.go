package icon

import (
	"encoding/hex"
	"strings"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
)

// Events
var (
	// Client Events
	EventTypeCreateClient = "CreateClient(str,bytes)"
	EventTypeUpdateClient = "UpdateClient(str)"

	// Connection Events
	EventTypeConnectionOpenInit    = "ConnectionOpenInit(str,str,bytes)"
	EventTypeConnectionOpenTry     = "ConnectionOpenTry(str,str,bytes)"
	EventTypeConnectionOpenAck     = "ConnectionOpenAck(str,bytes)"
	EventTypeConnectionOpenConfirm = "ConnectionOpenConfirm(str,bytes)"

	// Channel Events
	EventTypeChannelOpenInit     = "ChannelOpenInit(str,str,bytes)"
	EventTypeChannelOpenTry      = "ChannelOpenTry(str,str,bytes)"
	EventTypeChannelOpenAck      = "ChannelOpenAck(str,str,bytes)"
	EventTypeChannelOpenConfirm  = "ChannelOpenConfirm(str,str,bytes)"
	EventTypeChannelCloseInit    = "ChannelCloseInit(str,str,bytes)"
	EventTypeChannelCloseConfirm = "ChannelCloseConfirm(str,str,bytes)"

	// Packet Events
	EventTypeSendPacket           = "SendPacket(bytes)"
	EventTypeRecvPacket           = "RecvPacket(bytes)"
	EventTypeWriteAcknowledgement = "WriteAcknowledgement(bytes,bytes)"
	EventTypeAcknowledgePacket    = "AcknowledgePacket(bytes,bytes)"
	EventTypeTimeoutRequest       = "TimeoutRequest(bytes)"
	EventTypePacketTimeout        = "PacketTimeout(bytes)"
)

var IconCosmosEventMap = map[string]string{
	// client events
	EventTypeCreateClient: clienttypes.EventTypeCreateClient,
	EventTypeUpdateClient: clienttypes.EventTypeUpdateClient,

	// connection events
	EventTypeConnectionOpenInit:    conntypes.EventTypeConnectionOpenInit,
	EventTypeConnectionOpenTry:     conntypes.EventTypeConnectionOpenTry,
	EventTypeConnectionOpenAck:     conntypes.EventTypeConnectionOpenAck,
	EventTypeConnectionOpenConfirm: conntypes.EventTypeConnectionOpenConfirm,

	// channel events
	EventTypeChannelOpenInit:     chantypes.EventTypeChannelOpenInit,
	EventTypeChannelOpenTry:      chantypes.EventTypeChannelOpenTry,
	EventTypeChannelOpenAck:      chantypes.EventTypeChannelOpenAck,
	EventTypeChannelOpenConfirm:  chantypes.EventTypeChannelOpenConfirm,
	EventTypeChannelCloseInit:    chantypes.EventTypeChannelCloseInit,
	EventTypeChannelCloseConfirm: chantypes.EventTypeChannelCloseConfirm,

	// packet events
	EventTypeSendPacket:           chantypes.EventTypeSendPacket,
	EventTypeRecvPacket:           chantypes.EventTypeRecvPacket,
	EventTypeWriteAcknowledgement: chantypes.EventTypeWriteAck,
	EventTypeAcknowledgePacket:    chantypes.EventTypeAcknowledgePacket,
	EventTypePacketTimeout:        chantypes.EventTypeTimeoutPacket,
}

func MustConvertEventNameToBytes(eventName string) []byte {
	return []byte(eventName)
}

func ToEventLogBytes(evt types.EventLogStr) types.EventLog {
	indexed := make([][]byte, 0)

	for _, idx := range evt.Indexed {
		indexed = append(indexed, []byte(idx))
	}

	data := make([][]byte, 0)

	for _, d := range evt.Data {
		filtered, _ := hex.DecodeString(strings.TrimPrefix(d, "0x"))
		data = append(data, filtered)
	}

	return types.EventLog{
		Addr:    evt.Addr,
		Indexed: indexed,
		Data:    data,
	}

}

func GetMonitorEventFilters(address string) []*types.EventFilter {

	filters := []*types.EventFilter{}
	if address == "" {
		return filters
	}

	eventArr := []string{
		EventTypeSendPacket,
		EventTypeRecvPacket,
		EventTypeAcknowledgePacket,
		EventTypeWriteAcknowledgement,

		EventTypeConnectionOpenInit,
		EventTypeConnectionOpenTry,
		EventTypeConnectionOpenAck,
		EventTypeConnectionOpenConfirm,

		EventTypeChannelOpenInit,
		EventTypeChannelOpenTry,
		EventTypeChannelOpenAck,
		EventTypeChannelOpenConfirm,
	}

	for _, event := range eventArr {
		filters = append(filters, &types.EventFilter{
			Addr:      types.Address(address),
			Signature: event,
		})
	}
	return filters
}
