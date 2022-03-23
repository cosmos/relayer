package substrate

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer/provider"
)


func NewSubstrateRelayerMessage(msg sdk.Msg) provider.RelayerMessage {
	return SubstrateRelayerMessage{
		msg,
	}
}

func SubstrateMsg(rm provider.RelayerMessage) sdk.Msg {
	if val, ok := rm.(SubstrateRelayerMessage); !ok {
		fmt.Printf("got data of type %T but wanted provider.SubstrateMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func SubstrateMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		msg := SubstrateMsg(rMsg)
		if msg == nil {
			return nil
		}
		sdkMsgs = append(sdkMsgs, msg)
	}
	return sdkMsgs
}
