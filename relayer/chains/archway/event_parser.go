package archway

import (
	"time"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
	header          []byte
}

func (c clientInfo) ClientState(trustingPeriod time.Duration) provider.ClientState {
	return provider.ClientState{
		ClientID:        c.clientID,
		ConsensusHeight: c.consensusHeight,
		TrustingPeriod:  trustingPeriod,
		Header:          c.header,
	}
}
