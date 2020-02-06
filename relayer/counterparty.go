package relayer

import (
	"fmt"
)

// NewCounterparty returns a new instance of Counterparty
func NewCounterparty(chainID, clientID string) Counterparty {
	return Counterparty{chainID, clientID}
}

// Counterparty represents the counterparty to relay against for
type Counterparty struct {
	ChainID  string `yaml:"chain-id"`
	ClientID string `yaml:"client-id"`
}

// GetCounterparty returns the specified counterparty from a given chain
func (c *Chain) GetCounterparty(chainID string) (Counterparty, error) {
	for _, cp := range c.Counterparties {
		if cp.ChainID == chainID {
			return cp, nil
		}
	}
	return Counterparty{}, fmt.Errorf("chain %s has no counterparty with id %s", c.ChainID, chainID)
}
