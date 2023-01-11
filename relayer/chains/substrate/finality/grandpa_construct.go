package finality

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ChainSafe/gossamer/lib/common"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	client2 "github.com/ComposableFi/go-substrate-rpc-client/v4/client"
	codec "github.com/ComposableFi/go-substrate-rpc-client/v4/scale"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	types2 "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/finality/types"
	"golang.org/x/exp/slices"
	"sort"
)

const prefixGrandpa = "Grandpa"
const methodCurrentSetID = "CurrentSetId"
const methodStateReadProof = "state_getReadProof"
const grandpaConsensusEngineID = "FRNK"
const MaxUnknownHeaders = 100000

func (g *Grandpa) getCurrentSetId(finalizedHash rpcclienttypes.Hash) (uint64, error) {
	// TODO: it's a heavy operation, we should cache it
	meta, err := g.relayChainClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return 0, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, prefixGrandpa, methodCurrentSetID, nil, nil)
	if err != nil {
		return 0, err
	}

	var currentSetId uint64
	ok, err := g.relayChainClient.RPC.State.GetStorage(storageKey, &currentSetId, finalizedHash)
	if err != nil {
		return 0, err
	}
	if !ok {
		hex, err := rpcclienttypes.EncodeToHex(storageKey)
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("%s: storage key %v, block hash %v", ErrCurrentSetIdNotFound, hex, finalizedHash)
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

func (g *Grandpa) getParachainHeader(finalizedRelayHash rpcclienttypes.Hash) (rpcclienttypes.Header, error) {
	paraKey, err := g.parachainheaderStorageKey()
	if err != nil {
		return rpcclienttypes.Header{}, err
	}
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

func (g *Grandpa) parachainheaderStorageKey() (rpcclienttypes.StorageKey, error) {
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix(prefixParas, methodHeads)
	encodedParaId, err := rpcclienttypes.Encode(g.paraID)
	if err != nil {
		return nil, err
	}

	twoxhash, err := common.Twox64(encodedParaId)
	if err != nil {
		return nil, err
	}
	paraKey := append(append(keyPrefix, twoxhash[:]...), encodedParaId...)
	return paraKey, nil
}

/*
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
*/

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
			return nil, err
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

/*
// todo: implement condition for when previouslyFinalizedHash is nil
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
*/

type ParachainHeadersWithFinalityProof struct {
	FinalityProof    *FinalityProof
	ParachainHeaders []types.ParachainHeaderWithRelayHash
	current          int
}

func (p *ParachainHeadersWithFinalityProof) Reset() {
	p.FinalityProof = nil
	p.ParachainHeaders = nil
}

func (p *ParachainHeadersWithFinalityProof) String() string {
	return fmt.Sprintf("FinalityProof: %v, ParachainHeaders: %v", p.FinalityProof, p.ParachainHeaders)
}

func (p *ParachainHeadersWithFinalityProof) ProtoMessage() {
	fmt.Println("ProtoMessage")
}

func (p *ParachainHeadersWithFinalityProof) ClientType() string {
	return types.ClientType
}

func (p *ParachainHeadersWithFinalityProof) GetHeight() ibcexported.Height {
	fmt.Println("TODO: GetHeight")
	return types2.Height{
		RevisionNumber: 0,
		RevisionHeight: 0,
	}
	//return p.ParachainHeaders[current].ParachainHeader.StateProof
}

func (p *ParachainHeadersWithFinalityProof) ValidateBasic() error {
	fmt.Println("TODO: ValidateBasic")
	return nil
}

type PersistedValidationData struct {
	// The parent head-data.
	ParentHead []byte
	/// The relay-chain block number this is in the context of.
	RelayParentNumber uint32
	/// The relay-chain block storage root this is in the context of.
	RelayParentStorageRoot rpcclienttypes.Hash
	/// The maximum legal size of a POV block, in bytes.
	MaxPovSize uint32
}

func (pvd PersistedValidationData) Encode(encoder codec.Encoder) error {
	err := encoder.Encode(pvd.ParentHead)
	if err != nil {
		return err
	}

	err = encoder.Encode(pvd.RelayParentNumber)
	if err != nil {
		return err
	}

	err = encoder.Encode(pvd.RelayParentStorageRoot)
	if err != nil {
		return err
	}

	err = encoder.Encode(pvd.MaxPovSize)
	if err != nil {
		return err
	}

	return nil
}

func (pvd *PersistedValidationData) Decode(decoder codec.Decoder) error {
	err := decoder.Decode(&pvd.ParentHead)
	if err != nil {
		return err
	}

	err = decoder.Decode(&pvd.RelayParentNumber)
	if err != nil {
		return err
	}

	err = decoder.Decode(&pvd.RelayParentStorageRoot)
	if err != nil {
		return err
	}

	err = decoder.Decode(&pvd.MaxPovSize)
	if err != nil {
		return err
	}

	return nil
}

func HeaderHash(header *rpcclienttypes.Header) (rpcclienttypes.Hash, error) {
	enc, err := rpcclienttypes.Encode(header)
	if err != nil {
		return rpcclienttypes.Hash{}, err
	}

	hash, err := common.Blake2bHash(enc)
	return rpcclienttypes.Hash(hash), err
}

func GetReadProof(client *rpcclient.SubstrateAPI, keys []rpcclienttypes.StorageKey, blockHash *rpcclienttypes.Hash) (*ReadProof, error) {
	keysEncoded, err := rpcclienttypes.EncodeToHex(keys)
	if err != nil {
		return nil, err
	}
	blockHashEncoded, err := rpcclienttypes.EncodeToHex(blockHash)
	if err != nil {
		return nil, err
	}

	var proof string
	err = client2.CallWithBlockHash(client.Client, proof, "state_getKeys", blockHash, keysEncoded, blockHashEncoded)
	if err != nil {
		return nil, err
	}
	println("proof", proof)
	//var res []string
	//
	//err := client.Client.CallWithBlockHash(client, &res, "state_getKeys", blockHash, prefix.Hex())
	//if err != nil {
	//	return nil, err
	//}
	//
	//keys := make([]rpcclienttypes.StorageKey, len(res))
	//for i, r := range res {
	//	err = rpcclienttypes.DecodeFromHex(r, &keys[i])
	//	if err != nil {
	//		return nil, err
	//	}
	//}
	//return keys, err
	return nil, nil
}

func (g *Grandpa) queryFinalizedParachainHeadersWithProof(
	clientState *types.ClientState,
	latestFinalizedHeight uint32,
	headerNumbers []rpcclienttypes.BlockNumber) (*ParachainHeadersWithFinalityProof, error) {
	previousParaHash, err := g.parachainClient.RPC.Chain.GetBlockHash(uint64(clientState.LatestParaHeight) + 1)
	if err != nil {
		return nil, err
	}
	storageKey, err := rpcclienttypes.CreateStorageKey(g.paraMetadata, "ParachainSystem", "ValidationData", nil, nil)
	if err != nil {
		return nil, err
	}
	var validationData PersistedValidationData
	ok, err := g.parachainClient.RPC.State.GetStorage(storageKey, &validationData, previousParaHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s: storage key %v, block hash %v", ErrValidationDataNotFound, storageKey, previousParaHash)
	}
	// This ensures we don't miss any parachain blocks.
	previousFinalizedHeight := uint32(validationData.RelayParentNumber)
	if clientState.LatestRelayHeight < previousFinalizedHeight {
		previousFinalizedHeight = clientState.LatestRelayHeight
	}

	println("session for =", clientState.LatestRelayHeight)
	sessionEnd, err := g.sessionEndForBlock(clientState.LatestRelayHeight)
	println("session end =", sessionEnd)
	if err != nil {
		return nil, err
	}
	if clientState.LatestRelayHeight != sessionEnd && latestFinalizedHeight > sessionEnd {
		latestFinalizedHeight = sessionEnd
	}

	var encoded string
	finalityProof := FinalityProof{}
	println("proving finality for", latestFinalizedHeight)
	err = g.relayChainClient.Client.Call(&encoded, "grandpa_proveFinality", &latestFinalizedHeight)
	println("encoded", encoded)
	if encoded == "" {
		return nil, fmt.Errorf("no justification found for block %v", latestFinalizedHeight)
	}
	data, err := hex.DecodeString(encoded[2:])
	if err != nil {
		return nil, err
	}
	err = finalityProof.Decode(codec.NewDecoder(bytes.NewReader(data)))
	if err != nil {
		return nil, err
	}
	justification := GrandpaJustification{}
	err = justification.Decode(codec.NewDecoder(bytes.NewReader(finalityProof.Justification)))
	if err != nil {
		return nil, err
	}

	latestFinalizedHeight = justification.Commit.TargetNumber
	finalityProof.Block = justification.Commit.TargetHash

	start, err := g.relayChainClient.RPC.Chain.GetBlockHash(uint64(previousFinalizedHeight))
	if err != nil {
		return nil, err
	}

	latestFinalizedHash, err := g.relayChainClient.RPC.Chain.GetBlockHash(uint64(latestFinalizedHeight))
	if err != nil {
		return nil, err
	}

	unknownHeaders := make([]rpcclienttypes.Header, 0)
	for height := previousFinalizedHeight; height <= latestFinalizedHeight; height++ {
		hash, err := g.relayChainClient.RPC.Chain.GetBlockHash(uint64(height))
		if err != nil {
			return nil, err
		}
		header, err := g.relayChainClient.RPC.Chain.GetHeader(hash)
		if err != nil {
			return nil, err
		}
		unknownHeaders = append(unknownHeaders, *header)
	}

	finalityProof.UnknownHeaders = unknownHeaders
	paraStorageKey, err := g.parachainheaderStorageKey()
	if err != nil {
		return nil, err
	}
	keys := []rpcclienttypes.StorageKey{paraStorageKey}
	changeSet, err := g.relayChainClient.RPC.State.QueryStorage(keys, start, latestFinalizedHash)
	if err != nil {
		return nil, err
	}

	var parachainHeadersWithProof = make(map[rpcclienttypes.Hash]types.ParachainHeaderProofs)
	var paraHeaders []Pair[rpcclienttypes.Hash, rpcclienttypes.Header]

	var proofs []TrieProof
	for _, changes := range changeSet {
		header, err := g.relayChainClient.RPC.Chain.GetHeader(changes.Block)
		if err != nil {
			return nil, err
		}
		if header == nil {
			return nil, fmt.Errorf("block not found %v", changes.Block)
		}
		var paraHeader LocalHeader
		headerHash, err := HeaderHash(header)
		if err != nil {
			return nil, err
		}
		encodedParaId, err := rpcclienttypes.Encode(g.paraID)
		if err != nil {
			return nil, err
		}
		key, err := rpcclienttypes.CreateStorageKey(g.relayMetadata, prefixParas, methodHeads, encodedParaId)
		if err != nil {
			return nil, err
		}
		isOk, err := g.relayChainClient.RPC.State.GetStorage(key, &paraHeader, headerHash)
		if err != nil {
			return nil, err
		}
		if !isOk {
			return nil, fmt.Errorf("header exists in its own changeset; qed")
		}

		paraBlockNumber := paraHeader.Number
		// skip genesis header or any unknown headers
		if paraBlockNumber == 0 /*|| !slices.Contains(headerNumbers, paraBlockNumber)*/ {
			continue
		}
		paraHeaders = append(paraHeaders, Pair[rpcclienttypes.Hash, rpcclienttypes.Header]{
			First:  headerHash,
			Second: *header,
		})
		stateProof, err := g.readProof(keys, headerHash)
		if err != nil {
			return nil, err
		}

		hash, err := HeaderHash((*rpcclienttypes.Header)(&paraHeader))
		timeStampExtWithProof, err := g.fetchTimestampExtrinsicWithProof(hash)
		if err != nil {
			return nil, err
		}
		proof := slices.Clone(timeStampExtWithProof.Proof.Proof)
		proofs = append(proofs, timeStampExtWithProof.Proof)
		proofs := types.ParachainHeaderProofs{
			StateProof:     stateProof.Proof,
			Extrinsic:      timeStampExtWithProof.Ext,
			ExtrinsicProof: proof,
		}
		parachainHeadersWithProof[headerHash] = proofs
	}
	for _, p := range proofs {
		p.Free()
	}

	if len(paraHeaders) > 0 {
		relayHash, _ := paraHeaders[0].First, paraHeaders[0].Second
		ancestry := NewAncestryChain(finalityProof.UnknownHeaders)
		route, err := ancestry.Ancestry(relayHash, finalityProof.Block)
		if err != nil {
			return nil, err
		}
		sort.Slice(route, func(i, j int) bool {
			return slices.Compare(route[i][:], route[j][:]) < 0
		})
		var unknownHeaders []rpcclienttypes.Header
		for _, h := range finalityProof.UnknownHeaders {
			hash, err := HeaderHash(&h)
			if err != nil {
				return nil, err
			}
			if slices.Contains(route, hash) {
				unknownHeaders = append(unknownHeaders, h)
			}
		}
		finalityProof.UnknownHeaders = unknownHeaders
	} else {
		// in the special case where there's no parachain headers, let's only send the the
		// finality target and it's parent block. Fishermen should detect any byzantine
		// activity.
		if len(finalityProof.UnknownHeaders) > 2 {
			finalityProof.UnknownHeaders = finalityProof.UnknownHeaders[len(finalityProof.UnknownHeaders)-2:]
		}
	}

	phwfp := &ParachainHeadersWithFinalityProof{
		FinalityProof:    &finalityProof,
		ParachainHeaders: []types.ParachainHeaderWithRelayHash{},
	}
	return phwfp, nil
}

// AncestryChain is a utility trait implementing finality_grandpa.Chain using a given set of headers.
// This is useful when validating commits, using the given set of headers to verify a valid ancestry
// route to the target commit block.
type AncestryChain struct {
	ancestry map[rpcclienttypes.Hash]rpcclienttypes.Header
}

// NewAncestryChain initializes the ancestry chain given a set of relay chain headers.
func NewAncestryChain(ancestry []rpcclienttypes.Header) *AncestryChain {
	ancestryMap := make(map[rpcclienttypes.Hash]rpcclienttypes.Header)
	for _, h := range ancestry {
		hash, err := HeaderHash(&h)
		if err != nil {
			panic(err)
		}
		ancestryMap[hash] = h
	}
	return &AncestryChain{
		ancestry: ancestryMap,
	}
}

// Header fetches a header from the ancestry chain, given its hash. Returns nil if it doesn't exist.
func (ac *AncestryChain) Header(hash rpcclienttypes.Hash) *rpcclienttypes.Header {
	h, ok := ac.ancestry[hash]
	if !ok {
		return nil
	}
	return &h
}

// Ancestry returns the ancestry route from base to block.
func (ac *AncestryChain) Ancestry(base, block rpcclienttypes.Hash) ([]rpcclienttypes.Hash, error) {
	var route []rpcclienttypes.Hash
	currentHash := block
	for currentHash != base {
		currentHeader := ac.Header(currentHash)
		if currentHeader == nil {
			return nil, fmt.Errorf("not a descendent")
		}
		currentHash = currentHeader.ParentHash
		route = append(route, currentHash)
	}
	return route, nil
}

type Pair[T, U any] struct {
	First  T
	Second U
}

func (p *Pair[T, U]) Decode(decoder codec.Decoder) error {
	var err error
	err = decoder.Decode(&p.First)
	err = decoder.Decode(&p.Second)
	return err
}

func (g *Grandpa) sessionEndForBlock(block uint32) (uint32, error) {
	blockHash, err := g.relayChainClient.RPC.Chain.GetBlockHash(uint64(block))
	//blockH, err := hex.DecodeString("6f24523f40b2380c5f5d786982cdd89929a38730f62c5f733afb035c2495df9e")
	//blockHash = rpcclienttypes.NewHash(blockH)
	if err != nil {
		return 0, err
	}
	storageKey, err := rpcclienttypes.CreateStorageKey(g.relayMetadata, "Babe", "EpochStart", nil, nil)
	if err != nil {
		return 0, err
	}
	var p = Pair[uint32, uint32]{}
	ok, err := g.relayChainClient.RPC.State.GetStorage(storageKey, &p, blockHash)
	println("AA", p.First, p.Second)
	if err != nil {
		println("Failed to fetch epoch information")
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%s: storage key %v, block hash %v", ErrSessionEndDataNotFound, storageKey, blockHash)
	}
	previousEpochStart := p.First
	currentEpochStart := p.Second
	return currentEpochStart + (currentEpochStart - previousEpochStart), nil
}

type LocalHeader rpcclienttypes.Header

func (header *LocalHeader) Decode(decoder codec.Decoder) error {
	var err error
	var bn uint32
	err = decoder.Decode(&bn)
	if err != nil {
		return err
	}
	header.Number = rpcclienttypes.BlockNumber(bn)
	err = decoder.Decode(&header.ParentHash)
	if err != nil {
		return err
	}
	err = decoder.Decode(&header.StateRoot)
	if err != nil {
		return err
	}
	err = decoder.Decode(&header.ExtrinsicsRoot)
	if err != nil {
		return err
	}
	err = decoder.Decode(&header.Digest)
	if err != nil {
		return err
	}
	_, err = decoder.ReadOneByte()
	if err == nil {
		return fmt.Errorf("unexpected data after decoding header")
	}
	return nil
}
