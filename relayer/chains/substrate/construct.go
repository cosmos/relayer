package substrate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/ChainSafe/chaindb"
	"github.com/ChainSafe/gossamer/lib/trie"

	"github.com/ComposableFi/go-merkle-trees/mmr"

	hasher "github.com/ComposableFi/go-merkle-trees/hasher"
	"github.com/ComposableFi/go-merkle-trees/merkle"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	"github.com/ComposableFi/go-substrate-rpc-client/v4/xxhash"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type Authorities = [][33]uint8

func fetchParaIds(conn *rpcclient.SubstrateAPI, blockHash rpcclienttypes.Hash) ([]uint32, error) {
	// Fetch metadata
	meta, err := conn.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, "Paras", "Parachains", nil, nil)
	if err != nil {
		return nil, err
	}

	var paraIds []uint32

	ok, err := conn.RPC.State.GetStorage(storageKey, &paraIds, blockHash)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("Beefy authorities not found")
	}

	return paraIds, nil
}

func (sp *SubstrateProvider) parachainHeaderKey() ([]byte, error) {
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix("Paras", "Heads")
	encodedParaId, err := rpcclienttypes.Encode(sp.PCfg.ParaID)
	if err != nil {
		return nil, err
	}

	twoxhash := xxhash.New64(encodedParaId).Sum(nil)
	fullKey := append(append(keyPrefix, twoxhash[:]...), encodedParaId...)
	return fullKey, nil
}

func (sp *SubstrateProvider) paraHeadData(conn *rpcclient.SubstrateAPI, blockHash rpcclienttypes.Hash) ([]byte, error) {
	paraKey, err := sp.parachainHeaderKey()
	if err != nil {
		return nil, err
	}

	storage, err := conn.RPC.State.GetStorageRaw(paraKey, blockHash)
	if err != nil {
		return nil, err
	}

	return *storage, nil
}

func (sp *SubstrateProvider) clientState(
	conn *rpcclient.SubstrateAPI,
	commitment rpcclienttypes.SignedCommitment,
) (*beefyclienttypes.ClientState, error) {
	blockNumber := uint32(commitment.Commitment.BlockNumber)
	authorities, err := beefyAuthorities(blockNumber, conn, "Authorities")
	if err != nil {
		return nil, err
	}

	nextAuthorities, err := beefyAuthorities(blockNumber, conn, "NextAuthorities")
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

	blockHash, err := conn.RPC.Chain.GetBlockHash(uint64(blockNumber))
	if err != nil {
		return nil, err
	}

	headData, err := sp.paraHeadData(conn, blockHash)
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
		ParaID:           sp.PCfg.ParaID,
		LatestParaHeight: uint32(paraHead.Number),
		// TODO: add relaychain to config
		RelayChain: beefyclienttypes.RelayChain_KUSAMA,
	}, nil
}

func beefyAuthorities(blockNumber uint32, conn *rpcclient.SubstrateAPI, method string) ([][]byte, error) {
	blockHash, err := conn.RPC.Chain.GetBlockHash(uint64(blockNumber))
	if err != nil {
		return nil, err
	}

	// Fetch metadata
	meta, err := conn.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, "Beefy", method, nil, nil)
	if err != nil {
		return nil, err
	}

	var authorities Authorities

	ok, err := conn.RPC.State.GetStorage(storageKey, &authorities, blockHash)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("Beefy authorities not found")
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

func bytes32(bytes []byte) beefyclienttypes.SizedByte32 {
	var buffer beefyclienttypes.SizedByte32
	copy(buffer[:], bytes)
	return buffer
}

func signedCommitment(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclienttypes.Hash,
) (rpcclienttypes.SignedCommitment, error) {
	signedBlock, err := conn.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return rpcclienttypes.SignedCommitment{}, err
	}

	for _, v := range signedBlock.Justifications {
		if bytes.Equal(v.ConsensusEngineID[:], []byte("BEEF")) {
			versionedFinalityProof := &rpcclienttypes.VersionedFinalityProof{}

			err = rpcclienttypes.Decode(v.EncodedJustification, versionedFinalityProof)
			return versionedFinalityProof.AsCompactSignedCommitment.Unpack(), nil
		}
	}

	return rpcclienttypes.SignedCommitment{}, nil
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

// finalized block returns the finalized block double map that holds block numbers,
// for which our parachain header was included in the mmr leaf, seeing as our parachain
// headers might not make it into every relay chain block. Map<BlockNumber, Map<ParaId, Header>>
// It also returns the leaf indeces of those blocks
func (sp *SubstrateProvider) getFinalizedBlocks(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclienttypes.Hash,
	previouslyFinalizedBlockHash *rpcclienttypes.Hash,
) (map[uint32]map[uint32][]byte, []uint64, error) {
	var finalizedBlocks = make(map[uint32]map[uint32][]byte)
	var leafIndeces []uint64

	if previouslyFinalizedBlockHash == nil {
		var heads = make(map[uint32][]byte)
		headData, err := sp.paraHeadData(conn, blockHash)
		if err != nil {
			return nil, nil, err
		}

		paraHead, err := beefyclienttypes.DecodeParachainHeader(headData)
		if err != nil {
			return nil, nil, err
		}

		heads[sp.PCfg.ParaID] = headData
		finalizedBlocks[uint32(paraHead.Number)] = heads
		leafIndeces = append(leafIndeces, uint64(getLeafIndexForBlockNumber(sp.PCfg.BeefyActivationBlock,
			uint32(paraHead.Number))))
		return finalizedBlocks, leafIndeces, nil
	}

	paraHeaderKeys, err := parachainHeaderKeys(conn, blockHash)
	if err != nil {
		return nil, nil, err
	}

	changeSet, err := conn.RPC.State.QueryStorage(paraHeaderKeys, *previouslyFinalizedBlockHash, blockHash)
	if err != nil {
		return nil, nil, err
	}

	for _, changes := range changeSet {
		header, err := conn.RPC.Chain.GetHeader(changes.Block)
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
		if heads[sp.PCfg.ParaID] == nil {
			continue
		}

		finalizedBlocks[uint32(header.Number)] = heads

		leafIndeces = append(leafIndeces, uint64(getLeafIndexForBlockNumber(sp.PCfg.BeefyActivationBlock,
			uint32(header.Number))))
	}
	return finalizedBlocks, leafIndeces, nil
}

func parachainHeaderKeys(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclienttypes.Hash,
) ([]rpcclienttypes.StorageKey, error) {
	paraIds, err := fetchParaIds(conn, blockHash)
	if err != nil {
		return nil, err
	}

	var paraHeaderKeys []rpcclienttypes.StorageKey
	// create full storage key for each known paraId.
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix("Paras", "Heads")
	// so we can query all blocks from lastfinalized to latestBeefyHeight
	for _, paraId := range paraIds {
		encodedParaId, err := rpcclienttypes.Encode(paraId)
		if err != nil {
			panic(err)
		}
		twoxhash := xxhash.New64(encodedParaId).Sum(nil)
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
	var conn = sp.RelayerRPCClient
	var finalizedBlocks = make(map[uint32]map[uint32][]byte)
	var leafIndeces []uint64
	finalizedBlocks, leafIndeces, err := sp.getFinalizedBlocks(conn, blockHash, previouslyFinalizedBlockHash)
	if err != nil {
		return nil, err
	}

	// fetch mmr proofs for leaves containing our target paraId
	mmrBatchProof, err := conn.RPC.MMR.GenerateBatchProof(leafIndeces, blockHash)
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
		var leafBlockNumber = getBlockNumberForLeaf(sp.PCfg.BeefyActivationBlock, uint32(leafIndex))
		paraHeaders := finalizedBlocks[leafBlockNumber]

		var paraHeadsLeaves [][]byte
		// index of our parachain header in the
		// parachain heads merkle root
		var index uint64

		count := 0

		// sort by paraId
		var sortedParaIds []uint32
		for paraId, _ := range paraHeaders {
			sortedParaIds = append(sortedParaIds, paraId)
		}
		sort.SliceStable(sortedParaIds, func(i, j int) bool {
			return sortedParaIds[i] < sortedParaIds[j]
		})

		for _, paraId := range sortedParaIds {
			paraIdScale := make([]byte, 4)
			// scale encode para_id
			binary.LittleEndian.PutUint32(paraIdScale[:], paraId)
			leaf := append(paraIdScale, paraHeaders[paraId]...)
			paraHeadsLeaves = append(paraHeadsLeaves, crypto.Keccak256(leaf))
			if paraId == sp.PCfg.ParaID {
				// note index of paraId
				index = uint64(count)
			}
			count++
		}

		tree, err := merkle.NewTree(hasher.Keccak256Hasher{}).FromLeaves(paraHeadsLeaves)
		if err != nil {
			panic(err)
		}
		paraHeadsProof := tree.Proof([]uint64{index})
		authorityRoot := bytes32(v.BeefyNextAuthoritySet.Root[:])
		parentHash := bytes32(v.ParentNumberAndHash.Hash[:])

		parachainHeaderDecoded, err := beefyclienttypes.DecodeParachainHeader(paraHeaders[sp.PCfg.ParaID])
		if err != nil {
			return nil, err
		}

		timestampExt, extProof, err := sp.constructExtrinsics(uint32(parachainHeaderDecoded.Number))
		if err != nil {
			return nil, err
		}

		header := beefyclienttypes.ParachainHeader{
			ParachainHeader: paraHeaders[sp.PCfg.ParaID],
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

	timestampExtrinsic, err = rpcclienttypes.Encode(exts[0])
	if err != nil {
		return nil, nil, err
	}

	t := trie.NewEmptyTrie()
	for i := 0; i < len(exts); i++ {
		ext, err := rpcclienttypes.Encode(exts[i])
		if err != nil {
			return nil, nil, err
		}

		key := rpcclienttypes.NewUCompactFromUInt(uint64(i))
		encodedKey, err := rpcclienttypes.Encode(key)
		if err != nil {
			return nil, nil, err
		}

		t.Put(encodedKey, ext)
	}

	memdb, err := chaindb.NewBadgerDB(&chaindb.Config{
		InMemory: true,
		DataDir:  "./",
	})

	err = t.Store(memdb)
	if err != nil {
		return nil, nil, err
	}

	rootHash, err := t.Hash()
	if err != nil {
		return nil, nil, err
	}

	timestampKey := rpcclienttypes.NewUCompactFromUInt(uint64(0))
	encodedTPKey, err := rpcclienttypes.Encode(timestampKey)
	if err != nil {
		return nil, nil, err
	}
	extrinsicProof, err = trie.GenerateProof(rootHash.ToBytes(), [][]byte{encodedTPKey}, memdb)
	if err != nil {
		return nil, nil, err
	}

	return
}

func (sp *SubstrateProvider) mmrBatchProofs(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclienttypes.Hash,
	previouslyFinalizedBlockHash *rpcclienttypes.Hash,
) (rpcclienttypes.GenerateMmrBatchProofResponse, error) {
	var leafIndeces []uint64
	_, leafIndeces, err := sp.getFinalizedBlocks(conn, blockHash, previouslyFinalizedBlockHash)
	if err != nil {
		return rpcclienttypes.GenerateMmrBatchProofResponse{}, err
	}

	// fetch mmr proofs for leaves containing our target paraId
	batchProofs, err := conn.RPC.MMR.GenerateBatchProof(leafIndeces, blockHash)
	if err != nil {
		return rpcclienttypes.GenerateMmrBatchProofResponse{}, err
	}

	return batchProofs, nil
}

func mmrBatchProofItems(mmrBatchProof rpcclienttypes.GenerateMmrBatchProofResponse) [][]byte {
	var proofItems = make([][]byte, len(mmrBatchProof.Proof.Items))
	for i := 0; i < len(mmrBatchProof.Proof.Items); i++ {
		proofItems[i] = mmrBatchProof.Proof.Items[i][:]
	}
	return proofItems
}

func mmrUpdateProof(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclienttypes.Hash,
	signedCommitment rpcclienttypes.SignedCommitment,
	leafIndex uint64,
) (*beefyclienttypes.MMRUpdateProof, error) {
	mmrProof, err := conn.RPC.MMR.GenerateProof(
		leafIndex,
		blockHash,
	)
	if err != nil {
		return nil, err
	}

	latestLeaf := mmrProof.Leaf
	parentHash := bytes32(latestLeaf.ParentNumberAndHash.Hash[:])
	ParachainHeads := bytes32(latestLeaf.ParachainHeads[:])
	BeefyNextAuthoritySetRoot := bytes32(latestLeaf.BeefyNextAuthoritySet.Root[:])
	CommitmentPayload := signedCommitment.Commitment.Payload[0]

	var latestLeafMmrProof = make([][]byte, len(mmrProof.Proof.Items))
	for i := 0; i < len(mmrProof.Proof.Items); i++ {
		latestLeafMmrProof[i] = mmrProof.Proof.Items[i][:]
	}

	var signatures []*beefyclienttypes.CommitmentSignature
	var authorityIndeces []uint64
	// luckily for us, this is already sorted and maps to the right authority index in the authority root.
	for i, v := range signedCommitment.Signatures {
		if v.IsSome() {
			_, sig := v.Unwrap()
			signatures = append(signatures, &beefyclienttypes.CommitmentSignature{
				Signature:      sig[:],
				AuthorityIndex: uint32(i),
			})
			authorityIndeces = append(authorityIndeces, uint64(i))
		}
	}

	authorities, err := beefyAuthorities(uint32(signedCommitment.Commitment.BlockNumber), conn, "Authorities")
	if err != nil {
		panic(err)
	}

	var authorityLeaves [][]byte
	for _, v := range authorities {
		authorityLeaves = append(authorityLeaves, crypto.Keccak256(v))
	}
	authorityTree, err := merkle.NewTree(hasher.Keccak256Hasher{}).FromLeaves(authorityLeaves)
	if err != nil {
		panic(err)
	}

	var payloadId beefyclienttypes.SizedByte2 = CommitmentPayload.ID
	return &beefyclienttypes.MMRUpdateProof{
		LatestMMRLeaf: &beefyclienttypes.BeefyMMRLeaf{
			Version:        beefyclienttypes.U8(latestLeaf.Version),
			ParentNumber:   uint32(latestLeaf.ParentNumberAndHash.ParentNumber),
			ParentHash:     &parentHash,
			ParachainHeads: &ParachainHeads,
			BeefyNextAuthoritySet: beefyclienttypes.BeefyAuthoritySet{
				ID:            uint64(latestLeaf.BeefyNextAuthoritySet.ID),
				Len:           uint32(latestLeaf.BeefyNextAuthoritySet.Len),
				AuthorityRoot: &BeefyNextAuthoritySetRoot,
			},
		},
		LatestMMRLeafIndex: leafIndex,
		MMRProof:           latestLeafMmrProof,
		SignedCommitment: &beefyclienttypes.SignedCommitment{
			Commitment: &beefyclienttypes.Commitment{
				Payload: []*beefyclienttypes.Payload{
					{PayloadID: &payloadId, PayloadData: CommitmentPayload.Value},
				},
				BlockNumber:    uint32(signedCommitment.Commitment.BlockNumber),
				ValidatorSetID: uint64(signedCommitment.Commitment.ValidatorSetID),
			},
			Signatures: signatures,
		},
		AuthoritiesProof: authorityTree.Proof(authorityIndeces).ProofHashes(),
	}, nil
}

func (sp *SubstrateProvider) constructBeefyHeader(
	blockHash rpcclienttypes.Hash,
	previousFinalizedHash *rpcclienttypes.Hash,
) (*beefyclienttypes.Header, error) {
	var conn = sp.RelayerRPCClient
	// assuming blockHash is always the latest beefy block hash
	// TODO: check that it is the latest block hash
	latestCommitment, err := signedCommitment(conn, blockHash)
	if err != nil {
		return nil, err
	}

	parachainHeads, err := sp.constructParachainHeaders(blockHash, previousFinalizedHash)
	if err != nil {
		return nil, err
	}

	batchProofs, err := sp.mmrBatchProofs(conn, blockHash, previousFinalizedHash)
	if err != nil {
		return nil, err
	}

	leafIndex := getLeafIndexForBlockNumber(sp.PCfg.BeefyActivationBlock, uint32(latestCommitment.Commitment.BlockNumber))
	blockNumber := uint32(latestCommitment.Commitment.BlockNumber)
	mmrProof, err := mmrUpdateProof(conn, blockHash, latestCommitment,
		uint64(getLeafIndexForBlockNumber(sp.PCfg.BeefyActivationBlock, blockNumber)))
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
