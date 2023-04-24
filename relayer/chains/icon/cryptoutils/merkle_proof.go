package cryptoutils

import (
	"bytes"
	"math/bits"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
)

const hashLen = 32

type HashedList [][]byte

func (b HashedList) Len() int {
	return len(b)
}

func (b HashedList) Get(i int) []byte {
	return b[i]
}

func (b HashedList) FindIndex(target []byte) int {
	for i, item := range b {
		if bytes.Equal(item, target) {
			return i
		}
	}
	return -1
}

type MerkleHashTree struct {
	Hashes HashedList
}

func NewMerkleHashTree(byteList [][]byte) *MerkleHashTree {

	var hashList HashedList
	for _, b := range byteList {
		hashList = append(hashList, Sha3keccak256(b))
	}
	return &MerkleHashTree{
		Hashes: hashList,
	}
}

func AppendHash(out []byte, data []byte) []byte {
	return appendKeccak256(out, data)
}

func __merkleRoot(data []byte) []byte {
	for len(data) > hashLen {
		i, j := 0, 0
		for ; i < len(data); i, j = i+hashLen*2, j+hashLen {
			if i+hashLen*2 <= len(data) {
				AppendHash(data[:j], data[i:i+hashLen*2])
			} else {
				copy(data[j:j+hashLen], data[i:i+hashLen])
			}
		}
		data = data[:j]
	}
	return data[:hashLen]
}

func (m *MerkleHashTree) MerkleRoot() []byte {
	data := m.Hashes
	if data.Len() == 0 {
		return nil
	}
	if data.Len() == 1 {
		return data.Get(0)
	}
	dataBuf := make([]byte, 0, data.Len()*hashLen)
	for i := 0; i < data.Len(); i++ {
		dataBuf = append(dataBuf, data.Get(i)...)
	}
	return __merkleRoot(dataBuf)
}

func __merkleProof(data []byte, idx int) []*icon.MerkleNode {
	proof := make([]*icon.MerkleNode, 0, bits.Len(uint(len(data))))
	for len(data) > hashLen {
		i, j := 0, 0
		for ; i < len(data); i, j = i+hashLen*2, j+hashLen {
			if i+hashLen*2 <= len(data) {
				var val []byte
				if idx == i {
					val = append(val, data[i+hashLen:i+hashLen*2]...)
					proof = append(
						proof,
						&icon.MerkleNode{Dir: int32(types.DirRight), Value: val},
					)
					idx = j
				} else if idx == i+hashLen {
					val = append(val, data[i:i+hashLen]...)
					proof = append(
						proof,
						&icon.MerkleNode{Dir: int32(types.DirLeft), Value: val},
					)
					idx = j
				}
				AppendHash(data[:j], data[i:i+hashLen*2])
			} else {
				if idx == i {
					proof = append(
						proof,
						&icon.MerkleNode{Dir: int32(types.DirRight), Value: nil},
					)
					idx = j
				}
				copy(data[j:j+hashLen], data[i:i+hashLen])
			}
		}
		data = data[:j]
	}
	return proof
}

func (m *MerkleHashTree) VerifyMerkleProof(root []byte, value []byte, proof []icon.MerkleNode) bool {
	computedHash := make([]byte, len(value))
	copy(computedHash, value)

	for _, node := range proof {
		hashBuf := make([]byte, hashLen*2)
		if node.Dir == int32(types.DirLeft) {
			copy(hashBuf[:hashLen], node.Value)
			copy(hashBuf[hashLen:], computedHash)
		} else {
			copy(hashBuf[:hashLen], computedHash)
			if node.Value != nil {
				copy(hashBuf[hashLen:], node.Value)
			} else {
				copy(hashBuf[hashLen:], make([]byte, hashLen))
			}
		}
		AppendHash(computedHash[:0], hashBuf)
	}

	return bytes.Equal(root, computedHash)
}

func (m *MerkleHashTree) MerkleProof(idx int) []*icon.MerkleNode {
	data := m.Hashes
	if data.Len() == 0 {
		return nil
	}
	if data.Len() == 1 {
		return nil
	}
	dataBuf := make([]byte, 0, data.Len()*hashLen)
	for i := 0; i < data.Len(); i++ {
		dataBuf = append(dataBuf, data.Get(i)...)
	}
	return __merkleProof(dataBuf, idx*hashLen)
}
