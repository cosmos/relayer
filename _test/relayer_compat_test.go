package test

import "testing"

var (
	gaiaChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}
	akashChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"ibc-1", 1, akashTestConfig, akashProviderCfg},
	}
	osmoChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"ibc-1", 1, osmosisTestConfig, osmosisProviderCfg},
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
