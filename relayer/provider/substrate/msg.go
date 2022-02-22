package substrate

import "github.com/cosmos/relayer/relayer/provider"

// TODO: define methods that are required for a substrate message
type SubstrateMsg interface{}

type SubstrateRelayerMessage struct {
	Msg SubstrateMsg
}

func NewSubstrateRelayerMessage(msg SubstrateMsg) provider.RelayerMessage {
	return SubstrateRelayerMessage{
		msg,
	}
}
