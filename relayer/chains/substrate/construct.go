package substrate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/ChainSafe/gossamer/lib/trie"
	"github.com/ComposableFi/go-merkle-trees/hasher"
	"github.com/ComposableFi/go-merkle-trees/merkle"
	"github.com/ComposableFi/go-merkle-trees/mmr"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/OneOfOne/xxhash"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/exp/maps"
)

type Authorities = [][33]uint8

const (
	prefixParas           = "Paras"
	prefixBeefy           = "Beefy"
	methodParachains      = "Parachains"
	methodHeads           = "Heads"
	methodAuthorities     = "Authorities"
	methodNextAuthorities = "NextAuthorities"
)

func (sp *SubstrateProvider) clientState(
	commitment rpcclienttypes.SignedCommitment,
) (*beefyclienttypes.ClientState, error) {
	blockNumber := uint32(commitment.Commitment.BlockNumber)
	authorities, err := sp.beefyAuthorities(blockNumber, methodAuthorities)
	if err != nil {
		return nil, err
	}

	nextAuthorities, err := sp.beefyAuthorities(blockNumber, methodNextAuthorities)
	if err != nil {
		return nil, err
	}

	var authorityLeaves [][]byte
	for _, v := range authorities {
		authorityLeaves = append(authorityLeaves, crypto.Keccak256(v))
	}

	authorityTree, err := merkle.NewTree(hasher.Keccak256Hasher{}).FromLeaves(authorityLeaves)
	if err != nil {
		return nil, err
	}

	var nextAuthorityLeaves [][]byte
	for _, v := range nextAuthorities {
		nextAuthorityLeaves = append(nextAuthorityLeaves, crypto.Keccak256(v))
	}

	nextAuthorityTree, err := merkle.NewTree(hasher.Keccak256Hasher{}).FromLeaves(nextAuthorityLeaves)
	if err != nil {
		return nil, err
	}

	var authorityTreeRoot = bytes32(authorityTree.Root())
	var nextAuthorityTreeRoot = bytes32(nextAuthorityTree.Root())

	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(blockNumber))
	if err != nil {
		return nil, err
	}

	headData, err := sp.paraHeadData(blockHash)
	if err != nil {
		return nil, err
	}

	paraHead, err := beefyclienttypes.DecodeParachainHeader(headData)
	if err != nil {
		return nil, err
	}

	return &beefyclienttypes.ClientState{
		MMRRootHash:          commitment.Commitment.Payload[0].Value,
		LatestBeefyHeight:    blockNumber,
		BeefyActivationBlock: 0,
		Authority: &beefyclienttypes.BeefyAuthoritySet{
			ID:            uint64(commitment.Commitment.ValidatorSetID),
			Len:           uint32(len(authorities)),
			AuthorityRoot: &authorityTreeRoot,
		},
		NextAuthoritySet: &beefyclienttypes.BeefyAuthoritySet{
			ID:            uint64(commitment.Commitment.ValidatorSetID) + 1,
			Len:           uint32(len(nextAuthorities)),
			AuthorityRoot: &nextAuthorityTreeRoot,
		},
		ParaID:           sp.Config.ParaID,
		LatestParaHeight: uint32(paraHead.Number),
		RelayChain:       beefyclienttypes.RelayChain_KUSAMA,
	}, nil
}

func (sp *SubstrateProvider) fetchParaIds(blockHash rpcclienttypes.Hash) ([]uint32, error) {
	// Fetch metadata
	meta, err := sp.RPCClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, prefixParas, methodParachains, nil, nil)
	if err != nil {
		return nil, err
	}

	var paraIds []uint32

	ok, err := sp.RPCClient.RPC.State.GetStorage(storageKey, &paraIds, blockHash)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("%s: storage key %v, paraids %v, block hash %v", ErrBeefyAttributesNotFound, storageKey, paraIds, blockHash)
	}

	return paraIds, nil
}

func (sp *SubstrateProvider) parachainHeaderKey() ([]byte, error) {
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix(prefixParas, methodHeads)
	encodedParaId, err := Encode(sp.Config.ParaID)
	if err != nil {
		return nil, err
	}

	twoxhash := xxhash.New64().Sum(encodedParaId)
	fullKey := append(append(keyPrefix, twoxhash[:]...), encodedParaId...)
	return fullKey, nil
}

func (sp *SubstrateProvider) paraHeadData(blockHash rpcclienttypes.Hash) ([]byte, error) {
	paraKey, err := sp.parachainHeaderKey()
	if err != nil {
		return nil, err
	}

	storage, err := sp.RPCClient.RPC.State.GetStorageRaw(paraKey, blockHash)
	if err != nil {
		return nil, err
	}

	return *storage, nil
}

func (sp *SubstrateProvider) beefyAuthorities(blockNumber uint32, method string) ([][]byte, error) {
	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(blockNumber))
	if err != nil {
		return nil, err
	}

	// Fetch metadata
	meta, err := sp.RPCClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, prefixBeefy, method, nil, nil)
	if err != nil {
		return nil, err
	}

	var authorities Authorities

	ok, err := sp.RPCClient.RPC.State.GetStorage(storageKey, &authorities, blockHash)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("%s: beefy construct not found: storage key %v, authorities %v, block hash %v",
			ErrBeefyConstructNotFound, storageKey, authorities, blockHash)
	}

	// Convert from ecdsa public key to ethereum address
	var authorityEthereumAddresses [][]byte
	for _, authority := range authorities {
		pub, err := crypto.DecompressPubkey(authority[:])
		if err != nil {
			return nil, err
		}
		ethereumAddress := crypto.PubkeyToAddress(*pub)
		if err != nil {
			return nil, err
		}
		authorityEthereumAddresses = append(authorityEthereumAddresses, ethereumAddress[:])
	}

	return authorityEthereumAddresses, nil
}

func (sp *SubstrateProvider) signedCommitment(
	blockHash rpcclienttypes.Hash,
) (rpcclienttypes.SignedCommitment, error) {
	signedBlock, err := sp.RelayerRPCClient.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return rpcclienttypes.SignedCommitment{}, err
	}

	for _, v := range signedBlock.Justifications {
		if bytes.Equal(v.ConsensusEngineID[:], []byte("BEEF")) {
			versionedFinalityProof := &rpcclienttypes.VersionedFinalityProof{}

			err = rpcclienttypes.Decode(v.EncodedJustification, versionedFinalityProof)
			if err != nil {
				return rpcclienttypes.SignedCommitment{}, err
			}

			return versionedFinalityProof.AsCompactSignedCommitment.Unpack(), nil
		}
	}

	return rpcclienttypes.SignedCommitment{}, nil
}

// finalized block returns the finalized block double map that holds block numbers,
// for which our parachain header was included in the mmr leaf, seeing as our parachain
// headers might not make it into every relay chain block. Map<BlockNumber, Map<ParaId, Header>>
// It also returns the leaf indices of those blocks
func (sp *SubstrateProvider) getFinalizedBlocks(
	blockHash rpcclienttypes.Hash,
	previouslyFinalizedBlockHash *rpcclienttypes.Hash,
) (map[uint32]map[uint32][]byte, []uint64, error) {
	var finalizedBlocks = make(map[uint32]map[uint32][]byte)
	var leafIndices []uint64

	if previouslyFinalizedBlockHash == nil {
		var heads = make(map[uint32][]byte)
		headData, err := sp.paraHeadData(blockHash)
		if err != nil {
			return nil, nil, err
		}

		paraHead, err := beefyclienttypes.DecodeParachainHeader(headData)
		if err != nil {
			return nil, nil, err
		}

		heads[sp.Config.ParaID] = headData
		finalizedBlocks[uint32(paraHead.Number)] = heads
		leafIndices = append(leafIndices, uint64(getLeafIndexForBlockNumber(sp.Config.BeefyActivationBlock,
			uint32(paraHead.Number))))
		return finalizedBlocks, leafIndices, nil
	}

	paraHeaderKeys, err := sp.parachainHeaderKeys(blockHash)
	if err != nil {
		return nil, nil, err
	}

	changeSet, err := sp.RPCClient.RPC.State.QueryStorage(paraHeaderKeys, *previouslyFinalizedBlockHash, blockHash)
	if err != nil {
		return nil, nil, err
	}

	for _, changes := range changeSet {
		header, err := sp.RPCClient.RPC.Chain.GetHeader(changes.Block)
		if err != nil {
			return nil, nil, err
		}
		var heads = make(map[uint32][]byte)

		for _, keyValue := range changes.Changes {
			if keyValue.HasStorageData {
				var paraId uint32
				err = rpcclienttypes.Decode(keyValue.StorageKey[40:], &paraId)
				if err != nil {
					return nil, nil, err
				}

				heads[paraId] = keyValue.StorageData
			}
		}

		// check if heads has target id, else skip
		if heads[sp.Config.ParaID] == nil {
			continue
		}

		finalizedBlocks[uint32(header.Number)] = heads

		leafIndices = append(leafIndices, uint64(getLeafIndexForBlockNumber(sp.Config.BeefyActivationBlock,
			uint32(header.Number))))
	}
	return finalizedBlocks, leafIndices, nil
}

func (sp *SubstrateProvider) parachainHeaderKeys(

	blockHash rpcclienttypes.Hash,
) ([]rpcclienttypes.StorageKey, error) {
	paraIds, err := sp.fetchParaIds(blockHash)
	if err != nil {
		return nil, err
	}

	var paraHeaderKeys []rpcclienttypes.StorageKey
	// create full storage key for each known paraId.
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix(prefixParas, methodHeads)
	// so we can query all blocks from lastfinalized to latestBeefyHeight
	for _, paraId := range paraIds {
		encodedParaId, err := Encode(paraId)
		if err != nil {
			return nil, err
		}
		twoxhash := xxhash.New64().Sum(encodedParaId)
		// full key path in the storage source: https://www.shawntabrizi.com/assets/presentations/substrate-storage-deep-dive.pdf
		// xx128("Paras") + xx128("Heads") + xx64(Encode(paraId)) + Encode(paraId)
		fullKey := append(append(keyPrefix, twoxhash[:]...), encodedParaId...)
		paraHeaderKeys = append(paraHeaderKeys, fullKey)
	}

	return paraHeaderKeys, nil
}

func (sp *SubstrateProvider) constructParachainHeaders(
	blockHash rpcclienttypes.Hash,
	previouslyFinalizedBlockHash *rpcclienttypes.Hash,
) ([]*beefyclienttypes.ParachainHeader, error) {
	var finalizedBlocks = make(map[uint32]map[uint32][]byte)
	var leafIndices []uint64
	finalizedBlocks, leafIndices, err := sp.getFinalizedBlocks(blockHash, previouslyFinalizedBlockHash)
	if err != nil {
		return nil, err
	}

	// fetch mmr proofs for leaves containing our target paraId
	mmrBatchProof, err := sp.RPCClient.RPC.MMR.GenerateBatchProof(leafIndices, blockHash)
	if err != nil {
		return nil, err
	}

	var parachainHeaders []*beefyclienttypes.ParachainHeader

	var paraHeads = make([][]byte, len(mmrBatchProof.Leaves))

	for i := 0; i < len(mmrBatchProof.Leaves); i++ {
		v := mmrBatchProof.Leaves[i]
		leafIndex := mmrBatchProof.Proof.LeafIndex[i]

		paraHeads[i] = v.ParachainHeads[:]
		// TODO: The activation block number can be added to the substrate provider and this method as well
		var leafBlockNumber = getBlockNumberForLeaf(sp.Config.BeefyActivationBlock, uint32(leafIndex))
		paraHeaders := finalizedBlocks[leafBlockNumber]

		var paraHeadsLeaves [][]byte
		// index of our parachain header in the
		// parachain heads merkle root
		var index uint64

		count := 0

		// sort by paraId
		sortedParaIds := maps.Keys(paraHeaders)
		sort.SliceStable(sortedParaIds, func(i, j int) bool {
			return sortedParaIds[i] < sortedParaIds[j]
		})

		for _, paraId := range sortedParaIds {
			paraIdScale := make([]byte, 4)
			// scale encode para_id
			binary.LittleEndian.PutUint32(paraIdScale[:], paraId)
			leaf := append(paraIdScale, paraHeaders[paraId]...)
			paraHeadsLeaves = append(paraHeadsLeaves, crypto.Keccak256(leaf))
			if paraId == sp.Config.ParaID {
				// note index of paraId
				index = uint64(count)
			}
			count++
		}

		tree, err := merkle.NewTree(hasher.Keccak256Hasher{}).FromLeaves(paraHeadsLeaves)
		if err != nil {
			return nil, err
		}
		paraHeadsProof := tree.Proof([]uint64{index})
		authorityRoot := bytes32(v.BeefyNextAuthoritySet.Root[:])
		parentHash := bytes32(v.ParentNumberAndHash.Hash[:])

		parachainHeaderDecoded, err := beefyclienttypes.DecodeParachainHeader(paraHeaders[sp.Config.ParaID])
		if err != nil {
			return nil, err
		}

		timestampExt, extProof, err := sp.constructExtrinsics(uint32(parachainHeaderDecoded.Number))
		if err != nil {
			return nil, err
		}

		header := beefyclienttypes.ParachainHeader{
			ParachainHeader: paraHeaders[sp.Config.ParaID],
			PartialMMRLeaf: &beefyclienttypes.PartialMMRLeaf{
				Version:      beefyclienttypes.U8(v.Version),
				ParentNumber: uint32(v.ParentNumberAndHash.ParentNumber),
				ParentHash:   &parentHash,
				BeefyNextAuthoritySet: beefyclienttypes.BeefyAuthoritySet{
					ID:            uint64(v.BeefyNextAuthoritySet.ID),
					Len:           uint32(v.BeefyNextAuthoritySet.Len),
					AuthorityRoot: &authorityRoot,
				},
			},
			ParachainHeadsProof: paraHeadsProof.ProofHashes(),
			HeadsLeafIndex:      uint32(index),
			HeadsTotalCount:     uint32(len(paraHeadsLeaves)),
			TimestampExtrinsic:  timestampExt,
			ExtrinsicProof:      extProof,
		}

		parachainHeaders = append(parachainHeaders, &header)
	}

	return parachainHeaders, nil
}

func (sp *SubstrateProvider) constructExtrinsics(
	blockNumber uint32,
) (timestampExtrinsic []byte, extrinsicProof [][]byte, err error) {
	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(blockNumber))
	if err != nil {
		return nil, nil, err
	}

	block, err := sp.RPCClient.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return nil, nil, err
	}

	exts := block.Block.Extrinsics
	if len(exts) == 0 {
		return nil, nil, nil
	}

	timestampExtrinsic, err = Encode(exts[0])
	if err != nil {
		return nil, nil, err
	}

	t := trie.NewEmptyTrie()
	for i := 0; i < len(exts); i++ {
		ext, err := Encode(exts[i])
		if err != nil {
			return nil, nil, err
		}

		key := rpcclienttypes.NewUCompactFromUInt(uint64(i))
		encodedKey, err := Encode(key)
		if err != nil {
			return nil, nil, err
		}

		t.Put(encodedKey, ext)
	}

	err = t.Store(sp.Memdb)
	if err != nil {
		return nil, nil, err
	}

	rootHash, err := t.Hash()
	if err != nil {
		return nil, nil, err
	}

	timestampKey := rpcclienttypes.NewUCompactFromUInt(uint64(0))
	encodedTPKey, err := Encode(timestampKey)
	if err != nil {
		return nil, nil, err
	}
	extrinsicProof, err = trie.GenerateProof(rootHash.ToBytes(), [][]byte{encodedTPKey}, sp.Memdb)
	if err != nil {
		return nil, nil, err
	}

	return
}

func (sp *SubstrateProvider) mmrBatchProofs(
	blockHash rpcclienttypes.Hash,
	previouslyFinalizedBlockHash *rpcclienttypes.Hash,
) (rpcclienttypes.GenerateMmrBatchProofResponse, error) {
	var leafIndices []uint64
	_, leafIndices, err := sp.getFinalizedBlocks(blockHash, previouslyFinalizedBlockHash)
	if err != nil {
		return rpcclienttypes.GenerateMmrBatchProofResponse{}, err
	}

	// fetch mmr proofs for leaves containing our target paraId
	batchProofs, err := sp.RelayerRPCClient.RPC.MMR.GenerateBatchProof(leafIndices, blockHash)
	if err != nil {
		return rpcclienttypes.GenerateMmrBatchProofResponse{}, err
	}

	return batchProofs, nil
}

func (sp *SubstrateProvider) mmrUpdateProof(
	blockHash rpcclienttypes.Hash,
	signedCommitment rpcclienttypes.SignedCommitment,
	leafIndex uint64,
) (*beefyclienttypes.MMRUpdateProof, error) {
	mmrProof, err := sp.RPCClient.RPC.MMR.GenerateProof(
		leafIndex,
		blockHash,
	)
	if err != nil {
		return nil, err
	}

	latestLeaf := mmrProof.Leaf
	parentHash := bytes32(latestLeaf.ParentNumberAndHash.Hash[:])
	parachainHeads := bytes32(latestLeaf.ParachainHeads[:])
	beefyNextAuthoritySetRoot := bytes32(latestLeaf.BeefyNextAuthoritySet.Root[:])
	commitmentPayload := signedCommitment.Commitment.Payload[0]

	var latestLeafMmrProof = make([][]byte, len(mmrProof.Proof.Items))
	for i := 0; i < len(mmrProof.Proof.Items); i++ {
		latestLeafMmrProof[i] = mmrProof.Proof.Items[i][:]
	}

	var signatures []*beefyclienttypes.CommitmentSignature
	var authorityIndices []uint64
	// luckily for us, this is already sorted and maps to the right authority index in the authority root.
	for i, v := range signedCommitment.Signatures {
		if v.IsSome() {
			_, sig := v.Unwrap()
			signatures = append(signatures, &beefyclienttypes.CommitmentSignature{
				Signature:      sig[:],
				AuthorityIndex: uint32(i),
			})
			authorityIndices = append(authorityIndices, uint64(i))
		}
	}

	authorities, err := sp.beefyAuthorities(uint32(signedCommitment.Commitment.BlockNumber), methodAuthorities)
	if err != nil {
		return nil, err
	}

	var authorityLeaves [][]byte
	for _, v := range authorities {
		authorityLeaves = append(authorityLeaves, crypto.Keccak256(v))
	}
	authorityTree, err := merkle.NewTree(hasher.Keccak256Hasher{}).FromLeaves(authorityLeaves)
	if err != nil {
		return nil, err
	}

	var payloadId beefyclienttypes.SizedByte2 = commitmentPayload.ID
	return &beefyclienttypes.MMRUpdateProof{
		LatestMMRLeaf: &beefyclienttypes.BeefyMMRLeaf{
			Version:        beefyclienttypes.U8(latestLeaf.Version),
			ParentNumber:   uint32(latestLeaf.ParentNumberAndHash.ParentNumber),
			ParentHash:     &parentHash,
			ParachainHeads: &parachainHeads,
			BeefyNextAuthoritySet: beefyclienttypes.BeefyAuthoritySet{
				ID:            uint64(latestLeaf.BeefyNextAuthoritySet.ID),
				Len:           uint32(latestLeaf.BeefyNextAuthoritySet.Len),
				AuthorityRoot: &beefyNextAuthoritySetRoot,
			},
		},
		LatestMMRLeafIndex: leafIndex,
		MMRProof:           latestLeafMmrProof,
		SignedCommitment: &beefyclienttypes.SignedCommitment{
			Commitment: &beefyclienttypes.Commitment{
				Payload: []*beefyclienttypes.Payload{
					{PayloadID: &payloadId, PayloadData: commitmentPayload.Value},
				},
				BlockNumber:    uint32(signedCommitment.Commitment.BlockNumber),
				ValidatorSetID: uint64(signedCommitment.Commitment.ValidatorSetID),
			},
			Signatures: signatures,
		},
		AuthoritiesProof: authorityTree.Proof(authorityIndices).ProofHashes(),
	}, nil
}

func (sp *SubstrateProvider) constructBeefyHeader(
	blockHash rpcclienttypes.Hash,
	previousFinalizedHash *rpcclienttypes.Hash,
) (*beefyclienttypes.Header, error) {
	// assuming blockHash is always the latest beefy block hash
	// TODO: check that it is the latest block hash
	latestCommitment, err := sp.signedCommitment(blockHash)
	if err != nil {
		return nil, err
	}

	parachainHeads, err := sp.constructParachainHeaders(blockHash, previousFinalizedHash)
	if err != nil {
		return nil, err
	}

	batchProofs, err := sp.mmrBatchProofs(blockHash, previousFinalizedHash)
	if err != nil {
		return nil, err
	}

	leafIndex := getLeafIndexForBlockNumber(sp.Config.BeefyActivationBlock, uint32(latestCommitment.Commitment.BlockNumber))
	blockNumber := uint32(latestCommitment.Commitment.BlockNumber)
	mmrProof, err := sp.mmrUpdateProof(blockHash, latestCommitment,
		uint64(getLeafIndexForBlockNumber(sp.Config.BeefyActivationBlock, blockNumber)))
	if err != nil {
		return nil, err
	}

	return &beefyclienttypes.Header{
		HeadersWithProof: &beefyclienttypes.ParachainHeadersWithProof{
			Headers:   parachainHeads,
			MMRProofs: mmrBatchProofItems(batchProofs),
			MMRSize:   mmr.LeafIndexToMMRSize(uint64(leafIndex)),
		},
		MMRUpdateProof: mmrProof,
	}, nil
}

func mmrBatchProofItems(mmrBatchProof rpcclienttypes.GenerateMmrBatchProofResponse) [][]byte {
	var proofItems = make([][]byte, len(mmrBatchProof.Proof.Items))
	for i := 0; i < len(mmrBatchProof.Proof.Items); i++ {
		proofItems[i] = mmrBatchProof.Proof.Items[i][:]
	}
	return proofItems
}

func getBlockNumberForLeaf(beefyActivationBlock, leafIndex uint32) uint32 {
	var blockNumber uint32

	// calculate the leafIndex for this leaf.
	if beefyActivationBlock == 0 {
		// in this case the leaf index is the same as the block number - 1 (leaf index starts at 0)
		blockNumber = leafIndex + 1
	} else {
		// in this case the leaf index is activation block - current block number.
		blockNumber = beefyActivationBlock + leafIndex
	}

	return blockNumber
}

// GetLeafIndexForBlockNumber given the MmrLeafPartial.ParentNumber & BeefyActivationBlock,
func getLeafIndexForBlockNumber(beefyActivationBlock, blockNumber uint32) uint32 {
	var leafIndex uint32

	// calculate the leafIndex for this leaf.
	if beefyActivationBlock == 0 {
		// in this case the leaf index is the same as the block number - 1 (leaf index starts at 0)
		leafIndex = blockNumber - 1
	} else {
		// in this case the leaf index is activation block - current block number.
		leafIndex = beefyActivationBlock - (blockNumber + 1)
	}

	return leafIndex
}

func bytes32(bytes []byte) beefyclienttypes.SizedByte32 {
	var buffer beefyclienttypes.SizedByte32
	copy(buffer[:], bytes)
	return buffer
}
