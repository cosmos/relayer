package relayer

import (
	"fmt"
	"strings"

	host "github.com/cosmos/cosmos-sdk/x/ibc/24-host"
)

// Vclient validates the client identifier in the path
func (p *PathEnd) Vclient() error {
	return host.ClientIdentifierValidator(p.ClientID)
}

// Vconn validates the connection identifier in the path
func (p *PathEnd) Vconn() error {
	return host.ConnectionIdentifierValidator(p.ConnectionID)
}

// Vchan validates the channel identifier in the path
func (p *PathEnd) Vchan() error {
	return host.ChannelIdentifierValidator(p.ChannelID)
}

// Vport validates the port identifier in the path
func (p *PathEnd) Vport() error {
	return host.PortIdentifierValidator(p.PortID)
}

func (p PathEnd) String() string {
	return fmt.Sprintf("%s:cl(%s):co(%s):ch(%s):pt(%s)", p.ChainID, p.ClientID, p.ConnectionID, p.ChannelID, p.PortID)
}

// PathSet check if the chain has a path set
func (src *Chain) PathSet() bool {
	return src.PathEnd != nil
}

// PathsSet checks if the chains have their paths set
func PathsSet(chains ...*Chain) bool {
	for _, c := range chains {
		if !c.PathSet() {
			return false
		}
	}
	return true
}

// SetPath sets the path and validates the identifiers
func (src *Chain) SetPath(p *PathEnd) error {
	err := p.Validate()
	if err != nil {
		return src.ErrCantSetPath(err)
	}
	src.PathEnd = p
	return nil
}

// AddPath takes the elements of a path and validates then, setting that path to the chain
func (src *Chain) AddPath(clientID, connectionID, channelID, port, order string) error {
	return src.SetPath(&PathEnd{ChainID: src.ChainID, ClientID: clientID,
		ConnectionID: connectionID, ChannelID: channelID, PortID: port, Order: order})
}

// Validate returns errors about invalid identifiers as well as
// unset path variables for the appropriate type
func (p *PathEnd) Validate() error {
	if err := p.Vclient(); err != nil {
		return err
	}
	if err := p.Vconn(); err != nil {
		return err
	}
	if err := p.Vchan(); err != nil {
		return err
	}
	if err := p.Vport(); err != nil {
		return err
	}
	if !(strings.ToUpper(p.Order) == "ORDERED" || strings.ToUpper(p.Order) == "UNORDERED") {
		return fmt.Errorf("channel must be either 'ORDERED' or 'UNORDERED' is '%s'", p.Order)
	}
	return nil
}

// ErrPathNotSet returns information what identifiers are needed to relay
func (src *Chain) ErrPathNotSet() error {
	return fmt.Errorf("path on chain %s not set", src.ChainID)
}

// ErrCantSetPath returns an error if the path doesn't set properly
func (src *Chain) ErrCantSetPath(err error) error {
	return fmt.Errorf("path on chain %s failed to set: %w", src.ChainID, err)
}
