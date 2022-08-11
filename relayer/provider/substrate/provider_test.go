package substrate_test

import (
	"context"
	"testing"

	"github.com/cosmos/relayer/v2/relayer/provider/substrate"
	"github.com/cosmos/relayer/v2/relayer/provider/substrate/keystore"
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

func getSubstrateConfig(keyHome string, debug bool) *substrate.SubstrateProviderConfig {
	return &substrate.SubstrateProviderConfig{
		Key:            "default",
		ChainID:        "substrate-test",
		RPCAddr:        rpcAddress,
		KeyringBackend: keystore.BackendTest,
		KeyDirectory:   keyHome,
		Timeout:        "20s",
	}
}

func getTestProvider() (*substrate.SubstrateProvider, error) {
	testProvider, err := substrate.NewSubstrateProvider(getSubstrateConfig(homePath, true), "")
	return testProvider, err

}
