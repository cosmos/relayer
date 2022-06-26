package processor

// PathEnd references one chain involved in a path.
// A path is composed of two PathEnds.
type PathEnd struct {
	ChainID  string
	ClientID string
	// ConnectionIDs are tracked by pathEndRuntime in PathProcessor for known connections on this client

	// can only provide one. panic if both are provided
	AllowList []ChannelKey // only use these if provided
	BlockList []ChannelKey // use everything except these if provided
}

// NewPathEnd constructs a PathEnd, validating initial parameters.
func NewPathEnd(chainID string, clientID string, allowList []ChannelKey, blockList []ChannelKey) PathEnd {
	if len(allowList) > 0 && len(blockList) > 0 {
		panic("only one of allowlist or blocklist are allowed")
	}
	return PathEnd{
		ChainID:   chainID,
		ClientID:  clientID,
		AllowList: allowList,
		BlockList: blockList,
	}
}

func (pe PathEnd) checkChannelMatch(listChannelID, listPortID string, channelKey ChannelKey) bool {
	if listChannelID == "" {
		return false
	}
	if listChannelID == channelKey.ChannelID {
		if listPortID == "" {
			return true
		}
		if listPortID == channelKey.PortID {
			return true
		}
	}
	if listChannelID == channelKey.CounterpartyChannelID {
		if listPortID == "" {
			return true
		}
		if listPortID == channelKey.CounterpartyPortID {
			return true
		}
	}
	return false
}

func (pe PathEnd) shouldRelayChannelSingle(channelKey ChannelKey, listChannel ChannelKey, allowList bool) bool {
	if pe.checkChannelMatch(listChannel.ChannelID, listChannel.PortID, channelKey) {
		return allowList
	}
	if pe.checkChannelMatch(listChannel.CounterpartyChannelID, listChannel.CounterpartyPortID, channelKey) {
		return allowList
	}
	return !allowList
}

// if port ID is empty on allowlist channel, allow all ports
// if port ID is non-empty on allowlist channel, allow only that specific port
// if port ID is empty on blocklist channel, block all ports
// if port ID is non-empty on blocklist channel, block only that specific port
func (pe PathEnd) ShouldRelayChannel(channelKey ChannelKey) bool {
	if len(pe.AllowList) > 0 {
		for _, allowedChannel := range pe.AllowList {
			if pe.shouldRelayChannelSingle(channelKey, allowedChannel, true) {
				return true
			}
		}
		return false
	} else if len(pe.BlockList) > 0 {
		for _, blockedChannel := range pe.BlockList {
			if !pe.shouldRelayChannelSingle(channelKey, blockedChannel, false) {
				return false
			}
		}
		return true
	}
	// if neither allow list or block list are provided, all channels are okay
	return true
}
