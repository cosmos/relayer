package finality

import (
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

var _ FinalityGadget = Grandpa{}

type Grandpa struct {
	parachainClient  *rpcclient.SubstrateAPI
	relayChainClient *rpcclient.SubstrateAPI
	paraID           uint32
}

func NewGrandpa(parachainClient, relayChainClient *rpcclient.SubstrateAPI, paraID uint32) *Grandpa {
	return &Grandpa{parachainClient, relayChainClient, paraID}
}

func (g Grandpa) QueryLatestHeight() (paraHeight int64, relayChainHeight int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (g Grandpa) QueryHeaderAt(latestRelayChainHeight uint64) (header ibcexported.Header, err error) {
	//TODO implement me
	panic("implement me")
}

func (g Grandpa) QueryHeaderOverBlocks(finalizedBlockHeight, previouslyFinalizedBlockHeight uint64) (ibcexported.Header, error) {
	//TODO implement me
	panic("implement me")
}

func (g Grandpa) IBCHeader(header ibcexported.Header) provider.IBCHeader {
	//TODO implement me
	panic("implement me")
}

func (g Grandpa) ClientState(header provider.IBCHeader) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me")
}
