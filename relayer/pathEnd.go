package relayer

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
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
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
	Order        string `yaml:"order,omitempty" json:"order,omitempty"`
	Version      string `yaml:"version,omitempty" json:"version,omitempty"`
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

// GetOrder returns the channel order for the path end
func (pe *PathEnd) GetOrder() chantypes.Order {
	return OrderFromString(strings.ToUpper(pe.Order))
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

// NewPacket returns a new packet from src to dist w
func (pe *PathEnd) NewPacket(dst *PathEnd, sequence uint64, packetData []byte,
	timeoutHeight clienttypes.Height, timeoutStamp uint64) chantypes.Packet {
	return chantypes.NewPacket(
		packetData,
		sequence,
		pe.PortID,
		pe.ChannelID,
		dst.PortID,
		dst.ChannelID,
		timeoutHeight,
		timeoutStamp,
	)
}

// XferPacket creates a new transfer packet
func (pe *PathEnd) XferPacket(amount sdk.Coin, sender, receiver string) []byte {
	return transfertypes.NewFungibleTokenPacketData(
		amount.Denom,
		amount.Amount.BigInt().String(),
		sender,
		receiver,
	).GetBytes()
}
