package substrate

import (
	"fmt"
	"github.com/cosmos/relayer/relayer/provider"
)

type Msg interface {}

func NewSubstrateRelayerMessage(msg Msg) provider.RelayerMessage {
	return SubstrateRelayerMessage{
		msg,
	}
}

func SubstrateMsg(rm provider.RelayerMessage) Msg {
	if val, ok := rm.(SubstrateRelayerMessage); !ok {
		fmt.Printf("got data of type %T but wanted provider.SubstrateMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func SubstrateMsgs(rm ...provider.RelayerMessage) []Msg {
	sdkMsgs := make([]Msg, 0)
	for _, rMsg := range rm {
		msg := SubstrateMsg(rMsg)
		if msg == nil {
			return nil
		}
		sdkMsgs = append(sdkMsgs, msg)
	}
	return sdkMsgs
}
