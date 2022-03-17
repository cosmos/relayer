package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChainsList_Empty(t *testing.T) {
	sys := NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	res := sys.MustRun(t, "chains", "list")

	// Before adding any chains, attempting to list the chains gives a helpful message on stderr.
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no chains found")
}
