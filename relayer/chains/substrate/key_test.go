package substrate_test

import (
	"testing"

	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"

	"github.com/stretchr/testify/require"
)

// TestKeyRestore restores a test mnemonic
func TestKeyRestoreAndRetrieve(t *testing.T) {
	keyName := "test_key"
	mnemonic := "blind master acoustic speak victory lend kiss grab glad help demand hood roast zone lend sponsor level cheap truck kingdom apology token hover reunion"
	expectedAddress := "5Hn67YZ75F3XrHiJAtiscJMGQ4zFNw9e45CNfLuxL6vEVYz8"

	provider, err := getTestProvider()
	require.Nil(t, err)

	config := getSubstrateConfig(homePath, 42)
	provider.Keybase, err = keystore.New(config.ChainName, config.KeyringBackend, config.KeyDirectory, nil)
	require.Nil(t, err)

	if provider.KeyExists(keyName) {
		err = provider.DeleteKey(keyName) // Delete if test is being run again
		require.Nil(t, err)
	}

	address, err := provider.RestoreKey(keyName, mnemonic, 0)
	require.Nil(t, err)
	require.Equal(t, expectedAddress, address, "Restored address: %s does not match expected: %s", address, expectedAddress)

	retrievedAddress, err := provider.ShowAddress(keyName)
	require.Nil(t, err)
	require.Equal(t, expectedAddress, address, "Restored address: %s does not match expected: %s", retrievedAddress, expectedAddress)
}
