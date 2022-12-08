package finality

import (
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type FinalityGadget interface {
	QueryLatestHeight() (paraHeight int64, relayChainHeight int64, err error)
	QueryHeaderAt(latestRelayChainHeight uint64) (header ibcexported.Header, err error)
	QueryHeaderOverBlocks(finalizedBlockHeight, previouslyFinalizedBlockHeight uint64) (ibcexported.Header, error)
	IBCHeader(header ibcexported.Header) provider.IBCHeader
	ClientState(header provider.IBCHeader) (ibcexported.ClientState, error)
}
