package cmd_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/stretchr/testify/require"
)

func TestKeysList_Empty(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, "testChain", cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	res := sys.MustRun(t, "keys", "list", "testChain")
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no keys found for chain testChain")
}

func TestKeysRestore_Delete(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, "testChain", cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			AccountPrefix:  "cosmos",
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	// Restore a key with mnemonic to the chain.

	res := sys.MustRun(t, "keys", "restore", "testChain", "default", relayertest.ZeroMnemonic)
	require.Equal(t, res.Stdout.String(), relayertest.ZeroCosmosAddr+"\n")
	require.Empty(t, res.Stderr.String())

	// Restored key must show up in list.
	res = sys.MustRun(t, "keys", "list", "testChain")
	require.Equal(t, res.Stdout.String(), "key(default) -> "+relayertest.ZeroCosmosAddr+"\n")
	require.Empty(t, res.Stderr.String())

	// Deleting the key must succeed.
	res = sys.MustRun(t, "keys", "delete", "testChain", "default", "-y")
	require.Empty(t, res.Stdout.String())
	require.Equal(t, res.Stderr.String(), "key default deleted\n")

	// Listing the keys again gives the no keys warning.
	res = sys.MustRun(t, "keys", "list", "testChain")
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no keys found for chain testChain")
}

func TestKeysExport(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, "testChain", cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			AccountPrefix:  "cosmos",
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	// Restore a key with mnemonic to the chain.
	res := sys.MustRun(t, "keys", "restore", "testChain", "default", relayertest.ZeroMnemonic)
	require.Equal(t, res.Stdout.String(), relayertest.ZeroCosmosAddr+"\n")
	require.Empty(t, res.Stderr.String())

	// Export the key.
	res = sys.MustRun(t, "keys", "export", "testChain", "default")
	armorOut := res.Stdout.String()
	require.Contains(t, armorOut, "BEGIN TENDERMINT PRIVATE KEY")
	require.Empty(t, res.Stderr.String())

	// Import the key to a temporary keyring.
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	kr := keyring.NewInMemory(cdc)
	require.NoError(t, kr.ImportPrivKey("temp", armorOut, keys.DefaultKeyPass))

	// TODO: confirm the imported address matches?
}
