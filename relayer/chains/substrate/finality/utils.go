package finality

import (
	"github.com/ChainSafe/chaindb"
	"github.com/ChainSafe/gossamer/lib/trie"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	"github.com/OneOfOne/xxhash"
)

func constructExtrinsics(
	conn *rpcclient.SubstrateAPI,
	blockNumber uint64, memDB *chaindb.BadgerDB,
) (timestampExtrinsic []byte, extrinsicProof [][]byte, err error) {
	blockHash, err := conn.RPC.Chain.GetBlockHash(blockNumber)
	if err != nil {
		return nil, nil, err
	}

	block, err := conn.RPC.Chain.GetBlock(blockHash)
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

	err = t.Store(memDB)
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
	extrinsicProof, err = trie.GenerateProof(rootHash.ToBytes(), [][]byte{encodedTPKey}, memDB)
	if err != nil {
		return nil, nil, err
	}

	return
}

func parachainHeaderKey(paraID uint32) ([]byte, error) {
	keyPrefix := rpcclienttypes.CreateStorageKeyPrefix(prefixParas, methodHeads)
	encodedParaId, err := rpcclienttypes.Encode(paraID)
	if err != nil {
		return nil, err
	}

	twoxhash := xxhash.New64().Sum(encodedParaId)
	fullKey := append(append(keyPrefix, twoxhash[:]...), encodedParaId...)
	return fullKey, nil
}
