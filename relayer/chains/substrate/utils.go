package substrate

import (
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
)

func intoIBCEventType(substrateType SubstrateEventType) string {
	switch substrateType {
	case CreateClient:
		return clienttypes.EventTypeCreateClient
	case UpdateClient:
		return clienttypes.EventTypeUpdateClient
	case ClientMisbehaviour:
		return "not supported" //TOOD: confirm
	case SendPacket:
		return chantypes.EventTypeSendPacket
	case ReceivePacket:
		return "not supported" //TOOD: confirm
	case WriteAcknowledgement:
		return "not supported" //TOOD: confirm
	case AcknowledgePacket:
		return "not supported" //TOOD: confirm
		// return clienttypes.AcknowledgePacket
	case TimeoutPacket:
		return chantypes.EventTypeTimeoutPacket
	case TimeoutOnClosePacket:
		return chantypes.EventTypeTimeoutPacketOnClose
	case OpenInitChannel:
		return chantypes.EventTypeChannelOpenInit
	case OpenTryChannel:
		return chantypes.EventTypeChannelOpenTry
	case OpenAckChannel:
		return chantypes.EventTypeChannelOpenAck
	case OpenConfirmChannel:
		return chantypes.EventTypeChannelOpenConfirm
	case CloseInitChannel:
		return chantypes.EventTypeChannelCloseInit
	case CloseConfirmChannel:
		return chantypes.EventTypeChannelCloseConfirm
	case OpenInitConnection:
		return chantypes.EventTypeChannelOpenInit
	case OpenTryConnection:
		return chantypes.EventTypeChannelOpenTry
	case OpenAckConnection:
		return "not supported" //TOOD: confirm
	case OpenConfirmConnection:
		return "not supported" //TOOD: confirm
	default:
		return ""
	}
}
