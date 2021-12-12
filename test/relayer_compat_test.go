package test

import "testing"

var (
	gaiaChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig},
		{"ibc-1", 1, gaiaTestConfig},
	}
	akashChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig},
		{"ibc-1", 1, akashTestConfig},
	}
	osmoChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig},
		{"ibc-1", 1, osmosisTestConfig},
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
