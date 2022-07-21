package test

import "testing"

var (
	gaiaChains = []testChain{
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}
	akashChains = []testChain{
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, akashTestConfig, akashProviderCfg},
	}
	osmoChains = []testChain{
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, osmosisTestConfig, osmosisProviderCfg},
	}
)

func TestGaiaToGaiaRelaying(t *testing.T) {
	chainTest(t, gaiaChains)
}

func TestAkashToGaiaRelaying(t *testing.T) {
	chainTest(t, akashChains)
}

func TestOsmoToGaiaRelaying(t *testing.T) {
	chainTest(t, osmoChains)
}
