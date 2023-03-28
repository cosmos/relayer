package icon

import (
	"encoding/hex"

	clientTypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
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
var iconEventNameToEventTypeMap = map[string]string{
	// packet Events
	EventTypeSendPacket:           chantypes.EventTypeSendPacket,
	EventTypeRecvPacket:           chantypes.EventTypeRecvPacket,
	EventTypeWriteAck:             chantypes.EventTypeWriteAck,
	EventTypeAcknowledgePacket:    chantypes.EventTypeAcknowledgePacket,
	EventTypeTimeoutPacket:        chantypes.EventTypeTimeoutPacket,
	EventTypeTimeoutPacketOnClose: chantypes.EventTypeTimeoutPacketOnClose,

	// channel events
	EventTypeChannelOpenInit:     chantypes.EventTypeChannelOpenInit,
	EventTypeChannelOpenTry:      chantypes.EventTypeChannelOpenTry,
	EventTypeChannelOpenAck:      chantypes.EventTypeChannelOpenAck,
	EventTypeChannelOpenConfirm:  chantypes.EventTypeChannelOpenConfirm,
	EventTypeChannelCloseInit:    chantypes.EventTypeChannelCloseInit,
	EventTypeChannelCloseConfirm: chantypes.EventTypeChannelCloseConfirm,
	EventTypeChannelClosed:       chantypes.EventTypeChannelClosed,

	// connection Events
	EventTypeConnectionOpenInit:    conntypes.EventTypeConnectionOpenInit,
	EventTypeConnectionOpenTry:     conntypes.EventTypeConnectionOpenTry,
	EventTypeConnectionOpenAck:     conntypes.EventTypeConnectionOpenAck,
	EventTypeConnectionOpenConfirm: conntypes.EventTypeConnectionOpenConfirm,

	// client Events
	EventTypeCreateClient:          clientTypes.EventTypeCreateClient,
	EventTypeUpdateClient:          clientTypes.EventTypeUpdateClient,
	EventTypeUpgradeClient:         clientTypes.EventTypeUpgradeClient,
	EventTypeSubmitMisbehaviour:    clientTypes.EventTypeSubmitMisbehaviour,
	EventTypeUpdateClientProposal:  clientTypes.EventTypeUpdateClientProposal,
	EventTypeUpgradeChain:          clientTypes.EventTypeUpgradeChain,
	EventTypeUpgradeClientProposal: clientTypes.EventTypeUpgradeClientProposal,
}

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
