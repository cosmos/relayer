package icon

import (
	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/db"
	"github.com/icon-project/goloop/common/trie/ompt"
	"github.com/icon-project/ibc-relayer/relayer/chains/icon/types"
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
