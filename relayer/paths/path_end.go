package paths

type PathEnd struct {
	ChainID string

	// TODO clientID, connectionID, and channel filter stuff (allowlist, denylist)
}

// TODO methods for determining if a given channel, client, or connection is relevant for this pathEnd
