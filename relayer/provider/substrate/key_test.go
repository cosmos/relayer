package substrate_test

import (
	"testing"
)

// TestKeyRestore restores a test mnemonic
func TestKeyRestore(t *testing.T) {
	keyName := "test_key"
	mnemonic := "blind master acoustic speak victory lend kiss grab glad help demand hood roast zone lend sponsor level cheap truck kingdom apology token hover reunion"
	expectedAddress := "5Hn67YZ75F3XrHiJAtiscJMGQ4zFNw9e45CNfLuxL6vEVYz8"

	testProvider, err := getTestProvider()
	_ = testProvider.DeleteKey(keyName) // Delete if test is being run again
	address, err := testProvider.RestoreKey(keyName, mnemonic)
	if err != nil {
		t.Fatalf("Error while restoring mnemonic: %v", err)
	}
	if address != expectedAddress {
		t.Fatalf("Restored address: %s does not match expected: %s", address, expectedAddress)
	}
}
