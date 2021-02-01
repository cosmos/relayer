package relayer

import (
	"fmt"
	"strings"

	host "github.com/cosmos/cosmos-sdk/x/ibc/core/24-host"
)

// Vclient validates the client identifier in the path
func (pe *PathEnd) Vclient() error {
	return host.ClientIdentifierValidator(pe.ClientID)
}

// Vconn validates the connection identifier in the path
func (pe *PathEnd) Vconn() error {
	return host.ConnectionIdentifierValidator(pe.ConnectionID)
}

// Vchan validates the channel identifier in the path
func (pe *PathEnd) Vchan() error {
	return host.ChannelIdentifierValidator(pe.ChannelID)
}

// Vport validates the port identifier in the path
func (pe *PathEnd) Vport() error {
	return host.PortIdentifierValidator(pe.PortID)
}

// Vversion validates the version identifier in the path
func (pe *PathEnd) Vversion() error {
	// TODO: version validation
	return nil
}

func (pe PathEnd) String() string {
	return fmt.Sprintf("%s:cl(%s):co(%s):ch(%s):pt(%s)", pe.ChainID, pe.ClientID, pe.ConnectionID, pe.ChannelID, pe.PortID)
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
	err := p.ValidateBasic()
	if err != nil {
		return c.ErrCantSetPath(err)
	}
	c.PathEnd = p
	return nil
}

// AddPath takes the elements of a path and validates then, setting that path to the chain
func (c *Chain) AddPath(clientID, connectionID, channelID, port, order string) error {
	return c.SetPath(&PathEnd{ChainID: c.ChainID, ClientID: clientID,
		ConnectionID: connectionID, ChannelID: channelID, PortID: port, Order: order})
}

// ValidateFull returns errors about invalid identifiers as well as
// unset path variables for the appropriate type
func (pe *PathEnd) ValidateFull() error {
	if err := pe.ValidateBasic(); err != nil {
		return err
	}
	if err := pe.Vclient(); err != nil {
		return err
	}
	if err := pe.Vconn(); err != nil {
		return err
	}
	if err := pe.Vchan(); err != nil {
		return err
	}
	return nil
}

// ValidateBasic validates fields that cannot be empty such as the
// port and channel order.
func (pe *PathEnd) ValidateBasic() error {
	if err := pe.Vport(); err != nil {
		return err
	}
	if !(strings.ToUpper(pe.Order) == "ORDERED" || strings.ToUpper(pe.Order) == "UNORDERED") {
		return fmt.Errorf("channel must be either 'ORDERED' or 'UNORDERED' is '%s'", pe.Order)
	}
	return nil
}

// ErrPathNotSet returns information what identifiers are needed to relay
func (c *Chain) ErrPathNotSet() error {
	return fmt.Errorf("path on chain %s not set", c.ChainID)
}

// ErrCantSetPath returns an error if the path doesn't set properly
func (c *Chain) ErrCantSetPath(err error) error {
	return fmt.Errorf("path on chain %s failed to set: %w", c.ChainID, err)
}
