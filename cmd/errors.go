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
	return fmt.Errorf("chain \"%s\" not found in config", chainName)
}

var (
	errMultipleAddFlags = errors.New("expected either --file/-f OR --url/u, found multiple")
)
