package cosmos

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap/zapcore"
)

var _ provider.RelayerMessage = &CosmosMessage{}

type CosmosMessage struct {
	Msg sdk.Msg
}

func NewCosmosMessage(msg sdk.Msg) provider.RelayerMessage {
	return CosmosMessage{
		Msg: msg,
	}
}

func CosmosMsg(rm provider.RelayerMessage) sdk.Msg {
	if val, ok := rm.(CosmosMessage); !ok {
		fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func CosmosMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(CosmosMessage); !ok {
			fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
			return nil
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}

func (cm CosmosMessage) Type() string {
	return sdk.MsgTypeURL(cm.Msg)
}

func (cm CosmosMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(cm.Msg)
}

// MarshalLogObject is used to encode cm to a zap logger with the zap.Object field type.
func (cm CosmosMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// Using plain json.Marshal or calling cm.Msg.String() both fail miserably here.
	// There is probably a better way to encode the message than this.
	j, err := codec.NewLegacyAmino().MarshalJSON(cm.Msg)
	if err != nil {
		return err
	}
	enc.AddByteString("msg_json", j)
	return nil
}
