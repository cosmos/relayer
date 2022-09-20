package ibctest_test

import (
	"context"
	"testing"

	relayeribctest "github.com/cosmos/relayer/v2/ibctest"
	"github.com/cosmos/relayer/v2/relayer"
	ibctest "github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/conformance"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/v5/relayer"
	"github.com/strangelove-ventures/ibctest/v5/testreporter"
	"go.uber.org/zap/zaptest"
)

// ibctestConformance runs the ibctest conformance tests against
// the provided RelayerFactory.
//
// This is meant to be a relatively quick sanity check,
// so it uses only one pair of chains.
//
// The canonical set of test chains are defined in the ibctest repository.
func ibctestConformance(t *testing.T, rf ibctest.RelayerFactory) {
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.1", ChainConfig: ibc.ChainConfig{ChainID: "cosmoshub-1004"}},
		{Name: "osmosis", Version: "v7.2.0", ChainConfig: ibc.ChainConfig{ChainID: "osmosis-1001"}},
	})
	conformance.Test(
		t,
		context.Background(),
		[]ibctest.ChainFactory{cf},
		[]ibctest.RelayerFactory{rf},
		testreporter.NewNopReporter(),
	)
}

// TestRelayerInProcess runs the ibctest conformance tests against
// the current state of this relayer implementation running in process.
func TestRelayerInProcess(t *testing.T) {
	t.Parallel()
	ibctestConformance(t, relayeribctest.RelayerFactory{})
}

// TestRelayerDockerEventProcessor runs the ibctest conformance tests against
// the current state of this relayer implementation built in docker.
// Relayer runs using the event processor.
func TestRelayerDockerEventProcessor(t *testing.T) {
	t.Parallel()

	rf := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest", "100:1000"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
	)

	ibctestConformance(t, rf)
}

// TestRelayerDockerLegacyProcessor runs the ibctest conformance tests against
// the current state of this relayer implementation built in docker.
// Relayer runs using the legacy processor.
func TestRelayerDockerLegacyProcessor(t *testing.T) {
	t.Parallel()
	relayeribctest.BuildRelayerImage(t)

	rf := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest", "100:1000"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--processor", "legacy"),
	)

	ibctestConformance(t, rf)
}

// TestRelayerEventProcessor runs the ibctest conformance tests against
// the local relayer code. This is helpful for detecting race conditions.
// Relayer runs using the event processor.
func TestRelayerEventProcessor(t *testing.T) {
	t.Parallel()

	ibctestConformance(t, relayeribctest.NewRelayerFactory(relayeribctest.RelayerConfig{
		Processor:           relayer.ProcessorEvents,
		InitialBlockHistory: 100,
	}))
}

// TestRelayerLegacyProcessor runs the ibctest conformance tests against
// the local relayer code. This is helpful for detecting race conditions.
// Relayer runs using the legacy processor.
func TestRelayerLegacyProcessor(t *testing.T) {
	t.Parallel()

	ibctestConformance(t, relayeribctest.NewRelayerFactory(relayeribctest.RelayerConfig{
		Processor: relayer.ProcessorLegacy,
	}))
}
