package ibctest

import (
	"testing"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/label"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/v5/relayer"
)

// RelayerFactory implements the ibctest RelayerFactory interface.
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

func (RelayerFactory) Capabilities() map[ibctestrelayer.Capability]bool {
	// It is currently expected that the main branch of the relayer supports all tested features.
	return ibctestrelayer.FullCapabilities()
}

func (RelayerFactory) Labels() []label.Relayer {
	return []label.Relayer{label.Rly}
}

func (RelayerFactory) Name() string { return "github.com/cosmos/relayer" }
