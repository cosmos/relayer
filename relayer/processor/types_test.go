package processor_test

import (
	"testing"

	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
)

type mockIBCHeader struct{}

func (h mockIBCHeader) Height() uint64                             { return 0 }
func (h mockIBCHeader) ConsensusState() ibcexported.ConsensusState { return nil }
func (h mockIBCHeader) NextValidatorsHash() []byte                 { return nil }

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

func TestPacketSequenceStateCachePrune(t *testing.T) {
	cache := make(processor.PacketSequenceStateCache)

	for i := uint64(0); i < 50; i++ {
		cache[i] = chantypes.EventTypeSendPacket
	}

	cache.Prune(100)

	require.Len(t, cache, 50)

	cache.Prune(25)

	require.Len(t, cache, 25)

	min := uint64(1000)
	max := uint64(0)

	for seq := range cache {
		if seq < min {
			min = seq
		}
		if seq > max {
			max = seq
		}
	}

	require.Equal(t, uint64(25), min)
	require.Equal(t, uint64(49), max)
}
