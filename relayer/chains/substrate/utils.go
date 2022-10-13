package substrate

import (
	"bytes"

	"github.com/ComposableFi/go-substrate-rpc-client/v4/scale"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
)

// Encode scale encodes a data type and returns the scale encoded data as a byte type.
func Encode(data any) ([]byte, error) {
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode decodes an encoded type to a target type. It takes encoded bytes and target interface as arguments and
// returns decoded data as the target type.
func Decode(source []byte, target any) error {
	dec := scale.NewDecoder(bytes.NewReader(source))
	err := dec.Decode(target)
	if err != nil {
		return err
	}
	return nil

}

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
