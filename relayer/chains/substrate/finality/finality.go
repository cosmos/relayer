package finality

import (
	"fmt"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type FinalityGadget interface {
	ClientState(provider.IBCHeader) (ibcexported.ClientState, error)
}

func NewFinalityGadget(
	substrateCfg substrate.SubstrateProviderConfig,
	parachainClient, relayChainClient *rpcclient.SubstrateAPI,
) (FinalityGadget, error) {
	switch substrateCfg.FinalityGadget {
	case BeefyFinalityGadget:
		return NewBeefy(substrateCfg, parachainClient, relayChainClient), nil
	default:
		return nil, fmt.Errorf("unsupported finality gadget")
	}
}
