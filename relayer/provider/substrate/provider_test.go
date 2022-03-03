package substrate_test

import (
	"fmt"
	"testing"

	"github.com/cosmos/relayer/relayer/provider/substrate"
	"github.com/cosmos/relayer/relayer/provider/substrate/keystore"
	"github.com/stretchr/testify/require"
)

const homePath = "/tmp"

func TestGetTrustingPeriod(t *testing.T) {
	testProvider, err := getTestProvider()
	require.NoError(t, err)
	tp, err := testProvider.TrustingPeriod()
	fmt.Println(tp)
	require.NoError(t, err)
}

func getSubstrateConfig(keyHome string, debug bool) *substrate.SubstrateProviderConfig {
	return &substrate.SubstrateProviderConfig{
		Key:     "default",
		ChainID: "substrate-test",
		// TODO set RPC address
		RPCAddr:        "",
		KeyringBackend: keystore.BackendMemory,
		KeyDirectory:   keyHome,
		Timeout:        "20s",
	}
}

func getTestProvider() (*substrate.SubstrateProvider, error) {
	testProvider, err := substrate.NewSubstrateProvider(getSubstrateConfig(homePath, true), "")
	return testProvider, err

}
