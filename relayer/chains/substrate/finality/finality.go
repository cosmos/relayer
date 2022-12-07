package finality

import (
	"fmt"

	"github.com/ChainSafe/chaindb"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate"
)

type FinalityGadget interface {
	ClientState(rpcclienttypes.SignedCommitment) (ibcexported.ClientState, error)
	Header(blockHash rpcclienttypes.Hash, previousFinalizedHash *rpcclienttypes.Hash) (ibcexported.Header, error)
}

func NewFinalityGadget(
	substrateCfg substrate.SubstrateProviderConfig,
	parachainClient, relayChainClient *rpcclient.SubstrateAPI,
	memDB *chaindb.BadgerDB,
) (FinalityGadget, error) {
	switch substrateCfg.FinalityGadget {
	case BeefyFinalityGadget:
		return NewBeefy(substrateCfg, parachainClient, relayChainClient, memDB), nil
	default:
		return nil, fmt.Errorf("unsupported finality gadget")
	}
}
