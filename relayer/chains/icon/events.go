package icon

import (
	"encoding/hex"

	"github.com/icon-project/ibc-relayer/relayer/chains/icon/types"
)

// Events
var (

	// Client Events
	EventTypeCreateClient          = "create_client"
	EventTypeUpdateClient          = "update_client"
	EventTypeUpgradeClient         = "upgrade_client"
	EventTypeSubmitMisbehaviour    = "client_misbehaviour"
	EventTypeUpdateClientProposal  = "update_client_proposal"
	EventTypeUpgradeChain          = "upgrade_chain"
	EventTypeUpgradeClientProposal = "upgrade_client_proposal"

	// Connection Events
	EventTypeConnectionOpenInit    = "connection_open_init"
	EventTypeConnectionOpenTry     = "connection_open_try"
	EventTypeConnectionOpenAck     = "connection_open_ack"
	EventTypeConnectionOpenConfirm = "connection_open_confirm"

	// Channel Events
	EventTypeChannelOpenInit     = "channel_open_init"
	EventTypeChannelOpenTry      = "channel_open_try"
	EventTypeChannelOpenAck      = "channel_open_ack"
	EventTypeChannelOpenConfirm  = "channel_open_confirm"
	EventTypeChannelCloseInit    = "channel_close_init"
	EventTypeChannelCloseConfirm = "channel_close_confirm"
	EventTypeChannelClosed       = "channel_close"

	// Packet Events
	EventTypeSendPacket           = "SendPacket(bytes)"
	EventTypeRecvPacket           = "RecvPacket(bytes)"
	EventTypeWriteAck             = "WriteAcknowledgement(string,string,int,bytes)"
	EventTypeAcknowledgePacket    = "AcknowledgePacket(bytes, bytes)"
	EventTypeTimeoutPacket        = "timeout_packet"
	EventTypeTimeoutPacketOnClose = "timeout_on_close_packet"
)

func MustConvertEventNameToBytes(eventName string) []byte {
	input, err := hex.DecodeString(eventName)
	if err != nil {
		return nil
	}
	return input
}

func GetMonitorEventFilters(address string) []*types.EventFilter {

	filters := []*types.EventFilter{}
	if address == "" {
		return filters
	}

	eventArr := []string{
		EventTypeSendPacket,
		// EventTypeRecvPacket,
		// EventTypeWriteAck,
		// EventTypeAcknowledgePacket,
	}

	for _, event := range eventArr {
		filters = append(filters, &types.EventFilter{
			Addr:      types.Address(address),
			Signature: event,
		})
	}
	return filters
}
