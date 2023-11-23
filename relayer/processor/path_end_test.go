package processor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testChain0 = "test-chain-0"

	testChannel0 = "test-channel-0"
	testChannel1 = "test-channel-1"
	testChannel2 = "test-channel-2"

	testPort0 = "test-port-0"
	testPort1 = "test-port-1"
)

// empty allow list and block list should relay everything
func TestAllowAllChannels(t *testing.T) {
	mockPathEnd := PathEnd{}

	mockPathEndRuntime := pathEndRuntime{
		info: mockPathEnd,
		channelStateCache: ChannelStateCache{
			ChannelKey{
				ChannelID: testChain0,
				PortID:    testPort0,
			}: ChannelState{Open: true},
			ChannelKey{
				CounterpartyChannelID: testChannel1,
				CounterpartyPortID:    testPort1,
			}: ChannelState{Open: true},
		},
	}

	mockAllowedChannel := ChainChannelKey{
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though allow list and block list are empty")

	// test counterparty
	mockAllowedChannel2 := ChainChannelKey{
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel1,
			CounterpartyPortID:    testPort1,
		},
	}
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannel2), "does not allow counterparty channel to be relayed, even though allow list and block list are empty")
}

func TestAllowChannel(t *testing.T) {
	mockAllowedChannel := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	mockCounterPartyChannel := ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
		},
	}

	mockAllowList := []ChainChannelKey{
		mockAllowedChannel,
		mockCounterPartyChannel,
	}

	mockPathEnd := PathEnd{
		Rule:       RuleAllowList,
		FilterList: mockAllowList,
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though channelID is in allow list")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockCounterPartyChannel), "does not allow counterparty channel to be relayed, even though channelID is in allow list")

	// same channel in allow list except has designated port
	mockAllowedChannelWithSpecificPort := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}

	// same channel in counterparty allow list except has designated port
	mockCounterPartyChannelWithSpecificPort := ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort0,
		},
	}
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannelWithSpecificPort), "does not allow channel with port to be relayed, even though there is no port in the allow filter")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockCounterPartyChannelWithSpecificPort), "does not allow channel with port to be relayed, even though there is no port in the allow filter")

	// channel not on allow list
	mockBlockedChannel := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel2,
		},
	}

	mockBlockedCounterpartyChannel := ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel2,
		},
	}
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedChannel), "allows channel to be relayed, even though portID is not in allow list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedCounterpartyChannel), "allows channel to be relayed, even though portID is not in allow list")
}

func TestBlockedChannel(t *testing.T) {
	mockBlockedChannel := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	mockBlockedCounterPartyChannel := ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
		},
	}

	mockDenyList := []ChainChannelKey{
		mockBlockedChannel,
		mockBlockedCounterPartyChannel,
	}

	mockPathEnd := PathEnd{
		Rule:       RuleDenyList,
		FilterList: mockDenyList,
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedChannel), "allows channel to be relayed, even though channelID is not in allow list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedCounterPartyChannel), "allows channel to be relayed, even though channelID is not in allow list")

	// same blocked channels except with specific port
	mockBlockedChannelWithSpecificPort := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}

	mockBlockedCounterPartyChannelWithSpecificPort := ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			PortID:                testPort0,
		},
	}

	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedChannelWithSpecificPort), "allows channel to be relayed, even though channel/portID is not in allow list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedCounterPartyChannelWithSpecificPort), "allows channel to be relayed, even though channel/portID is not in allow list")

	// not on blocked list
	mockAllowedChannel := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel2,
		},
	}

	mockAllowedCounterpartyChannel := ChainChannelKey{
		CounterpartyChainID: testChain0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel2,
		},
	}
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though channelID is in allow list")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedCounterpartyChannel), "does not allow channel to be relayed, even though channelID is in allow list")
}

func TestAllowChannelOnlyWithSpecificPort(t *testing.T) {
	mockAllowedChannel := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}

	mockAllowList := []ChainChannelKey{
		mockAllowedChannel,
	}

	mockPathEnd := PathEnd{
		Rule:       RuleAllowList,
		FilterList: mockAllowList,
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannel), "does not allow port to be relayed on, even though portID is in allow list")

	// same path without designated port
	mockChannelWithOutPort := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockChannelWithOutPort), "allows port to be relayed on, even though portID is not in allow list")
}

func TestDenyChannelWithSpecificPort(t *testing.T) {
	mockDeniedChannel := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort0,
		},
	}

	mockDenyList := []ChainChannelKey{
		mockDeniedChannel,
	}

	mockPathEnd := PathEnd{
		Rule:       RuleDenyList,
		FilterList: mockDenyList,
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockDeniedChannel), "allows port to be relayed on, even though portID is not in allow list")

	// same path with different port
	mockChannelWithOutPort := ChainChannelKey{
		ChainID: testChain0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort1,
		},
	}

	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockChannelWithOutPort), "does not allow port to be relayed on, even though portID is in allow list")
}
