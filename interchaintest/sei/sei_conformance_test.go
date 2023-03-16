package ibc

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

	cf := interchaintest.NewBuiltinChainFactory(log, []*interchaintest.ChainSpec{
		{Name: "stride", Version: "v6.0.0"},
		{Name: "sei", Version: "2.0.44beta", NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{
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
