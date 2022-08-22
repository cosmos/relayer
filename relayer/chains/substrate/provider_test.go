package substrate_test

import (
	"github.com/cosmos/relayer/v2/relayer/chains/substrate"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
)

const homePath = "/tmp"

func getSubstrateConfig(keyDir string, network uint16) *substrate.SubstrateProviderConfig {
	return &substrate.SubstrateProviderConfig{
		Key:            "default",
		ChainID:        "substrate-test",
		KeyringBackend: keystore.BackendTest,
		KeyDirectory:   keyDir,
		Timeout:        "20s",
		Network:        network,
	}
}

func getTestProvider() (*substrate.SubstrateProvider, error) {
	config := getSubstrateConfig(homePath, 42)
	provider, err := config.NewProvider(nil, homePath, true, "substrate")
	if err != nil {
		return nil, err
	}
	return provider.(*substrate.SubstrateProvider), nil
}
