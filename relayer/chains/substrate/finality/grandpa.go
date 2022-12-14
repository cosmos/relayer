package finality

import (
	"github.com/ChainSafe/chaindb"

	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// todo
//   - implement condition for when previouslyFinalizedHeight isn't passed as an argument
//   - implement finality methods
//   - construct grandpa consensus state
//   - implement parsing methods from grandpa to wasm client types.
//   - check if validation data can be used to fetch relaychain data from the parachain
//   - make justification subscription in chain processor generic. It only subscribes to beefy justifications currently
//   - write tests for grandpa construct methods
var _ FinalityGadget = &Grandpa{}

const GrandpaFinalityGadget = "grandpa"

type Grandpa struct {
	parachainClient  *rpcclient.SubstrateAPI
	relayChainClient *rpcclient.SubstrateAPI
	paraID           uint32
	relayChain       int32
	memDB            *chaindb.BadgerDB
}

func NewGrandpa(
	parachainClient,
	relayChainClient *rpcclient.SubstrateAPI,
	paraID uint32,
	relayChain int32,
	memDB *chaindb.BadgerDB,
) *Grandpa {
	return &Grandpa{
		parachainClient,
		relayChainClient,
		paraID,
		relayChain,
		memDB,
	}
}

type GrandpaIBCHeader struct {
	height       uint64
	SignedHeader *types.Header
}

func (h GrandpaIBCHeader) Height() uint64 {
	return h.height
}

func (h GrandpaIBCHeader) ConsensusState() ibcexported.ConsensusState {
	// todo: should this be the first parachain header in the list?
	//parachainHeader := h.SignedHeader.ParachainHeaders[0].ParachainHeader
	//timestamp, err := decodeExtrinsicTimestamp(parachainHeader.Extrinsic)
	//if err != nil {
	//	panic(err)
	//}

	// todo: construct the grandpa consensus state and wrap it in a Wasm consensus
	//return types.ConsensusState{Timestamp: timestamppb.New(timestamp)}
	return nil
}

func (g *Grandpa) QueryLatestHeight() (paraHeight int64, relayChainHeight int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (g *Grandpa) QueryHeaderAt(latestRelayChainHeight uint64) (header ibcexported.Header, err error) {
	//TODO implement me
	panic("implement me")
}

func (g *Grandpa) QueryHeaderOverBlocks(finalizedBlockHeight, previouslyFinalizedBlockHeight uint64) (ibcexported.Header, error) {
	//TODO implement me
	panic("implement me")
}

func (g *Grandpa) IBCHeader(header ibcexported.Header) provider.IBCHeader {
	//TODO implement me
	panic("implement me")
}

func (g *Grandpa) ClientState(_ provider.IBCHeader) (ibcexported.ClientState, error) {
	//grandpaHeader, ok := header.(GrandpaIBCHeader)
	//if !ok {
	//	return nil, fmt.Errorf("got data of type %T but wanted  finality.GrandpaIBCHeader \n", header)
	//}
	//
	//currentAuthorities, err := g.getCurrentAuthorities()
	//if err != nil {
	//	return nil, err
	//}
	//
	//blockHash, err := g.relayChainClient.RPC.Chain.GetBlockHash(grandpaHeader.height)
	//if err != nil {
	//	return nil, err
	//}
	//
	//currentSetId, err := g.getCurrentSetId(blockHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//latestRelayHash, err := g.relayChainClient.RPC.Chain.GetFinalizedHead()
	//if err != nil {
	//	return nil, err
	//}
	//
	//latestRelayheader, err := g.relayChainClient.RPC.Chain.GetHeader(latestRelayHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//paraHeader, err := g.getLatestFinalizedParachainHeader(latestRelayHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//var relayChain types.RelayChain
	//switch types.RelayChain(g.relayChain) {
	//case types.RelayChain_POLKADOT:
	//	relayChain = types.RelayChain_POLKADOT
	//case types.RelayChain_KUSAMA:
	//	relayChain = types.RelayChain_KUSAMA
	//case types.RelayChain_ROCOCO:
	//	relayChain = types.RelayChain_ROCOCO
	//}

	//return types.ClientState{
	//	ParaId:             g.paraID,
	//	CurrentSetId:       currentSetId,
	//	CurrentAuthorities: currentAuthorities,
	//	LatestRelayHash:    latestRelayHash[:],
	//	LatestRelayHeight:  uint32(latestRelayheader.Number),
	//	LatestParaHeight:   uint32(paraHeader.Number),
	//	RelayChain:         relayChain,
	//}, nil
	return nil, nil
}

func (g *Grandpa) ConsensusState() (types.ConsensusState, error) {
	// todo
	return types.ConsensusState{}, nil
}
