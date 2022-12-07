package finality

import (
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

const BeefyFinalityGadget = "beefy"

var _ FinalityGadget = &Beefy{}

type Beefy struct {
	PCfg             provider.ProviderConfig
	parachainClient  *rpcclient.SubstrateAPI
	relayChainClient *rpcclient.SubstrateAPI
}

func NewBeefy(
	cfg provider.ProviderConfig,
	parachainClient, relayChainClient *rpcclient.SubstrateAPI,
) *Beefy {
	return &Beefy{cfg, parachainClient, relayChainClient}
}

func (b Beefy) ClientState(header provider.IBCHeader) (ibcexported.ClientState, error) {
	return &beefyclienttypes.ClientState{}, nil
}
