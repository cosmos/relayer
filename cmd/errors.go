package cmd

import (
	"errors"
	"fmt"
)

func wrapInitFailed(err error) error {
	return fmt.Errorf("init failed: %w", err)
}

func errKeyExists(name string) error {
	return fmt.Errorf("a key with name %s already exists", name)
}

func errKeyDoesntExist(name string) error {
	return fmt.Errorf("a key with name %s doesn't exist", name)
}

var (
	errInitWrongFlags   = errors.New("expected either (--hash/-x & --height) OR --url/-u OR --force/-f, none given")
	errMultipleAddFlags = errors.New("expected either --file/-f OR --url/u, found multiple")
)
