package finality

import (
	"fmt"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
)

func (g Grandpa) getCurrentSetID() (uint64, error) {
	meta, err := g.parachainClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return 0, err
	}

	storageKey, err := rpcclienttypes.CreateStorageKey(meta, prefixParas, methodParachains, nil, nil)
	if err != nil {
		return 0, err
	}

	var paraIds []uint32

	ok, err := g.parachainClient.RPC.State.GetStorage(storageKey, &paraIds, blockHash)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("%s: storage key %v, paraids %v, block hash %v", ErrParachainSetNotFound, storageKey, paraIds, blockHash)
	}

	return paraIds, nil
}
