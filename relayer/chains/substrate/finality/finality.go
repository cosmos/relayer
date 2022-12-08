package finality

import (
	"fmt"

	"github.com/ChainSafe/chaindb"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type FinalityGadget interface {
	QueryLatestHeight() (paraHeight int64, relayChainHeight int64, err error)
	QueryHeaderAt(latestRelayChainHeight uint64) (header ibcexported.Header, err error)
	QueryHeaderOverBlocks(finalizedBlockHeight, previouslyFinalizedBlockHeight uint64) (ibcexported.Header, error)
	IBCHeader(header ibcexported.Header) provider.IBCHeader
	ClientState(header provider.IBCHeader) (ibcexported.ClientState, error)
}

func NewFinalityGadget(
	substrateCfg *substrate.SubstrateProviderConfig,
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
