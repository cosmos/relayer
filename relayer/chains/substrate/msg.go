package substrate

import (
	"bytes"
	"fmt"

	"github.com/ComposableFi/go-substrate-rpc-client/scale"
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
		fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func CosmosMsgs(rm ...provider.RelayerMessage) []Msg {
	sdkMsgs := make([]Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(SubstrateMessage); !ok {
			fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
			return nil
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}

func (sm SubstrateMessage) Type() string {
	return "substrate"
}

func (sm SubstrateMessage) MsgBytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	err := enc.Encode(sm.Msg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalLogObject is used to encode cm to a zap logger with the zap.Object field type.
func (sm SubstrateMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// TODO: marshal substrate message
	// // Using plain json.Marshal or calling cm.Msg.String() both fail miserably here.
	// // There is probably a better way to encode the message than this.
	// j, err := codec.NewLegacyAmino().MarshalJSON(sm.Msg)
	// if err != nil {
	// 	return err
	// }
	// enc.AddByteString("msg_json", j)
	return nil
}
