package interchaintest

import (
	"testing"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v7/relayer"
)

// RelayerFactory implements the interchaintest RelayerFactory interface.
type RelayerFactory struct {
	config RelayerConfig
}

// LocalRelayerConfig defines parameters for customizing a LocalRelayer.
type RelayerConfig struct {
	Processor           string
	Memo                string
	InitialBlockHistory uint64
}

func NewRelayerFactory(config RelayerConfig) RelayerFactory {
	return RelayerFactory{
		config: config,
	}
}

// Build returns a relayer interface
func (rf RelayerFactory) Build(
	t *testing.T,
	_ *client.Client,
	networkID string,
) ibc.Relayer {
	return NewRelayer(t, rf.config)
}

func (RelayerFactory) Capabilities() map[interchaintestrelayer.Capability]bool {
	// It is currently expected that the main branch of the relayer supports all tested features.
	return interchaintestrelayer.FullCapabilities()
}

func (RelayerFactory) Name() string { return "github.com/cosmos/relayer" }
