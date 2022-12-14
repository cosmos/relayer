package finality

import (
	"bytes"
	"fmt"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/OneOfOne/xxhash"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/types"
)

const prefixGrandpa = "Grandpa"
const methodCurrentSetID = "CurrentSetId"
const methodStateReadProof = "state_getReadProof"
const grandpaConsensusEngineID = "FRNK"
const MaxUnknownHeaders = 100000

func (g *Grandpa) getCurrentSetId(finalizedHash rpcclienttypes.Hash) (uint64, error) {
	meta, err := g.relayChainClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return 0, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, prefixGrandpa, methodCurrentSetID, nil, nil)
	if err != nil {
		return 0, err
	}

	var currentSetId uint64
	ok, err := g.parachainClient.RPC.State.GetStorage(storageKey, &currentSetId, finalizedHash)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%s: storage key %v, block hash %v", ErrCurrentSetIdNotFound, storageKey, finalizedHash)
	}

	return currentSetId, nil
}

func (g *Grandpa) getCurrentAuthorities() ([]*types.Authority, error) {
	var currentAuthorities []*types.Authority
	err := g.relayChainClient.Client.Call(&currentAuthorities, "GrandpaApi_grandpa_authorities", "0x")
	if err != nil {
		return nil, err
	}

	// TODO: check for duplicate authority set and return an error if one exists
	return currentAuthorities, nil
}

func (g *Grandpa) getLatestFinalizedParachainHeader(finalizedRelayHash rpcclienttypes.Hash) (rpcclienttypes.Header, error) {
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix(prefixParas, methodHeads)
	encodedParaId, err := rpcclienttypes.Encode(g.paraID)
	if err != nil {
		return rpcclienttypes.Header{}, err
	}

	twoxhash := xxhash.New64().Sum(encodedParaId)
	paraKey := append(append(keyPrefix, twoxhash[:]...), encodedParaId...)
	headData, err := g.relayChainClient.RPC.State.GetStorageRaw(paraKey, finalizedRelayHash)
	if err != nil {
		return rpcclienttypes.Header{}, err
	}

	paraHead, err := beefyclienttypes.DecodeParachainHeader(*headData)
	if err != nil {
		return rpcclienttypes.Header{}, err
	}

	return paraHead, nil
}

func (g *Grandpa) grandpaJustification(blockHash rpcclienttypes.Hash) ([]byte, error) {
	signedBlock, err := g.relayChainClient.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return nil, err
	}

	for _, v := range signedBlock.Justifications {
		if bytes.Equal(v.ConsensusEngineID[:], []byte(grandpaConsensusEngineID)) {
			return v.EncodedJustification, nil
		}
	}

	return nil, fmt.Errorf("%s height: %d", ErrMissingGrandpaJustification, signedBlock.Block.Header.Number)
}

func (g *Grandpa) constructFinalityProof(finalizedBlock, previouslyFinalizedBlock uint64) (*types.FinalityProof, error) {
	finalizedBlockHash, err := g.relayChainClient.RPC.Chain.GetBlockHash(finalizedBlock)
	if err != nil {
		return nil, err
	}

	justification, err := g.grandpaJustification(finalizedBlockHash)
	if err != nil {
		return nil, err
	}

	var unknownHeaders [][]byte
	for {
		if previouslyFinalizedBlock+1 > finalizedBlock || len(unknownHeaders) >= MaxUnknownHeaders {
			break
		}

		header, err := g.relayChainClient.RPC.Chain.GetHeader(finalizedBlockHash)
		if err != nil {
			return nil, err
		}

		encodedHeader, err := rpcclienttypes.Encode(header)
		if err != nil {
			return nil, err
		}

		unknownHeaders = append(unknownHeaders, encodedHeader)
	}

	return &types.FinalityProof{
		Block:          finalizedBlockHash[:],
		Justification:  justification,
		UnknownHeaders: unknownHeaders,
	}, nil
}

// ReadProof struct returned by the RPC call to state_getReadProof
type ReadProof struct {
	/// Block hash used to generate the proof
	Hash rpcclienttypes.Hash
	/// A proof used to prove that storage entries are included in the storage trie
	Proof [][]byte
}

func (g *Grandpa) readProof(key []rpcclienttypes.StorageKey, blockHash rpcclienttypes.Hash) (ReadProof, error) {
	var rp ReadProof
	err := g.relayChainClient.Client.Call(&rp, methodStateReadProof, key, blockHash)
	if err != nil {
		return ReadProof{}, nil
	}

	return rp, nil
}

func (g *Grandpa) fetchParachainHeadersWithRelaychainHash(
	blockHash rpcclienttypes.Hash,
	previouslyFinalizedHash *rpcclienttypes.Hash,
) ([]*types.ParachainHeaderWithRelayHash, error) {
	paraHeaderKey, err := parachainHeaderKey(g.paraID)
	if err != nil {
		return nil, err
	}

	storageKey := []rpcclienttypes.StorageKey{paraHeaderKey}
	changeSet, err := g.relayChainClient.RPC.State.QueryStorage(storageKey, *previouslyFinalizedHash, blockHash)
	if err != nil {
		return nil, err
	}

	var paraHeadersWithRelayHash []*types.ParachainHeaderWithRelayHash
	for _, changes := range changeSet {
		header, err := g.relayChainClient.RPC.Chain.GetHeader(changes.Block)
		if err != nil {
			return nil, err
		}

		headerHash, err := g.relayChainClient.RPC.Chain.GetBlockHash(uint64(header.Number))
		if err != nil {

		}

		meta, err := g.relayChainClient.RPC.State.GetMetadataLatest()
		if err != nil {
			return nil, err
		}

		encodedParaId, err := rpcclienttypes.Encode(g.paraID)
		if err != nil {
			return nil, err
		}

		key, err := rpcclienttypes.CreateStorageKey(meta, prefixParas, methodHeads, encodedParaId)
		if err != nil {
			return nil, err
		}

		var paraHeader rpcclienttypes.Header
		ok, err := g.parachainClient.RPC.State.GetStorage(key, &paraHeader, headerHash)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%s: storage key %v, block hash %v", ErrParachainHeaderNotFound, key, headerHash)
		}

		paraBlockHash, err := g.parachainClient.RPC.Chain.GetBlockHash(uint64(paraHeader.Number))
		if err != nil {
			return nil, err
		}

		stateProof, err := g.readProof(storageKey, paraBlockHash)
		if err != nil {
			return nil, err
		}

		extrinsic, extrinsicProof, err := constructExtrinsics(g.parachainClient, uint64(paraHeader.Number), g.memDB)
		paraHeadersWithRelayHash = append(paraHeadersWithRelayHash, &types.ParachainHeaderWithRelayHash{
			ParachainHeader: &types.ParachainHeaderProofs{
				StateProof:     stateProof.Proof,
				Extrinsic:      extrinsic,
				ExtrinsicProof: extrinsicProof,
			},
			RelayHash: headerHash[:],
		})
	}

	return paraHeadersWithRelayHash, nil
}

//todo: implement condition for when previoslyFinalizedHash is nil
func (g *Grandpa) constructHeader(blockHash rpcclienttypes.Hash,
	previouslyFinalizedHash *rpcclienttypes.Hash) (types.Header, error) {
	blockHeader, err := g.relayChainClient.RPC.Chain.GetHeader(blockHash)
	if err != nil {
		return types.Header{}, err
	}

	finalizedBlockHeader, err := g.relayChainClient.RPC.Chain.GetHeader(*previouslyFinalizedHash)
	if err != nil {
		return types.Header{}, err
	}

	proof, err := g.constructFinalityProof(uint64(blockHeader.Number), uint64(finalizedBlockHeader.Number))
	if err != nil {
		return types.Header{}, err
	}

	parachainHeaders, err := g.fetchParachainHeadersWithRelaychainHash(blockHash, previouslyFinalizedHash)
	if err != nil {
		return types.Header{}, err
	}

	return types.Header{
		FinalityProof:    proof,
		ParachainHeaders: parachainHeaders,
	}, nil
}
