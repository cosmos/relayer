package cryptoutils

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/common"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
)

func TestMerkleRoot(t *testing.T) {
	// Test case data
	data := HashedList{
		common.Sha3keccak256([]byte("hello")),
		common.Sha3keccak256([]byte("world")),
		common.Sha3keccak256([]byte("test")),
	}
	expectedRoot := "f071961cfd9021ffb0ee8c7b7462bed91140d643b4c39e44f6ced91b0bd1e0fc"

	// Create Merkle tree
	tree := &MerkleHashTree{
		Hashes: data,
	}

	// Calculate Merkle root
	root := tree.MerkleRoot()

	// Compare calculated root with expected root
	if hex.EncodeToString(root) != expectedRoot {
		t.Errorf("Merkle root mismatch. Got %s, expected %s", hex.EncodeToString(root), expectedRoot)
	}
}

func TestMerkleProof(t *testing.T) {
	data := HashedList{
		common.Sha3keccak256([]byte("hello")),
		common.Sha3keccak256([]byte("world")),
		common.Sha3keccak256([]byte("test")),
	}

	tree := &MerkleHashTree{
		Hashes: data,
	}
	root := tree.MerkleRoot()
	proofOfFirstItem := tree.MerkleProof(1)
	proof := make([]icon.MerkleNode, 0)
	for _, p := range proofOfFirstItem {
		proof = append(proof, *p)
	}

	if !tree.VerifyMerkleProof(root, data[1], proof) {
		t.Errorf("Merkle proof is not correct")
	}

}

func TestAppendHash(t *testing.T) {
	data := [][]byte{
		[]byte("hello"),
		[]byte("world"),
	}

	h1 := common.Sha3keccak256(data[0])
	h1 = AppendHash(h1, data[1])
	fmt.Printf("h1: %x \n", h1)

	h2 := common.Sha3keccak256(data...)

	fmt.Printf("h2: %x \n", h2)

}

func TestMerkleProofMisMatch(t *testing.T) {
	data := HashedList{
		common.Sha3keccak256([]byte("hello")),
		common.Sha3keccak256([]byte("world")),
		common.Sha3keccak256([]byte("test")),
	}

	failcase := common.Sha3keccak256([]byte("should_fail"))

	tree := &MerkleHashTree{
		Hashes: data,
	}
	root := tree.MerkleRoot()
	proofOfFirstItem := tree.MerkleProof(1)
	proof := make([]icon.MerkleNode, 0)
	for _, p := range proofOfFirstItem {
		proof = append(proof, *p)
	}

	if tree.VerifyMerkleProof(root, failcase, proof) {
		t.Errorf("Merkle proof of data %x should not match data_list", failcase)
	}

}
