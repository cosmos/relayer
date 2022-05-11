package substrate_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyRestore restores a test mnemonic
func TestKeyRestore(t *testing.T) {
	keyName := "test_key"
	mnemonic := "blind master acoustic speak victory lend kiss grab glad help demand hood roast zone lend sponsor level cheap truck kingdom apology token hover reunion"
	expectedAddress := "5Hn67YZ75F3XrHiJAtiscJMGQ4zFNw9e45CNfLuxL6vEVYz8"

	testProvider, err := getTestProvider()
	require.Nil(t, err)

	if testProvider.KeyExists(keyName) {
		err = testProvider.DeleteKey(keyName) // Delete if test is being run again
		require.Nil(t, err)
	}

	address, err := testProvider.RestoreKey(keyName, mnemonic, 0)
	require.Nil(t, err)

	require.Equal(t, expectedAddress, address, "Restored address: %s does not match expected: %s", address, expectedAddress)
}

func TestKeyInsert(t *testing.T) {
	keyName := "test_key"
	mnemonic := "blind master acoustic speak victory lend kiss grab glad help demand hood roast zone lend sponsor level cheap truck kingdom apology token hover reunion"
	expectedAddress := "5Hn67YZ75F3XrHiJAtiscJMGQ4zFNw9e45CNfLuxL6vEVYz8"

	testProvider, err := getTestProvider()
	require.Nil(t, err)

	if testProvider.KeyExists(keyName) {
		err = testProvider.DeleteKey(keyName) // Delete if test is being run again
		require.Nil(t, err)
	}

	address, err := testProvider.RestoreKey(keyName, mnemonic, 0)
	require.Nil(t, err)

	retrievedAddress, err := testProvider.ShowAddress(keyName)
	require.Nil(t, err)

	require.Equal(t, expectedAddress, address, "Restored address: %s does not match expected: %s", retrievedAddress, expectedAddress)
}
