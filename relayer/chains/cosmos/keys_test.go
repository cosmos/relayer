package cosmos_test

import (
	"path/filepath"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testProviderWithKeystore(t *testing.T, accountPrefix string, extraCodecs []string) provider.ChainProvider {
	homePath := t.TempDir()
	cfg := cosmos.CosmosProviderConfig{
		ChainID:        "test",
		KeyDirectory:   filepath.Join(homePath, "keys"),
		KeyringBackend: "test",
		Timeout:        "10s",
		AccountPrefix:  accountPrefix,
		ExtraCodecs:    extraCodecs,
	}
	p, err := cfg.NewProvider(zap.NewNop(), homePath, true, "test_chain")
	if err != nil {
		t.Fatalf("Error creating provider: %v", err)
	}
	err = p.CreateKeystore(homePath)
	if err != nil {
		t.Fatalf("Error creating keystore: %v", err)
	}
	return p
}

// TestKeyRestore restores a test mnemonic
func TestKeyRestore(t *testing.T) {
	const (
		keyName            = "test_key"
		signatureAlgorithm = "secp256k1"
		mnemonic           = "blind master acoustic speak victory lend kiss grab glad help demand hood roast zone lend sponsor level cheap truck kingdom apology token hover reunion"
		accountPrefix      = "cosmos"
		expectedAddress    = "cosmos15cw268ckjj2hgq8q3jf68slwjjcjlvxy57je2u"
		coinType           = uint32(118)
	)

	p := testProviderWithKeystore(t, accountPrefix, nil)

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}

// TestKeyRestoreEth restores a test mnemonic
func TestKeyRestoreEth(t *testing.T) {
	const (
		keyName            = "test_key"
		signatureAlgorithm = "secp256k1"
		mnemonic           = "three elevator silk family street child flip also leaf inmate call frame shock little legal october vivid enable fetch siege sell burger dolphin green"
		accountPrefix      = "evmos"
		expectedAddress    = "evmos1dea7vlekr9e34vugwkvesulglt8fx4e457vk9z"
		coinType           = uint32(60)
	)

	p := testProviderWithKeystore(t, accountPrefix, []string{"ethermint"})

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}

// TestKeyRestoreInj restores a test mnemonic
func TestKeyRestoreInj(t *testing.T) {
	const (
		keyName            = "inj_key"
		signatureAlgorithm = "secp256k1"
		mnemonic           = "three elevator silk family street child flip also leaf inmate call frame shock little legal october vivid enable fetch siege sell burger dolphin green"
		accountPrefix      = "inj"
		expectedAddress    = "inj1dea7vlekr9e34vugwkvesulglt8fx4e4uk2udj"
		coinType           = uint32(60)
	)

	p := testProviderWithKeystore(t, accountPrefix, []string{"injective"})

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}

// TestKeyRestoreSr25519 restores a test mnemonic
func TestKeyRestoreSr25519(t *testing.T) {
	const (
		keyName            = "sei_key"
		signatureAlgorithm = "sr25519"
		mnemonic           = "three elevator silk family street child flip also leaf inmate call frame shock little legal october vivid enable fetch siege sell burger dolphin green"
		accountPrefix      = "sei"
		expectedAddress    = "sei1nmlj0guznnt0qyfj4yl6q5g4xuvgly4qw0w026"
		coinType           = uint32(118)
	)

	p := testProviderWithKeystore(t, accountPrefix, nil)

	address, err := p.RestoreKey(keyName, mnemonic, coinType, signatureAlgorithm)
	require.NoError(t, err)
	require.Equal(t, expectedAddress, address)
}
