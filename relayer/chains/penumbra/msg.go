package penumbra

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap/zapcore"
)

type PenumbraMessage struct {
	Msg sdk.Msg
}

func NewPenumbraMessage(msg sdk.Msg) provider.RelayerMessage {
	return PenumbraMessage{
		Msg: msg,
	}
}

func PenumbraMsg(rm provider.RelayerMessage) sdk.Msg {
	if val, ok := rm.(PenumbraMessage); !ok {
		fmt.Printf("got data of type %T but wanted PenumbraMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

// typedPenumbraMsg does not accept nil. IBC Message must be of the requested type.
func typedPenumbraMsg[T *chantypes.MsgRecvPacket | *chantypes.MsgAcknowledgement](msg provider.RelayerMessage) T {
	if msg == nil {
		panic("msg is nil")
	}
	cosmosMsg := PenumbraMsg(msg)
	if cosmosMsg == nil {
		panic("cosmosMsg is nil")
	}
	return cosmosMsg.(T)
}

func PenumbraMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		switch rMsg.(type) {
		case PenumbraMessage:
			sdkMsgs = append(sdkMsgs, rMsg.(PenumbraMessage).Msg)
		case cosmos.CosmosMessage:
			sdkMsgs = append(sdkMsgs, rMsg.(cosmos.CosmosMessage).Msg)
		default:
			fmt.Printf("got data of type %T but wanted PenumbraMessage \n", rMsg)
			return nil
		}
	}
	return sdkMsgs
}

func (cm PenumbraMessage) Type() string {
	return sdk.MsgTypeURL(cm.Msg)
}

func (cm PenumbraMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(cm.Msg)
}

// MarshalLogObject is used to encode cm to a zap logger with the zap.Object field type.
func (cm PenumbraMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// Using plain json.Marshal or calling cm.Msg.String() both fail miserably here.
	// There is probably a better way to encode the message than this.
	j, err := codec.NewLegacyAmino().MarshalJSON(cm.Msg)
	if err != nil {
		return err
	}
	enc.AddByteString("msg_json", j)
	return nil
}
