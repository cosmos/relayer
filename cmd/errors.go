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

var (
	errMultipleAddFlags = errors.New("expected either --file/-f OR --url/u, found multiple")
)
