package cmd

import (
	"errors"
	"fmt"
)

func errKeyExists(name string) error {
	return fmt.Errorf("a key with name %s already exists", name)
}

func errKeyDoesntExist(name string) error {
	return fmt.Errorf("a key with name %s doesn't exist", name)
}

func errChainNotFound(chainName string) error {
	return fmt.Errorf("chain with name \"%s\" not found in config. consider running `rly chains add %s`", chainName, chainName)
}

var (
	errMultipleAddFlags = errors.New("expected either --file/-f OR --url/u, found multiple")
)
