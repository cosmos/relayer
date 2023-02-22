package processor_test

import (
	"testing"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
)

const (
	testChain0   = "test-chain-0"
	testChannel0 = "test-channel-0"
	testPort0    = "test-port-0"
	testChannel1 = "test-channel-1"
	testPort1    = "test-port-1"
)

// empty allow list and block list should relay everything
func TestAllowAllChannels(t *testing.T) {
	mockPathEnd := processor.PathEnd{}

	mockAllowedChannel := processor.ChainChannelKey{
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though allow list and block list are empty")

	// test counterparty
	mockAllowedChannel2 := processor.ChainChannelKey{
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel1,
			CounterpartyPortID:    testPort1,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel2), "does not allow counterparty channel to be relayed, even though allow list and block list are empty")
}

func TestAllowAllPortsForChannel(t *testing.T) {
	mockAllowList := []processor.ChainChannelKey{{
		ChainID:    testChain0,
		ChannelKey: processor.ChannelKey{ChannelID: testChannel0},
	}}
	mockPathEnd := processor.PathEnd{
		Rule:       processor.RuleAllowList,
		FilterList: mockAllowList,
	}

	mockAllowedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though channelID is in allow list")

	// test counterparty
	mockAllowedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort0,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel2), "does not allow counterparty channel to be relayed, even though channelID is in allow list")

	mockBlockedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel1,
			PortID:    testPort1,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel), "allows channel to be relayed, even though channelID is not in allow list")

	mockBlockedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel1,
			CounterpartyPortID:    testPort1,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel2), "allows channel to be relayed, even though channelID is not in allow list")
}

func TestAllowSpecificPortForChannel(t *testing.T) {
	mockAllowList := []processor.ChainChannelKey{
		{
			ChainID: testChain0,
			ChannelKey: processor.ChannelKey{
				ChannelID: testChannel0,
				PortID:    testPort0,
			},
		},
	}
	mockPathEnd := processor.PathEnd{
		Rule:       processor.RuleAllowList,
		FilterList: mockAllowList,
	}

	mockAllowedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though channelID is in allow list")

	// test counterparty
	mockAllowedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort0,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel2), "does not allow counterparty channel to be relayed, even though channelID is in allow list")

	mockBlockedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort1,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel), "allows channel to be relayed, even though portID is not in allow list")

	mockBlockedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort1,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel2), "allows channel to be relayed, even though portID is not in allow list")
}

func TestBlockAllPortsForChannel(t *testing.T) {
	mockBlockList := []processor.ChainChannelKey{
		{
			ChainID: testChain0,
			ChannelKey: processor.ChannelKey{
				ChannelID: testChannel0,
			},
		},
	}
	mockPathEnd := processor.PathEnd{
		Rule:       processor.RuleDenyList,
		FilterList: mockBlockList,
	}

	mockBlockedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel), "allows channel to be relayed, even though channelID is in block list")

	// test counterparty
	mockBlockedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort0,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel2), "allows counterparty channel to be relayed, even though channelID is in block list")

	mockAllowedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel1,
			PortID:    testPort1,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though channelID is not in block list")

	mockAllowedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel1,
			CounterpartyPortID:    testPort1,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel2), "does not allow counterparty channel to be relayed, even though channelID is not in block list")
}

func TestBlockSpecificPortForChannel(t *testing.T) {
	mockBlockList := []processor.ChainChannelKey{
		{
			ChainID: testChain0,
			ChannelKey: processor.ChannelKey{
				ChannelID: testChannel0,
				PortID:    testPort0,
			},
		},
	}
	mockPathEnd := processor.PathEnd{
		Rule:       processor.RuleDenyList,
		FilterList: mockBlockList,
	}

	mockBlockedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel), "allows channel to be relayed, even though channelID/portID is in block list")

	// test counterparty
	mockBlockedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort0,
		},
	}
	require.False(t, mockPathEnd.ShouldRelayChannel(mockBlockedChannel2), "allows counterparty channel to be relayed, even though channelID/portID is in block list")

	mockAllowedChannel := processor.ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort1,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though portID is not in block list")

	mockAllowedChannel2 := processor.ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: processor.ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort1,
		},
	}
	require.True(t, mockPathEnd.ShouldRelayChannel(mockAllowedChannel2), "does not allow counterparty channel to be relayed, even though portID is not in block list")
}
