package processor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testChainID0 = "test-chain-0"
	testChainID1 = "test-chain-1"

	testChannel0 = "test-channel-0"
	testChannel1 = "test-channel-1"
	testChannel2 = "test-channel-2"

	testPort  = "trasnfer"
	testPort2 = "ica-XXX"
)

func TestNoChannelFilter(t *testing.T) {
	mockPathEnd := PathEnd{}

	mockChannel := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	mockCounterpartyChannel := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
		},
	}

	mockPathEndRuntime := pathEndRuntime{
		info: mockPathEnd,

		channelStateCache: ChannelStateCache{
			mockChannel.ChannelKey:             ChannelState{Open: true},
			mockCounterpartyChannel.ChannelKey: ChannelState{Open: true},
		},
	}

	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockChannel), "does not allow channel to be relayed, even though there is no filter and channel is cached")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockCounterpartyChannel), "does not allow counterparty channel to be relayed, even though there is no filter and channel is cached")

	// channel or counterparty channel is not  not included in channelStateCache,
	// ie, this channel does not pertain to a both a src and dest chain in the path section of the config
	// this channel is from a different client
	mockChannel2 := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel2,
		},
	}

	mockCounterpartyChanne2 := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel2,
		},
	}

	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockChannel2), "allowed channel to be relayed, even though it was outside of cached state; this channel does not pertain to a src or dest chain in the path secion of the config")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockCounterpartyChanne2), "allowed channel to be relayed, even though it was outside of cached state; this channel does not pertain to a src or dest chain in the path secion of the config")

}

func TestAllowChannelFilter(t *testing.T) {
	mockAllowedChannel := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	mockAllowedCounterPartyChannel := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
		},
	}

	// above two channels without port added to allow filter
	// we should relay on any port for the channel
	mockPathEnd := PathEnd{
		Rule: RuleAllowList,
		FilterList: []ChainChannelKey{
			mockAllowedChannel,
		},
	}

	// same channel and counterparty channel as above except has designated port
	// need to add these to the channelStateCache
	// these should still be allowed to relay b/c the channels added to the allowlist
	// didn't include a port
	mockAllowedChannelWithSpecificPort := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort,
		},
	}
	mockCounterPartyChannelWithSpecificPort := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort,
		},
	}

	// channel and counterparty channel not on allow list
	mockNotAllowedChannel := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel2,
		},
	}
	mockNotAllowedCounterpartyChannel := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel2,
		},
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	// allowed channels, no port
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannel), "does not allow channel to be relayed, even though channelID is in allow list")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedCounterPartyChannel), "does not allow counterparty channel to be relayed, even though channelID is in allow list")

	// allowed channels with port
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannelWithSpecificPort), "does not allow channel with specified port to be relayed, even though there is no port in the allow filter")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockCounterPartyChannelWithSpecificPort), "does not allow counterparty channel with specified port to be relayed, even though there is no port in the allow filter")

	// channels not included in allow list
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockNotAllowedChannel), "allows channel to be relayed, even though channelID is not in allow list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockNotAllowedCounterpartyChannel), "allows channel to be relayed, even though channelID is not in allow list")
}

func TestDenyChannelFilter(t *testing.T) {
	mockBlocked := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	mockBlockedCounterparty := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
		},
	}

	// blocked channels added to deny list
	mockPathEnd := PathEnd{
		Rule: RuleDenyList,
		FilterList: []ChainChannelKey{
			mockBlocked,
		},
	}

	// same channel and counterparty channel as above except has designated port
	// need to add these to the channelStateCache
	// these should not allowed to relay b/c the channels were added to the denylist
	mockBlockedChannelWithSpecificPort := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort,
		},
	}
	mockBlockedCounterPartyChannelWithSpecificPort := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort,
		},
	}

	// channel and counterparty channel not added to deny list
	mockNotBlocked := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel2,
		},
	}

	mockNotBlockedCounterparty := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel2,
		},
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	// channels added to deny list
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlocked), "allows channel to be relayed, even though channelID is in deny list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedCounterparty), "allows channel to be relayed, even though channelID is in deny list")

	// channels added to deny list with specific port
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedChannelWithSpecificPort), "allows channel to be relayed, even though channel is in deny list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockBlockedCounterPartyChannelWithSpecificPort), "allows channel to be relayed, even though channel is in deny list")

	// channels not included in deny list
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockNotBlocked), "does not allow channel to be relayed, even though channelID is not in deny list")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockNotBlockedCounterparty), "does not allow channel to be relayed, even though channelID is not in deny list")

}

func TestAllowChannelFilterWithSpecificPort(t *testing.T) {
	mockAllowedChannelWithPort := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort,
		},
	}

	mockAllowedCounterpartyWithPort := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort,
		},
	}

	mockPathEnd := PathEnd{
		Rule: RuleAllowList,
		FilterList: []ChainChannelKey{
			mockAllowedChannelWithPort,
		},
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannelWithPort), "does not allow port to be relayed on, even though portID is in allow list")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedCounterpartyWithPort), "does not allow port to be relayed on, even though portID is in allow list")

	// same channel without designated port
	mockAllowedChannelWithOutPort := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
		},
	}

	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockAllowedChannelWithOutPort), "allows port to be relayed on, even though portID is not in allow list")
}

func TestDenyChannelWithSpecificPort(t *testing.T) {
	mockDeniedChannelWithPort := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort,
		},
	}

	mockCounterpartyDeniedChannelWithPort := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort,
		},
	}

	// same path with different port
	mockChannelWithAllowedPort := ChainChannelKey{
		ChainID: testChainID0,
		ChannelKey: ChannelKey{
			ChannelID: testChannel0,
			PortID:    testPort2,
		},
	}

	mocCounterpartykChannelWithAllowedPort := ChainChannelKey{
		CounterpartyChainID: testChainID0,
		ChannelKey: ChannelKey{
			CounterpartyChannelID: testChannel0,
			CounterpartyPortID:    testPort2,
		},
	}

	mockPathEnd := PathEnd{
		Rule: RuleDenyList,
		FilterList: []ChainChannelKey{
			mockDeniedChannelWithPort,
		},
	}

	mockPathEndRuntime := pathEndRuntime{info: mockPathEnd}

	// channels and ports added to deny list
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockDeniedChannelWithPort), "allows port to be relayed on, even though portID is in deny list")
	require.False(t, mockPathEndRuntime.ShouldRelayChannel(mockCounterpartyDeniedChannelWithPort), "allows port to be relayed on, even though portID is in deny list")

	// same channels with different ports
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mockChannelWithAllowedPort), "does not allow port to be relayed on, even though portID is not in deny list")
	require.True(t, mockPathEndRuntime.ShouldRelayChannel(mocCounterpartykChannelWithAllowedPort), "does not allow port to be relayed on, even though portID is not in deny list")

}
