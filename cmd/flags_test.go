package cmd

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"
)

// TestFlagEqualityAgainstSDK makes assertions against our local flags and the corresponding flags from
// the Cosmos SDK to ensure that the CLI experience is consistent across both the SDK and the relayer.
// This allows us to avoid directly depending on the `sdk/client/flags` package inside of our cmd package.
func TestFlagEqualityAgainstSDK(t *testing.T) {
	require.Equal(t, flagHome, flags.FlagHome)
	require.Equal(t, flagLimit, flags.FlagLimit)
	require.Equal(t, flagHeight, flags.FlagHeight)
	require.Equal(t, flagPage, flags.FlagPage)
	require.Equal(t, flagPageKey, flags.FlagPageKey)
	require.Equal(t, flagCountTotal, flags.FlagCountTotal)
	require.Equal(t, flagReverse, flags.FlagReverse)
}
