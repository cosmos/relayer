package sei_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/conformance"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"go.uber.org/zap/zaptest"
)

func TestSeiStrideConformance(t *testing.T) {
	ctx := context.Background()

	log := zaptest.NewLogger(t)

	seiConfigFileOverrides := make(map[string]any)
	seiConfigTomlOverrides := make(testutil.Toml)

	seiConfigTomlOverrides["mode"] = "validator"

	seiBlockTime := 100 * time.Millisecond

	consensus := make(testutil.Toml)

	seiBlockT := seiBlockTime.String()
	consensus["timeout-commit"] = seiBlockT
	consensus["timeout-propose"] = seiBlockT
	seiConfigTomlOverrides["consensus"] = consensus

	seiConfigFileOverrides[filepath.Join("config", "config.toml")] = seiConfigTomlOverrides

	nf := 0
	nv := 2

	cf := interchaintest.NewBuiltinChainFactory(log, []*interchaintest.ChainSpec{
		{
			Name:          "stride",
			ChainName:     "stride",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "stride",
				ChainID: "stride-1",
				Images: []ibc.DockerImage{{
					Repository: "ghcr.io/strangelove-ventures/heighliner/stride",
					Version:    "v6.0.0",
					UidGid:     "1025:1025",
				}},
				Bin:            "strided",
				Bech32Prefix:   "stride",
				Denom:          "ustrd",
				GasPrices:      "0.0ustrd",
				TrustingPeriod: "504h",
				GasAdjustment:  1.1,
			}},
		{Name: "sei", ChainName: "sei", Version: "2.0.44beta", NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{
			Type:           "cosmos",
			Name:           "sei",
			ChainID:        "sei-1",
			Bin:            "seid",
			Bech32Prefix:   "sei",
			Denom:          "usei",
			GasPrices:      "0.0usei",
			GasAdjustment:  1.2,
			TrustingPeriod: "504h",
			Images: []ibc.DockerImage{
				{
					Repository: "ghcr.io/strangelove-ventures/heighliner/sei",
					UidGid:     "1025:1025",
					Version:    "2.0.44beta",
				},
			},
			CoinType:            "118",
			SigningAlgorithm:    "sr25519",
			ConfigFileOverrides: seiConfigFileOverrides,
		}},
	})

	conformance.Test(
		t,
		ctx,
		[]interchaintest.ChainFactory{cf},
		[]interchaintest.RelayerFactory{relayerinterchaintest.NewRelayerFactory(relayerinterchaintest.RelayerConfig{})},
		testreporter.NewNopReporter(),
	)
}
