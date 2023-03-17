package relayer

// PathHop represents the local connection identifiers for intermediate hops on a multi-hop relay path
// The path is set on the chain before performing operations
type PathHop struct {
	ChainID  string      `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	PathEnds [2]*PathEnd `yaml:"path-ends,omitempty" json:"path-ends,omitempty"`
}

func NewPathHops(ibcData *IBCdata, srcPathEnd, dstPathEnd *PathEnd, chains *Chains) ([]*PathHop, error) {
	hopSrcChainID := srcPathEnd.ChainID
	hopDstChainID := dstPathEnd.ChainID
	pathHops := []*PathHop{}
	for i, hop := range ibcData.Hops {
		if i > 0 {
			hopSrcChainID = pathHops[i-1].ChainID
		}
		if i < len(ibcData.Hops)-1 {
			// We need to pull this one out of the config since we haven't processed the PathHop yet
			hopDstChain, err := chains.Get(ibcData.Hops[i+1].ChainName)
			if err != nil {
				return nil, err
			}
			hopDstChainID = hopDstChain.ChainID()
		}
		chain, err := chains.Get(hop.ChainName)
		if err != nil {
			return nil, err
		}
		pathHops = append(pathHops, &PathHop{
			ChainID: chain.ChainID(),
			PathEnds: [2]*PathEnd{
				{
					ChainID:      hopSrcChainID,
					ClientID:     hop.ClientIDs[0],
					ConnectionID: hop.ConnectionIDs[0],
				},
				{
					ChainID:      hopDstChainID,
					ClientID:     hop.ClientIDs[1],
					ConnectionID: hop.ConnectionIDs[1],
				},
			},
		})
	}
	return pathHops, nil
}
