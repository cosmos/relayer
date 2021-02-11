package helpers

import (
	"errors"
	"fmt"
)

func wrapInitFailed(err error) error {
	return fmt.Errorf("init failed: %w", err)
}

var (
	errInitWrongFlags = errors.New("expected either (--hash/-x & --height) OR --force/-f, none given")
)
