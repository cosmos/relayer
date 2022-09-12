package processor

// PathEnd references one chain involved in a path.
// A path is composed of two PathEnds.
type PathEnd struct {
	PathName string
	ChainID  string
	ClientID string
	// ConnectionIDs are tracked by pathEndRuntime in PathProcessor for known connections on this client

	// Can be either "allowlist" or "denylist"
	Rule       string
	FilterList []ChainChannelKey // which channels to allow or deny
}

type ChainChannelKey struct {
	ChainID             string
	CounterpartyChainID string
	ChannelKey          ChannelKey
}

// NewPathEnd constructs a PathEnd, validating initial parameters.
func NewPathEnd(pathName string, chainID string, clientID string, rule string, filterList []ChainChannelKey) PathEnd {
	return PathEnd{
		PathName:   pathName,
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

func (pe PathEnd) checkChannelMatch(listChainID, listChannelID, listPortID string, channelKey ChainChannelKey) bool {
	if listChannelID == "" {
		return false
	}
	if listChannelID == channelKey.ChannelKey.ChannelID && listChainID == channelKey.ChainID {
		if listPortID == "" {
			return true
		}
		if listPortID == channelKey.ChannelKey.PortID {
			return true
		}
	}
	if listChannelID == channelKey.ChannelKey.CounterpartyChannelID && listChainID == channelKey.CounterpartyChainID {
		if listPortID == "" {
			return true
		}
		if listPortID == channelKey.ChannelKey.CounterpartyPortID {
			return true
		}
	}
	return false
}

func (pe PathEnd) shouldRelayChannelSingle(channelKey ChainChannelKey, listChannel ChainChannelKey, allowList bool) bool {
	if pe.checkChannelMatch(listChannel.ChainID, listChannel.ChannelKey.ChannelID, listChannel.ChannelKey.PortID, channelKey) {
		return allowList
	}
	if pe.checkChannelMatch(listChannel.CounterpartyChainID, listChannel.ChannelKey.CounterpartyChannelID, listChannel.ChannelKey.CounterpartyPortID, channelKey) {
		return allowList
	}
	return !allowList
}

// if port ID is empty on allowlist channel, allow all ports
// if port ID is non-empty on allowlist channel, allow only that specific port
// if port ID is empty on blocklist channel, block all ports
// if port ID is non-empty on blocklist channel, block only that specific port
func (pe PathEnd) ShouldRelayChannel(channelKey ChainChannelKey) bool {
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
