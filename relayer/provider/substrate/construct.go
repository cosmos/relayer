package substrate

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/ComposableFi/go-merkle-trees/mmr"

	"github.com/ComposableFi/go-merkle-trees/merkle"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	rpcclientTypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	"github.com/ComposableFi/go-substrate-rpc-client/v4/xxhash"
	beefyClientTypes "github.com/cosmos/ibc-go/v3/modules/light-clients/11-beefy/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const PARA_ID = 2000

type Authorities = [][33]uint8

func fetchParaIds(conn *rpcclient.SubstrateAPI, blockHash rpcclientTypes.Hash) ([]uint32, error) {
	// Fetch metadata
	meta, err := conn.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	storageKey, err := rpcclientTypes.CreateStorageKey(meta, "Paras", "Parachains", nil, nil)
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
	commitment rpcclientTypes.SignedCommitment,
) (*beefyClientTypes.ClientState, error) {
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

	authorityTree, err := merkle.NewTree(beefyClientTypes.Keccak256{}).FromLeaves(authorityLeaves)
	if err != nil {
		return nil, err
	}

	var nextAuthorityLeaves [][]byte
	for _, v := range nextAuthorities {
		nextAuthorityLeaves = append(nextAuthorityLeaves, crypto.Keccak256(v))
	}

	nextAuthorityTree, err := merkle.NewTree(beefyClientTypes.Keccak256{}).FromLeaves(nextAuthorityLeaves)
	if err != nil {
		panic(err)
	}

	var authorityTreeRoot = bytes32(authorityTree.Root())
	var nextAuthorityTreeRoot = bytes32(nextAuthorityTree.Root())
	clientState := &beefyClientTypes.ClientState{
		MmrRootHash:          commitment.Commitment.Payload[0].Value,
		LatestBeefyHeight:    blockNumber,
		BeefyActivationBlock: 0,
		Authority: &beefyClientTypes.BeefyAuthoritySet{
			Id:            uint64(commitment.Commitment.ValidatorSetID),
			Len:           uint32(len(authorities)),
			AuthorityRoot: &authorityTreeRoot,
		},
		NextAuthoritySet: &beefyClientTypes.BeefyAuthoritySet{
			Id:            uint64(commitment.Commitment.ValidatorSetID) + 1,
			Len:           uint32(len(nextAuthorities)),
			AuthorityRoot: &nextAuthorityTreeRoot,
		},
	}

	return clientState, err
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

	storageKey, err := rpcclientTypes.CreateStorageKey(meta, "Beefy", method, nil, nil)
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

func bytes32(bytes []byte) [32]byte {
	var buffer [32]byte
	copy(buffer[:], bytes)
	return buffer
}

func signedCommitment(conn *rpcclient.SubstrateAPI, blockHash rpcclientTypes.Hash) (rpcclientTypes.SignedCommitment, error) {
	signedBlock, err := conn.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return rpcclientTypes.SignedCommitment{}, err
	}

	//TODO: add test for this -> https://github.com/ComposableFi/relayer/issues/7
	compactCommitment := &rpcclientTypes.CompactSignedCommitment{}
	err = rpcclientTypes.DecodeFromHexString(string(signedBlock.Justification), compactCommitment)
	if err != nil {
		return rpcclientTypes.SignedCommitment{}, err
	}

	return compactCommitment.Unpack(), nil
}

// finalized block returns the finalized block double map that holds block numbers,
// for which our parachain header was included in the mmr leaf, seeing as our parachain
// headers might not make it into every relay chain block. Map<BlockNumber, Map<ParaId, Header>>
// It also returns the leaf indeces of those blocks
func getFinalizedBlocks(
	conn *rpcclient.SubstrateAPI,
	cs *beefyClientTypes.ClientState,
	blockHash rpcclientTypes.Hash,
) (map[uint32]map[uint32][]byte, []uint64, error) {

	previousFinalizedHash, err := conn.RPC.Chain.GetBlockHash(uint64(cs.LatestBeefyHeight + 1))
	if err != nil {
		return nil, nil, err
	}

	var paraHeaderKeys []rpcclientTypes.StorageKey
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
				err = beefyClientTypes.DecodeFromBytes(keyValue.StorageKey[40:], &paraId)
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
	blockHash rpcclientTypes.Hash,
) ([]rpcclientTypes.StorageKey, error) {
	paraIds, err := fetchParaIds(conn, blockHash)
	if err != nil {
		return nil, err
	}

	var paraHeaderKeys []rpcclientTypes.StorageKey
	// create full storage key for each known paraId.
	keyPrefix := rpcclientTypes.CreateStorageKeyPrefix("Paras", "Heads")
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

func constructParachainHeaders(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclientTypes.Hash,
	cs *beefyClientTypes.ClientState,
) ([]*beefyClientTypes.ParachainHeader, error) {
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

	var parachainHeaders []*beefyClientTypes.ParachainHeader

	var paraHeads = make([][]byte, len(mmrBatchProof.Leaves))

	for i := 0; i < len(mmrBatchProof.Leaves); i++ {
		v := mmrBatchProof.Leaves[i]
		paraHeads[i] = v.Leaf.ParachainHeads[:]
		var leafBlockNumber = cs.GetBlockNumberForLeaf(uint32(v.Index))
		paraHeaders := finalizedBlocks[leafBlockNumber]

		var paraHeadsLeaves [][]byte
		// index of our parachain header in the
		// parachain heads merkle root
		var index uint32

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
				index = uint32(count)
			}
			count++
		}

		tree, err := merkle.NewTree(beefyClientTypes.Keccak256{}).FromLeaves(paraHeadsLeaves)
		if err != nil {
			panic(err)
		}
		paraHeadsProof := tree.Proof([]uint32{index})
		authorityRoot := bytes32(v.Leaf.BeefyNextAuthoritySet.Root[:])
		parentHash := bytes32(v.Leaf.ParentNumberAndHash.Hash[:])

		header := beefyClientTypes.ParachainHeader{
			ParachainHeader: paraHeaders[PARA_ID],
			MmrLeafPartial: &beefyClientTypes.BeefyMmrLeafPartial{
				Version:      v.Leaf.Version,
				ParentNumber: v.Leaf.ParentNumberAndHash.ParentNumber,
				ParentHash:   &parentHash,
				BeefyNextAuthoritySet: beefyClientTypes.BeefyAuthoritySet{
					Id:            v.Leaf.BeefyNextAuthoritySet.ID,
					Len:           v.Leaf.BeefyNextAuthoritySet.Len,
					AuthorityRoot: &authorityRoot,
				},
			},
			ParachainHeadsProof: paraHeadsProof.ProofHashes(),
			ParaId:              PARA_ID,
			HeadsLeafIndex:      index,
			HeadsTotalCount:     uint32(len(paraHeadsLeaves)),
		}

		parachainHeaders = append(parachainHeaders, &header)
	}

	return parachainHeaders, nil
}

func mmrBatchProofs(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclientTypes.Hash,
	commitment rpcclientTypes.SignedCommitment,
) (rpcclientTypes.GenerateMmrBatchProofResponse, error) {
	cs, err := clientState(conn, commitment)
	if err != nil {
		return rpcclientTypes.GenerateMmrBatchProofResponse{}, err
	}

	var leafIndeces []uint64
	_, leafIndeces, err = getFinalizedBlocks(conn, cs, blockHash)
	if err != nil {
		return rpcclientTypes.GenerateMmrBatchProofResponse{}, err
	}

	// fetch mmr proofs for leaves containing our target paraId
	batchProofs, err := conn.RPC.MMR.GenerateBatchProof(leafIndeces, blockHash)
	if err != nil {
		return rpcclientTypes.GenerateMmrBatchProofResponse{}, err
	}

	return batchProofs, nil
}

func mmrBatchProofItems(mmrBatchProof rpcclientTypes.GenerateMmrBatchProofResponse) [][]byte {
	var proofItems = make([][]byte, len(mmrBatchProof.Proof.Items))
	for i := 0; i < len(mmrBatchProof.Proof.Items); i++ {
		proofItems[i] = mmrBatchProof.Proof.Items[i][:]
	}
	return proofItems
}

func mmrUpdateProof(
	conn *rpcclient.SubstrateAPI,
	blockHash rpcclientTypes.Hash,
	signedCommitment rpcclientTypes.SignedCommitment,
	leafIndex uint64,
) (*beefyClientTypes.MmrUpdateProof, error) {
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

	var signatures []*beefyClientTypes.CommitmentSignature
	var authorityIndeces []uint32
	// luckily for us, this is already sorted and maps to the right authority index in the authority root.
	for i, v := range signedCommitment.Signatures {
		if v.IsSome() {
			_, sig := v.Unwrap()
			signatures = append(signatures, &beefyClientTypes.CommitmentSignature{
				Signature:      sig[:],
				AuthorityIndex: uint32(i),
			})
			authorityIndeces = append(authorityIndeces, uint32(i))
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
	authorityTree, err := merkle.NewTree(beefyClientTypes.Keccak256{}).FromLeaves(authorityLeaves)
	if err != nil {
		panic(err)
	}

	return &beefyClientTypes.MmrUpdateProof{
		MmrLeaf: &beefyClientTypes.BeefyMmrLeaf{
			Version:        latestLeaf.Version,
			ParentNumber:   latestLeaf.ParentNumberAndHash.ParentNumber,
			ParentHash:     &parentHash,
			ParachainHeads: &ParachainHeads,
			BeefyNextAuthoritySet: beefyClientTypes.BeefyAuthoritySet{
				Id:            latestLeaf.BeefyNextAuthoritySet.ID,
				Len:           latestLeaf.BeefyNextAuthoritySet.Len,
				AuthorityRoot: &BeefyNextAuthoritySetRoot,
			},
		},
		MmrLeafIndex: leafIndex,
		MmrProof:     latestLeafMmrProof,
		SignedCommitment: &beefyClientTypes.SignedCommitment{
			Commitment: &beefyClientTypes.Commitment{
				Payload:        []*beefyClientTypes.PayloadItem{{PayloadId: &CommitmentPayload.Id, PayloadData: CommitmentPayload.Value}},
				BlockNumer:     uint32(signedCommitment.Commitment.BlockNumber),
				ValidatorSetId: uint64(signedCommitment.Commitment.ValidatorSetID),
			},
			Signatures: signatures,
		},
		AuthoritiesProof: authorityTree.Proof(authorityIndeces).ProofHashes(),
	}, nil
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
