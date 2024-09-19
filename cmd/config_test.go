package cmd_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
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

	tests := []struct {
		setting       string
		wantedPresent bool
	}{
		{
			"debug-listen-addr: 127.0.0.1:5183",
			true,
		},
		{
			"metrics-listen-addr: 127.0.0.1:5184",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.setting, func(t *testing.T) {
			sys := setupRelayer(t)

			configFile := fmt.Sprintf("%s/config/config.yaml", sys.HomeDir)
			data, err := os.ReadFile(configFile)
			require.NoError(t, err)
			config := string(data)

			require.Contains(t, config, tt.setting)
		})
	}
}
