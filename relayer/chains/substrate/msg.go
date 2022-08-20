package substrate

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
)

var _ provider.RelayerMessage = &SubstrateMessage{}

type SubstrateMessage struct {
	Msg sdk.Msg
}

func NewSubstrateMessage(msg sdk.Msg) provider.RelayerMessage {
	return SubstrateMessage{
		Msg: msg,
	}
}

func SubstrateMsg(rm provider.RelayerMessage) sdk.Msg {
	if val, ok := rm.(SubstrateMessage); !ok {
		fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func SubstrateMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
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

func (cm SubstrateMessage) Type() string {
	return sdk.MsgTypeURL(cm.Msg)
}

func (cm SubstrateMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(cm.Msg)
}
