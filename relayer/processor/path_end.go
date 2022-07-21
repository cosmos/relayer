package processor

// PathEnd references one chain involved in a path.
// A path is composed of two PathEnds.
type PathEnd struct {
	ChainID  string
	ClientID string
	// ConnectionIDs are tracked by pathEndRuntime in PathProcessor for known connections on this client

	// Can be either "allowlist" or "denylist"
	Rule       string
	FilterList []ChannelKey // which channels to allow or deny
}

// NewPathEnd constructs a PathEnd, validating initial parameters.
func NewPathEnd(chainID string, clientID string, rule string, filterList []ChannelKey) PathEnd {
	return PathEnd{
		ChainID:    chainID,
		ClientID:   clientID,
		Rule:       rule,
		FilterList: filterList,
	}
}

const (
	RuleAllowList = "allowlist"
	RuleDenyList  = "denylist"
)

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
	if pe.Rule == RuleAllowList {
		for _, allowedChannel := range pe.FilterList {
			if pe.shouldRelayChannelSingle(channelKey, allowedChannel, true) {
				return true
			}
		}
		return false
	} else if pe.Rule == RuleDenyList {
		for _, blockedChannel := range pe.FilterList {
			if !pe.shouldRelayChannelSingle(channelKey, blockedChannel, false) {
				return false
			}
		}
		return true
	}
	// if neither allow list or block list are provided, all channels are okay
	return true
}
