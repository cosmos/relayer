package relayer

// PathHop represents the local connection identifiers for intermediate hops on a multi-hop relay path
// The path is set on the chain before performing operations
type PathHop struct {
	ChainID  string      `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	PathEnds [2]*PathEnd `yaml:"path-ends,omitempty" json:"path-ends,omitempty"`
}
