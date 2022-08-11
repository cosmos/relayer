package substrate

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap/zapcore"
)

var _ provider.RelayerMessage = &SubstrateMessage{}

type Msg interface{}

type SubstrateMessage struct {
	Msg Msg
}

func NewSubstrateMessage(msg Msg) provider.RelayerMessage {
	return SubstrateMessage{
		Msg: msg,
	}
}

func SubstrateMsg(rm provider.RelayerMessage) Msg {
	if val, ok := rm.(SubstrateMessage); !ok {
		fmt.Printf("got data of type %T but wanted provider.SubstrateMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func SubstrateMsgs(rm ...provider.RelayerMessage) []Msg {
	sdkMsgs := make([]Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(SubstrateMessage); !ok {
			fmt.Printf("got data of type %T but wanted provider.SubstrateMessage \n", val)
			return nil
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}

func (cm SubstrateMessage) Type() string {
	// TODO: implement
	return ""
}

func (cm SubstrateMessage) MsgBytes() ([]byte, error) {
	// TODO: implement
	return nil, nil
}

// MarshalLogObject is used to encode cm to a zap logger with the zap.Object field type.
func (cm SubstrateMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// Using plain json.Marshal or calling cm.Msg.String() both fail miserably here.
	// There is probably a better way to encode the message than this.
	j, err := codec.NewLegacyAmino().MarshalJSON(cm.Msg)
	if err != nil {
		return err
	}
	enc.AddByteString("msg_json", j)
	return nil
}
