package cmd

import "fmt"

// NewChainDoesNotExistError returns the proper error when a chain doesn't exist in config
func NewChainDoesNotExistError(chainid string) error {
	return fmt.Errorf("chain with ID %s is not configured in %s: %s", chainid, cfgPath)
}
