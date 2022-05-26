package relayer

import (
	"strings"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
)

// TODO double check that these are okay
var (
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint64(0)
)

// PathEnd represents the local connection identifiers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
}

// OrderFromString parses a string into a channel order byte
func OrderFromString(order string) chantypes.Order {
	switch strings.ToUpper(order) {
	case "UNORDERED":
		return chantypes.UNORDERED
	case "ORDERED":
		return chantypes.ORDERED
	default:
		return chantypes.NONE
	}
}

// StringFromOrder returns the string representation of a channel order.
func StringFromOrder(order chantypes.Order) string {
	switch order {
	case chantypes.UNORDERED:
		return "unordered"
	case chantypes.ORDERED:
		return "ordered"
	default:
		return ""
	}
}

var marshalledChains = map[PathEnd]*Chain{}

// MarshalChain is PathEnd
func MarshalChain(c *Chain) PathEnd {
	pe := *c.PathEnd
	if _, ok := marshalledChains[pe]; !ok {
		marshalledChains[pe] = c
	}
	return pe
}

// UnmarshalChain returns Marshalled chain
func UnmarshalChain(pe PathEnd) *Chain {
	if c, ok := marshalledChains[pe]; ok {
		return c
	}
	return nil
}
