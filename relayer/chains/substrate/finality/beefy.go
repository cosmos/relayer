package finality

import (
	"fmt"

	"github.com/ChainSafe/chaindb"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

const BeefyFinalityGadget = "beefy"

type BeefyIBCHeader struct {
	height       uint64
	SignedHeader *beefyclienttypes.Header
}

func (h BeefyIBCHeader) Height() uint64 {
	return h.height
}

func (h BeefyIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return h.SignedHeader.ConsensusState()
}

var _ FinalityGadget = &Beefy{}

type Beefy struct {
	parachainClient      *rpcclient.SubstrateAPI
	relayChainClient     *rpcclient.SubstrateAPI
	paraID               uint32
	beefyActivationBlock uint32
	memDB                *chaindb.BadgerDB
}

func NewBeefy(
	parachainClient, relayChainClient *rpcclient.SubstrateAPI,
	paraID, beefyActivationBlock uint32,
	memDB *chaindb.BadgerDB,
) *Beefy {
	return &Beefy{
		parachainClient,
		relayChainClient,
		paraID,
		beefyActivationBlock,
		memDB,
	}
}

func (b *Beefy) ClientState(header provider.IBCHeader) (ibcexported.ClientState, error) {
	beefyHeader, ok := header.(BeefyIBCHeader)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted  finality.BeefyIBCHeader \n", header)
	}

	blockHash, err := b.relayChainClient.RPC.Chain.GetBlockHash(beefyHeader.Height())
	if err != nil {
		return nil, err
	}

	commitment, err := b.signedCommitment(blockHash)
	if err != nil {
		return nil, err
	}

	cs, err := b.clientState(commitment)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (b *Beefy) QueryHeaderOverBlocks(finalizedBlockHeight, previouslyFinalizedBlockHeight uint64) (ibcexported.Header, error) {
	finalizedHash, err := b.relayChainClient.RPC.Chain.GetBlockHash(finalizedBlockHeight)
	if err != nil {
		return nil, err
	}

	previouslyFinalizedHash, err := b.relayChainClient.RPC.Chain.GetBlockHash(previouslyFinalizedBlockHeight)
	if err != nil {
		return nil, err
	}

	return b.constructBeefyHeader(finalizedHash, &previouslyFinalizedHash)
}

func (b *Beefy) IBCHeader(signedHeader ibcexported.Header) provider.IBCHeader {
	beefyHeader := signedHeader.(*beefyclienttypes.Header)
	return BeefyIBCHeader{
		SignedHeader: beefyHeader,
		// this height is the relaychain height
		height: uint64(beefyHeader.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber),
	}
}

func (b *Beefy) QueryHeaderAt(relayChainHeight uint64) (header ibcexported.Header, err error) {
	blockHash, err := b.relayChainClient.RPC.Chain.GetBlockHash(relayChainHeight)
	if err != nil {
		return nil, err
	}

	header, err = b.constructBeefyHeader(blockHash, nil)
	if err != nil {
		return nil, err
	}

	return
}

func (b *Beefy) QueryLatestHeight() (int64, int64, error) {
	signedHash, err := b.relayChainClient.RPC.Beefy.GetFinalizedHead()
	if err != nil {
		return 0, 0, err
	}

	block, err := b.relayChainClient.RPC.Chain.GetBlock(signedHash)
	if err != nil {
		return 0, 0, err
	}

	header, err := b.constructBeefyHeader(signedHash, nil)
	if err != nil {
		return 0, 0, err
	}

	var latestParachainHeight int64
	for _, h := range header.HeadersWithProof.Headers {
		decodedHeader, err := beefyclienttypes.DecodeParachainHeader(h.ParachainHeader)
		if err != nil {
			return 0, 0, err
		}

		if int64(decodedHeader.Number) > latestParachainHeight {
			latestParachainHeight = int64(decodedHeader.Number)
		}
	}

	latestRelayChainHeight := int64(block.Block.Header.Number)
	return latestParachainHeight, latestRelayChainHeight, nil
}
