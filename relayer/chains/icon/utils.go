package icon

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"

	"github.com/cosmos/gogoproto/proto"

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

func HexStringToProtoUnmarshal(encoded string, v proto.Message) ([]byte, error) {
	if encoded == "" {
		return nil, fmt.Errorf("Encoded string is empty ")
	}

	input_ := strings.TrimPrefix(encoded, "0x")
	inputBytes, err := hex.DecodeString(input_)
	if err != nil {
		return nil, err
	}

	err = proto.Unmarshal(inputBytes, v)
	if err != nil {
		return nil, err
	}
	return inputBytes, nil

}
