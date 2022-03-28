package cmd_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/stretchr/testify/require"
)

func TestKeysList_Empty(t *testing.T) {
	t.Parallel()

	sys := NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	res := sys.MustRun(t, "keys", "list", "testcosmos")
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no keys found for chain testcosmos")
}

func TestKeysRestore_Delete(t *testing.T) {
	t.Parallel()

	sys := NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			AccountPrefix:  "cosmos",
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	// Restore a key with mnemonic to the chain.
	res := sys.MustRun(t, "keys", "restore", "testcosmos", "default", ZeroMnemonic)
	require.Equal(t, res.Stdout.String(), ZeroCosmosAddr+"\n")
	require.Empty(t, res.Stderr.String())

	// Restored key must show up in list.
	res = sys.MustRun(t, "keys", "list", "testcosmos")
	require.Equal(t, res.Stdout.String(), "key(default) -> "+ZeroCosmosAddr+"\n")
	require.Empty(t, res.Stderr.String())

	// Deleting the key must succeed.
	res = sys.MustRun(t, "keys", "delete", "testcosmos", "default", "-y")
	require.Empty(t, res.Stdout.String())
	require.Equal(t, res.Stderr.String(), "key default deleted\n")

	// Listing the keys again gives the no keys warning.
	res = sys.MustRun(t, "keys", "list", "testcosmos")
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no keys found for chain testcosmos")
}

func TestKeysExport(t *testing.T) {
	t.Parallel()

	sys := NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			AccountPrefix:  "cosmos",
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	// Restore a key with mnemonic to the chain.
	res := sys.MustRun(t, "keys", "restore", "testcosmos", "default", ZeroMnemonic)
	require.Equal(t, res.Stdout.String(), ZeroCosmosAddr+"\n")
	require.Empty(t, res.Stderr.String())

	// Export the key.
	res = sys.MustRun(t, "keys", "export", "testcosmos", "default")
	armorOut := res.Stdout.String()
	require.Contains(t, armorOut, "BEGIN TENDERMINT PRIVATE KEY")
	require.Empty(t, res.Stderr.String())

	// Import the key to a temporary keyring.
	kr := keyring.NewInMemory()
	require.NoError(t, kr.ImportPrivKey("temp", armorOut, keys.DefaultKeyPass))

	// TODO: confirm the imported address matches?
}
