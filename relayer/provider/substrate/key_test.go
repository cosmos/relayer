package substrate_test

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// TestKeyRestore restores a test mnemonic
func TestKeyRestore(t *testing.T) {
	keyName := "test_key"
	mnemonic := "blind master acoustic speak victory lend kiss grab glad help demand hood roast zone lend sponsor level cheap truck kingdom apology token hover reunion"
	expectedAddress := "5Hn67YZ75F3XrHiJAtiscJMGQ4zFNw9e45CNfLuxL6vEVYz8"

	testProvider, err := getTestProvider()
	require.Nil(t, err)

	err = testProvider.DeleteKey(keyName) // Delete if test is being run again
	require.Nil(t, err)

	address, err := testProvider.RestoreKey(keyName, mnemonic, 0)
	require.Nil(t, err)

	require.Equal(t, expectedAddress, address, "Restored address: %s does not match expected: %s", address, expectedAddress)
}
