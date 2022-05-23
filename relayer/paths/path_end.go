package paths

import "github.com/cosmos/relayer/v2/relayer/ibc"

type PathEnd struct {
	ChainID      string
	ClientID     string
	ConnectionID string

	// probably just allowlist to start
	// can only provide one. panic if both are provided
	AllowList []ibc.ChannelKey // only use these if provided
	BlockList []ibc.ChannelKey // use everything except these if provided
}

func NewPathEnd(chainID string, clientID string, connectionID string, allowList []ibc.ChannelKey, blockList []ibc.ChannelKey) PathEnd {
	if len(allowList) > 0 && len(blockList) > 0 {
		panic("only one of allowlist or blocklist are allowed")
	}
	return PathEnd{
		ChainID:      chainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
		AllowList:    allowList,
		BlockList:    blockList,
	}
}

// expanded for readability
func (pe PathEnd) ShouldRelayChannel(channelKey ibc.ChannelKey) bool {
	if len(pe.AllowList) > 0 {
		for _, allowedChannel := range pe.AllowList {
			if allowedChannel.ChannelID == channelKey.ChannelID {
				if allowedChannel.PortID == "" {
					return true
				}
				if allowedChannel.PortID == channelKey.PortID {
					return true
				}
			}
			if allowedChannel.ChannelID == channelKey.CounterpartyChannelID {
				if allowedChannel.PortID == "" {
					return true
				}
				if allowedChannel.PortID == channelKey.CounterpartyPortID {
					return true
				}
			}
			if allowedChannel.CounterpartyChannelID == channelKey.ChannelID {
				if allowedChannel.CounterpartyPortID == "" {
					return true
				}
				if allowedChannel.CounterpartyPortID == channelKey.PortID {
					return true
				}
			}
			if allowedChannel.CounterpartyChannelID == channelKey.CounterpartyChannelID {
				if allowedChannel.CounterpartyPortID == "" {
					return true
				}
				if allowedChannel.CounterpartyPortID == channelKey.CounterpartyPortID {
					return true
				}
			}
		}
		return false
	} else if len(pe.BlockList) > 0 {
		for _, blockedChannel := range pe.BlockList {
			if blockedChannel.ChannelID == channelKey.ChannelID {
				if blockedChannel.PortID == "" {
					return false
				}
				if blockedChannel.PortID == channelKey.PortID {
					return false
				}
			}
			if blockedChannel.ChannelID == channelKey.CounterpartyChannelID {
				if blockedChannel.PortID == "" {
					return false
				}
				if blockedChannel.PortID == channelKey.CounterpartyPortID {
					return false
				}
			}
			if blockedChannel.CounterpartyChannelID == channelKey.ChannelID {
				if blockedChannel.CounterpartyPortID == "" {
					return false
				}
				if blockedChannel.CounterpartyPortID == channelKey.PortID {
					return false
				}
			}
			if blockedChannel.CounterpartyChannelID == channelKey.CounterpartyChannelID {
				if blockedChannel.CounterpartyPortID == "" {
					return false
				}
				if blockedChannel.CounterpartyPortID == channelKey.CounterpartyPortID {
					return false
				}
			}
		}
		return true
	}
	// if neither allow list or block list are provided, all channels are okay
	return true
}

// store recent block header information (10 blocks?) for each chain so we can dynamically construct update client msgs
