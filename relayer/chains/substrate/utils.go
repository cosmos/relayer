package substrate

import (
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
)

func intoIBCEventType(substrateType SubstrateEventType) string {
	switch substrateType {
	case CreateClient:
		return clienttypes.EventTypeCreateClient
	case UpdateClient:
		return clienttypes.EventTypeUpdateClient
	case ClientMisbehaviour:
		return clienttypes.EventTypeSubmitMisbehaviour
	case SendPacket:
		return chantypes.EventTypeSendPacket
	case ReceivePacket:
		return chantypes.EventTypeRecvPacket
	case WriteAcknowledgement:
		return chantypes.EventTypeAcknowledgePacket
	case AcknowledgePacket:
		return chantypes.EventTypeWriteAck
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
		return conntypes.EventTypeConnectionOpenInit
	case OpenTryConnection:
		return conntypes.EventTypeConnectionOpenTry
	case OpenAckConnection:
		return conntypes.EventTypeConnectionOpenAck
	case OpenConfirmConnection:
		return conntypes.EventTypeConnectionOpenConfirm
	default:
		return ""
	}
}
