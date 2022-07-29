package processor_test

import (
	"testing"

	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
	tmtypes "github.com/tendermint/tendermint/types"
)

type mockIBCHeader struct{}

func (h mockIBCHeader) Height() uint64                                       { return 0 }
func (h mockIBCHeader) ConsensusState() ibcexported.ConsensusState           { return nil }
func (h mockIBCHeader) ToCosmosValidatorSet() (*tmtypes.ValidatorSet, error) { return nil, nil }

func TestIBCHeaderCachePrune(t *testing.T) {
	cache := make(processor.IBCHeaderCache)

	intermediaryCache1 := make(processor.IBCHeaderCache)
	for i := uint64(0); i < 10; i++ {
		intermediaryCache1[i] = mockIBCHeader{}
	}

	intermediaryCache2 := make(processor.IBCHeaderCache)
	for i := uint64(10); i < 20; i++ {
		intermediaryCache2[i] = mockIBCHeader{}
	}

	cache.Merge(intermediaryCache1)
	require.Len(t, cache, 10)

	// test pruning with keep greater than length
	cache.Prune(15)
	require.Len(t, cache, 10)

	cache.Merge(intermediaryCache2)
	require.Len(t, cache, 20)

	cache.Prune(5)
	require.Len(t, cache, 5)
	require.NotNil(t, cache[uint64(15)], cache[uint64(16)], cache[uint64(17)], cache[uint64(18)], cache[uint64(19)])
}
