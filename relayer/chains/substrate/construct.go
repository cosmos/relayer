package substrate

import (
	"bytes"

	"github.com/ComposableFi/go-merkle-trees/mmr"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	rpcclientTypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyClientTypes "github.com/cosmos/ibc-go/v5/modules/light-clients/11-beefy/types"
)

func signedCommitment(conn *rpcclient.SubstrateAPI, blockHash rpcclientTypes.Hash) (rpcclientTypes.SignedCommitment, error) {
	signedBlock, err := conn.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return rpcclientTypes.SignedCommitment{}, err
	}

	for _, v := range signedBlock.Justifications {
		// not every relay chain block has a beefy justification
		if bytes.Equal(v.ConsensusEngineID[:], []byte("BEEF")) {
			compactCommitment := &rpcclientTypes.CompactSignedCommitment{}

			err = rpcclientTypes.DecodeFromBytes(v.EncodedJustification, compactCommitment)
			if err != nil {
				return rpcclientTypes.SignedCommitment{}, err
			}
			return compactCommitment.Unpack(), nil
		}
	}

	return rpcclientTypes.SignedCommitment{}, nil
}

func constructBeefyHeader(conn *rpcclient.SubstrateAPI, blockHash rpcclientTypes.Hash) (*beefyClientTypes.Header, error) {
	commitment, err := signedCommitment(conn, blockHash)
	if err != nil {
		return nil, err
	}

	cs, err := clientState(conn, commitment)
	if err != nil {
		return nil, err
	}

	parachainHeads, err := constructParachainHeaders(conn, blockHash, cs)
	if err != nil {
		return nil, err
	}

	batchProofs, err := mmrBatchProofs(conn, blockHash, commitment)
	if err != nil {
		return nil, err
	}

	leafIndex := cs.GetLeafIndexForBlockNumber(uint32(commitment.Commitment.BlockNumber))
	blockNumber := uint32(commitment.Commitment.BlockNumber)
	updateProof, err := mmrUpdateProof(conn, blockHash, commitment, uint64(cs.GetLeafIndexForBlockNumber(blockNumber)))
	if err != nil {
		return nil, err
	}

	return &beefyClientTypes.Header{
		ParachainHeaders: parachainHeads,
		MmrProofs:        mmrBatchProofItems(batchProofs),
		MmrSize:          mmr.LeafIndexToMMRSize(uint64(leafIndex)),
		MmrUpdateProof:   updateProof,
	}, nil
}
