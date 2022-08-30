package substrate

import (
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func (scp *SubstrateChainProcessor) handleIBCMessagesFromEvents(ibcEvents rpcclienttypes.IBCEventsQueryResult, height uint64, c processor.IBCMessagesCache) error {
	//TODO implement me
	panic("implement me")
}

// client info attributes and methods
type clientInfo struct {
	Height          clienttypes.Height `json:"height"`
	ClientID        string             `json:"client_id"`
	ClientType      uint32             `json:"client_type"`
	ConsensusHeight clienttypes.Height `json:"consensus_height"`
}

func (c clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        c.ClientID,
		ConsensusHeight: c.ConsensusHeight,
	}
}
