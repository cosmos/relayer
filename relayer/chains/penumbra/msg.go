package penumbra

import (
	"fmt"

	"github.com/cosmos/gogoproto/proto"
	"go.uber.org/zap/zapcore"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
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
	} else { //nolint:revive // we need to use a val and that does not work when we fix this lint
		return val.Msg
	}
}

func PenumbraMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		switch msg := rMsg.(type) {
		case PenumbraMessage:
			sdkMsgs = append(sdkMsgs, msg.Msg)
		case cosmos.CosmosMessage:
			sdkMsgs = append(sdkMsgs, msg.Msg)
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
