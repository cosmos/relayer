package ibctest_test

import (
	"io"
	"testing"

	relayeribctest "github.com/cosmos/relayer/v2/ibctest"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/conformance"
	"github.com/strangelove-ventures/ibctest/ibc"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/relayer"
	"github.com/strangelove-ventures/ibctest/testreporter"
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
		[]ibctest.ChainFactory{cf},
		[]ibctest.RelayerFactory{rf},
		// The nop write closer means no test report will be generated,
		// which is fine for these tests for now.
		testreporter.NewReporter(newNopWriteCloser()),
	)
}

// TestRelayerInProcess runs the ibctest conformance tests against
// the current state of this relayer implementation running in process.
func TestRelayerInProcess(t *testing.T) {
	ibctestConformance(t, relayeribctest.RelayerFactory{})
}

// TestRelayerDocker runs the ibctest conformance tests against
// the current state of this relayer implementation built in docker.
// Relayer runs using the event processor.
func TestRelayerDockerEventProcessor(t *testing.T) {
	t.Parallel()
	relayeribctest.BuildRelayerImage(t)

	rf := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
	)

	ibctestConformance(t, rf)
}

// TestRelayerDocker runs the ibctest conformance tests against
// the current state of this relayer implementation built in docker.
// Relayer runs using the legacy processor.
func TestRelayerDockerLegacyProcessor(t *testing.T) {
	t.Parallel()
	relayeribctest.BuildRelayerImage(t)

	rf := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--processor", "legacy"),
	)

	ibctestConformance(t, rf)
}

// nopWriteCloser is a no-op io.WriteCloser used to satisfy the ibctest TestReporter type.
// Because the relayer is used in-process, all logs are simply streamed to the test log.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

func newNopWriteCloser() io.WriteCloser {
	return nopWriteCloser{Writer: io.Discard}
}
