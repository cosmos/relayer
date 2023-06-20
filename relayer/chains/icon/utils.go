package icon

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/cryptoutils"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/common"

	"github.com/cosmos/gogoproto/proto"

	icn "github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/db"
	"github.com/icon-project/goloop/common/trie/ompt"
)

func MptProve(key types.HexInt, proofs [][]byte, hash []byte) ([]byte, error) {
	db := db.NewMapDB()
	defer db.Close()
	index, err := key.Value()
	if err != nil {
		return nil, err
	}
	indexKey, err := codec.RLP.MarshalToBytes(index)
	if err != nil {
		return nil, err
	}
	mpt := ompt.NewMPTForBytes(db, hash)
	trie, err1 := mpt.Prove(indexKey, proofs)
	if err1 != nil {
		return nil, err1

	}
	return trie, nil
}

func Base64ToData(encoded string, v interface{}) ([]byte, error) {
	if encoded == "" {
		return nil, fmt.Errorf("Encoded string is empty ")
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	return codec.RLP.UnmarshalFromBytes(decoded, v)
}

func HexBytesToProtoUnmarshal(encoded types.HexBytes, v proto.Message) ([]byte, error) {
	inputBytes, err := encoded.Value()
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling HexByte")
	}

	if bytes.Equal(inputBytes, make([]byte, 0)) {
		return nil, fmt.Errorf("Encoded hexbyte is empty ")
	}

	if err := proto.Unmarshal(inputBytes, v); err != nil {
		return nil, err

	}
	return inputBytes, nil

}

func isHexString(s string) bool {
	s = strings.ToLower(s)
	if !strings.HasPrefix(s, "0x") {
		return false
	}

	s = s[2:]

	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func btpBlockNotPresent(err error) bool {
	if strings.Contains(err.Error(), "NotFound: E1005:fail to get a BTP block header") {
		return true
	}
	return false

}

func getCommitmentHash(key, msg []byte) []byte {
	msgHash := common.Sha3keccak256(msg)
	return common.Sha3keccak256(key, msgHash)
}

func VerifyProof(commitmentkey []byte, msgval []byte, root []byte, proof []byte) (bool, error) {
	leaf := getCommitmentHash(commitmentkey, msgval)
	fmt.Printf("leaf is  %x \n ", leaf)
	var decodedProof icn.MerkleProofs
	if err := proto.Unmarshal(proof, &decodedProof); err != nil {
		return false, err
	}

	for _, v := range decodedProof.Proofs {
		fmt.Printf("index %d value %v", v.Dir, v.Value)
	}
	return cryptoutils.VerifyMerkleProof(root, leaf, decodedProof.Proofs), nil
}

func getSrcNetworkId(id int64) string {
	return fmt.Sprintf("%s.icon", types.NewHexInt(id))
}
