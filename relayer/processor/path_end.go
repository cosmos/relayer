package processor

type PathEnd struct {
	ChainID      string
	ClientID     string
	ConnectionID string

	// TODO channel filter stuff (allowlist, denylist)
}

func (pe PathEnd) ShouldRelayChannel(channelKey ChannelKey) bool {
	// TODO return based on channel filter
	return true
}
