package substrate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/ChainSafe/chaindb"
	"github.com/ChainSafe/gossamer/lib/trie"
	"sort"

	"github.com/ComposableFi/go-merkle-trees/mmr"

	hasher "github.com/ComposableFi/go-merkle-trees/hasher"
	"github.com/ComposableFi/go-merkle-trees/merkle"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	"github.com/ComposableFi/go-substrate-rpc-client/v4/xxhash"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const PARA_ID = 2000

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

func clientState(
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
		panic(err)
	}

	var authorityTreeRoot = bytes32(authorityTree.Root())
	var nextAuthorityTreeRoot = bytes32(nextAuthorityTree.Root())
	return &beefyclienttypes.ClientState{
		MmrRootHash:          commitment.Commitment.Payload[0].Value,
		LatestBeefyHeight:    blockNumber,
		BeefyActivationBlock: 0,
		Authority: &beefyclienttypes.BeefyAuthoritySet{
			Id:            uint64(commitment.Commitment.ValidatorSetID),
			Len:           uint32(len(authorities)),
			AuthorityRoot: &authorityTreeRoot,
		},
		NextAuthoritySet: &beefyclienttypes.BeefyAuthoritySet{
			Id:            uint64(commitment.Commitment.ValidatorSetID) + 1,
			Len:           uint32(len(nextAuthorities)),
			AuthorityRoot: &nextAuthorityTreeRoot,
		},
		ParaId: PARA_ID,
		// TODO: pass a defined para chain height
		LatestParaHeight: 0,
		// TODO: add relaychain to config
		RelayChain: beefyclienttypes.RelayChain_KUSAMA,
	}, nil
}

// 	// Latest mmr root hash
//	MmrRootHash []byte `protobuf:"bytes,1,opt,name=mmr_root_hash,json=mmrRootHash,proto3" json:"mmr_root_hash,omitempty"`
//	// block number for the latest mmr_root_hash
//	LatestBeefyHeight uint32 `protobuf:"varint,2,opt,name=latest_beefy_height,json=latestBeefyHeight,proto3" json:"latest_beefy_height,omitempty"`
//	// Block height when the client was frozen due to a misbehaviour
//	FrozenHeight uint64 `protobuf:"varint,3,opt,name=frozen_height,json=frozenHeight,proto3" json:"frozen_height,omitempty"`
//	/// Known relay chains
//	RelayChain RelayChain `protobuf:"varint,4,opt,name=relay_chain,json=relayChain,proto3,enum=ibc.lightclients.beefy.v1.RelayChain" json:"relay_chain,omitempty"`
//	/// ParaId of associated parachain
//	ParaId uint32 `protobuf:"varint,5,opt,name=para_id,json=paraId,proto3" json:"para_id,omitempty"`
//	/// latest parachain height
//	LatestParaHeight uint32 `protobuf:"varint,6,opt,name=latest_para_height,json=latestParaHeight,proto3" json:"latest_para_height,omitempty"`
//	// block number that the beefy protocol was activated on the relay chain.
//	// This should be the first block in the merkle-mountain-range tree.
//	BeefyActivationBlock uint32 `protobuf:"varint,7,opt,name=beefy_activation_block,json=beefyActivationBlock,proto3" json:"beefy_activation_block,omitempty"`
//	// authorities for the current round
//	Authority *BeefyAuthoritySet `protobuf:"bytes,8,opt,name=authority,proto3" json:"authority,omitempty"`
//	// authorities for the next round
//	NextAuthoritySet *BeefyAuthoritySet `protobuf:"bytes,9,opt,name=next_authority_set,json=nextAuthoritySet,proto3" json:"next_authority_set,omitempty"`

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

func signedCommitment(conn *rpcclient.SubstrateAPI, blockHash rpcclienttypes.Hash) (rpcclienttypes.SignedCommitment, error) {
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

// finalized block returns the finalized block double map that holds block numbers,
// for which our parachain header was included in the mmr leaf, seeing as our parachain
// headers might not make it into every relay chain block. Map<BlockNumber, Map<ParaId, Header>>
// It also returns the leaf indeces of those blocks
func getFinalizedBlocks(
	conn *rpcclient.SubstrateAPI,
	cs *beefyclienttypes.ClientState,
	blockHash rpcclienttypes.Hash,
) (map[uint32]map[uint32][]byte, []uint64, error) {

	previousFinalizedHash, err := conn.RPC.Chain.GetBlockHash(uint64(cs.LatestBeefyHeight + 1))
	if err != nil {
		return nil, nil, err
	}

	var paraHeaderKeys []rpcclienttypes.StorageKey
	paraHeaderKeys, err = parachainHeaderKeys(conn, blockHash)
	if err != nil {
		return nil, nil, err
	}

	changeSet, err := conn.RPC.State.QueryStorage(paraHeaderKeys, previousFinalizedHash, blockHash)
	if err != nil {
		return nil, nil, err
	}

	var finalizedBlocks = make(map[uint32]map[uint32][]byte)

	// request for batch mmr proof of those leaves
	var leafIndeces []uint64

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
		if heads[PARA_ID] == nil {
			continue
		}

		finalizedBlocks[uint32(header.Number)] = heads

		leafIndeces = append(leafIndeces, uint64(cs.GetLeafIndexForBlockNumber(uint32(header.Number))))
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
		encodedParaId, err := Encode(paraId)
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
	cs *beefyclienttypes.ClientState,
) ([]*beefyclienttypes.ParachainHeader, error) {
	var conn = sp.RelayerRPCClient
	var finalizedBlocks = make(map[uint32]map[uint32][]byte)
	var leafIndeces []uint64
	finalizedBlocks, leafIndeces, err := getFinalizedBlocks(conn, cs, blockHash)
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
		var leafBlockNumber = cs.GetBlockNumberForLeaf(uint32(leafIndex))
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
			if paraId == PARA_ID {
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

		parachainHeaderDecoded, err := beefyclienttypes.DecodeParachainHeader(paraHeaders[PARA_ID])
		if err != nil {
			return nil, err
		}

		timestampExt, extProof, err := sp.constructExtrinsics(uint32(parachainHeaderDecoded.Number))
		if err != nil {
			return nil, err
		}

		header := beefyclienttypes.ParachainHeader{
			ParachainHeader: paraHeaders[PARA_ID],
			MmrLeafPartial: &beefyclienttypes.BeefyMmrLeafPartial{
				Version:      beefyclienttypes.U8(v.Version),
				ParentNumber: uint32(v.ParentNumberAndHash.ParentNumber),
				ParentHash:   &parentHash,
				BeefyNextAuthoritySet: beefyclienttypes.BeefyAuthoritySet{
					Id:            uint64(v.BeefyNextAuthoritySet.ID),
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
	encodedTPKey, err := Encode(timestampKey)
	if err != nil {
		return nil, nil, err
	}
	extrinsicProof, err = trie.GenerateProof(rootHash.ToBytes(), [][]byte{encodedTPKey}, memdb)
	if err != nil {
		return nil, nil, err
	}

	return
}

func mmrBatchProofs(
	conn *rpcclient.SubstrateAPI,
	cs *beefyclienttypes.ClientState,
	blockHash rpcclienttypes.Hash,
	commitment rpcclienttypes.SignedCommitment,
) (rpcclienttypes.GenerateMmrBatchProofResponse, error) {
	var leafIndeces []uint64
	_, leafIndeces, err := getFinalizedBlocks(conn, cs, blockHash)
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

func clientStateUpdateProof(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclienttypes.Hash,
	signedCommitment rpcclienttypes.SignedCommitment,
	leafIndex uint64,
) (*beefyclienttypes.ClientStateUpdateProof, error) {
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
	return &beefyclienttypes.ClientStateUpdateProof{
		MmrLeaf: &beefyclienttypes.BeefyMmrLeaf{
			Version:        beefyclienttypes.U8(latestLeaf.Version),
			ParentNumber:   uint32(latestLeaf.ParentNumberAndHash.ParentNumber),
			ParentHash:     &parentHash,
			ParachainHeads: &ParachainHeads,
			BeefyNextAuthoritySet: beefyclienttypes.BeefyAuthoritySet{
				Id:            uint64(latestLeaf.BeefyNextAuthoritySet.ID),
				Len:           uint32(latestLeaf.BeefyNextAuthoritySet.Len),
				AuthorityRoot: &BeefyNextAuthoritySetRoot,
			},
		},
		MmrLeafIndex: leafIndex,
		MmrProof:     latestLeafMmrProof,
		SignedCommitment: &beefyclienttypes.SignedCommitment{
			Commitment: &beefyclienttypes.Commitment{
				Payload: []*beefyclienttypes.PayloadItem{
					{PayloadId: &payloadId, PayloadData: CommitmentPayload.Value},
				},
				BlockNumer:     uint32(signedCommitment.Commitment.BlockNumber),
				ValidatorSetId: uint64(signedCommitment.Commitment.ValidatorSetID),
			},
			Signatures: signatures,
		},
		AuthoritiesProof: authorityTree.Proof(authorityIndeces).ProofHashes(),
	}, nil
}

// TODO: there's probably a more efficient way of getting the previously finalized block
func previouslyFinalizedBlock(conn *rpcclient.SubstrateAPI, blockNumber uint64) (rpcclienttypes.Hash, uint64, error) {
	var previousBlock = blockNumber
	var previousHash rpcclienttypes.Hash
	for {
		previousBlock = previousBlock - 1
		blockHash, err := conn.RPC.Chain.GetBlockHash(previousBlock)
		if err != nil {
			return rpcclienttypes.Hash{}, 0, err
		}
		previousHash = blockHash

		commitment, err := signedCommitment(conn, blockHash)
		if err != nil {
			return rpcclienttypes.Hash{}, 0, err
		}

		if commitment.Commitment.BlockNumber == 0 {
			continue
		}

		return previousHash, previousBlock, nil
	}
}

func (sp *SubstrateProvider) constructBeefyHeader(blockHash rpcclienttypes.Hash) (*beefyclienttypes.Header, error) {
	var conn = sp.RelayerRPCClient
	// assuming blockHash is always the latest beefy block hash
	latestFinalizedBlock, err := conn.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return nil, err
	}

	pHash, _, err := previouslyFinalizedBlock(conn, uint64(latestFinalizedBlock.Block.Header.Number))
	if err != nil {
		return nil, err
	}

	commitment, err := signedCommitment(conn, pHash)
	if err != nil {
		return nil, err
	}

	cs, err := clientState(conn, commitment)
	if err != nil {
		return nil, err
	}

	latestCommitment, err := signedCommitment(conn, blockHash)
	if err != nil {
		return nil, err
	}

	parachainHeads, err := sp.constructParachainHeaders(blockHash, cs)
	if err != nil {
		return nil, err
	}

	batchProofs, err := mmrBatchProofs(conn, cs, blockHash, latestCommitment)
	if err != nil {
		return nil, err
	}

	leafIndex := cs.GetLeafIndexForBlockNumber(uint32(latestCommitment.Commitment.BlockNumber))
	blockNumber := uint32(latestCommitment.Commitment.BlockNumber)
	csUpdateProof, err := clientStateUpdateProof(conn, blockHash, latestCommitment, uint64(cs.GetLeafIndexForBlockNumber(blockNumber)))
	if err != nil {
		return nil, err
	}

	return &beefyclienttypes.Header{
		ConsensusStateUpdate: &beefyclienttypes.ConsensusStateUpdateProof{
			ParachainHeaders: parachainHeads,
			MmrProofs:        mmrBatchProofItems(batchProofs),
			MmrSize:          mmr.LeafIndexToMMRSize(uint64(leafIndex)),
		},
		ClientState: csUpdateProof,
	}, nil
}
