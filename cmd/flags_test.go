package cmd

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"
)

func TestFlagEqualityAgainstSDK(t *testing.T) {
	require.Equal(t, flagHome, flags.FlagHome)
	require.Equal(t, flagOffset, flags.FlagOffset)
	require.Equal(t, flagLimit, flags.FlagLimit)
	require.Equal(t, flagHeight, flags.FlagHeight)
	require.Equal(t, flagPage, flags.FlagPage)
	require.Equal(t, flagPageKey, flags.FlagPageKey)
	require.Equal(t, flagCountTotal, flags.FlagCountTotal)
	require.Equal(t, flagReverse, flags.FlagReverse)
}
