package substrate_test

import (
	"context"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/chains/substrate"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
	"github.com/stretchr/testify/require"
)

const homePath = "/tmp"
const rpcAddress = "ws://127.0.0.1:9944"

func TestGetTrustingPeriod(t *testing.T) {
	testProvider, err := getTestProvider()
	require.NoError(t, err)
	tp, err := testProvider.TrustingPeriod(context.Background())
	require.NoError(t, err)
	require.NotNil(t, tp)
}

func getSubstrateConfig(keyHome string, network uint8, debug bool) *substrate.SubstrateProviderConfig {
	return &substrate.SubstrateProviderConfig{
		Key:            "default",
		ChainID:        "substrate-test",
		RPCAddr:        rpcAddress,
		KeyringBackend: keystore.BackendTest,
		KeyDirectory:   keyHome,
		Timeout:        "20s",
		Network:        network,
	}
}

func getTestProvider() (*substrate.SubstrateProvider, error) {
	config := getSubstrateConfig(homePath, 42, true)
	provider, err := config.NewProvider(nil, homePath, true, "substrate")
	if err != nil {
		return nil, err
	}
	return provider.(*substrate.SubstrateProvider), nil
}
