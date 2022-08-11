package relayer

import (
	"fmt"

	host "github.com/cosmos/ibc-go/v5/modules/core/24-host"
)

// Vclient validates the client identifier in the path
func (pe *PathEnd) Vclient() error {
	return host.ClientIdentifierValidator(pe.ClientID)
}

// Vconn validates the connection identifier in the path
func (pe *PathEnd) Vconn() error {
	return host.ConnectionIdentifierValidator(pe.ConnectionID)
}

func (pe PathEnd) String() string {
	return fmt.Sprintf("%s:cl(%s):co(%s)", pe.ChainID, pe.ClientID, pe.ConnectionID)
}

// PathSet check if the chain has a path set
func (c *Chain) PathSet() bool {
	return c.PathEnd != nil
}

// SetPath sets the path and validates the identifiers if they are initialized.
func (c *Chain) SetPath(p *PathEnd) error {
	err := p.ValidateFull()
	if err != nil {
		return c.ErrCantSetPath(err)
	}
	c.PathEnd = p
	return nil
}

// AddPath takes the client and connection identifiers for a Path,
// and if they are initialized, validates them before setting the PathEnd on the Chain.
// NOTE: if the Path is blank (i.e. the identifiers are not set) validation is skipped.
func (c *Chain) AddPath(clientID, connectionID string) error {
	return c.SetPath(&PathEnd{ChainID: c.ChainID(), ClientID: clientID, ConnectionID: connectionID})
}

// ValidateFull returns errors about invalid client and connection identifiers.
func (pe *PathEnd) ValidateFull() error {
	if pe.ClientID != "" {
		if err := pe.Vclient(); err != nil {
			return err
		}
	}

	if pe.ConnectionID != "" {
		if err := pe.Vconn(); err != nil {
			return err
		}
	}
	return nil
}

// ErrPathNotSet returns information what identifiers are needed to relay
func (c *Chain) ErrPathNotSet() error {
	return fmt.Errorf("path on chain %s not set", c.ChainID())
}

// ErrCantSetPath returns an error if the path doesn't set properly
func (c *Chain) ErrCantSetPath(err error) error {
	return fmt.Errorf("path on chain %s failed to set: %w", c.ChainID(), err)
}
