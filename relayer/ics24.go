package relayer

import (
	"fmt"
	"strings"

	host "github.com/cosmos/cosmos-sdk/x/ibc/24-host"
)

// Vclient validates the client identifer in the path
func (p *PathEnd) Vclient() error {
	return host.DefaultClientIdentifierValidator(p.ClientID)
}

// Vconn validates the connection identifer in the path
func (p *PathEnd) Vconn() error {
	return host.DefaultConnectionIdentifierValidator(p.ConnectionID)
}

// Vchan validates the channel identifer in the path
func (p *PathEnd) Vchan() error {
	return host.DefaultChannelIdentifierValidator(p.ChannelID)
}

// Vport validates the port identifer in the path
func (p *PathEnd) Vport() error {
	return host.DefaultPortIdentifierValidator(p.PortID)
}

func (p PathEnd) String() string {
	return fmt.Sprintf("%s:cl(%s):co(%s):ch(%s):pt(%s)", p.ChainID, p.ClientID, p.ConnectionID, p.ChannelID, p.PortID)
}

// PathSet check if the chain has a path set
func (c *Chain) PathSet() bool {
	return c.PathEnd != nil
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
func (c *Chain) SetPath(p *PathEnd) error {
	err := p.Validate()
	if err != nil {
		return c.ErrCantSetPath(err)
	}
	c.PathEnd = p
	return nil
}

// AddPath takes the elements of a path and validates then, setting that path to the chain
func (c *Chain) AddPath(clientID, connectionID, channelID, port, order string) error {
	return c.SetPath(&PathEnd{ChainID: c.ChainID, ClientID: clientID, ConnectionID: connectionID, ChannelID: channelID, PortID: port, Order: order})
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
func (c *Chain) ErrPathNotSet() error {
	return fmt.Errorf("Path on chain %s not set", c.ChainID)
}

// ErrCantSetPath returns an error if the path doesn't set properly
func (c *Chain) ErrCantSetPath(err error) error {
	return fmt.Errorf("Path on chain %s failed to set: %w", c.ChainID, err)
}
