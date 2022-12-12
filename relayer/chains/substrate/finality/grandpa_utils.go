package finality

import (
	"fmt"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/OneOfOne/xxhash"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/types"
)

const prefixGrandpa = "Grandpa"
const methodCurrentSetID = "CurrentSetId"

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
