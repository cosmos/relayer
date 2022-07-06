package ibctest

import (
	"fmt"
	"testing"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/label"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/relayer"
	"go.uber.org/zap/zaptest"
)

// RelayerFactory implements the ibctest RelayerFactory interface.
type RelayerFactory struct{}

// Build returns a relayer interface
func (RelayerFactory) Build(
	t *testing.T,
	_ *client.Client,
	networkID string,
	home string,
) ibc.Relayer {
	r := &Relayer{
		t:    t,
		home: home,
	}

	res := r.sys().Run(zaptest.NewLogger(t), "config", "init")
	if res.Err != nil {
		panic(fmt.Errorf("failed to rly config init: %w", res.Err))
	}

	return r
}

func (RelayerFactory) Capabilities() map[ibctestrelayer.Capability]bool {
	// It is currently expected that the main branch of the relayer supports all tested features.
	return ibctestrelayer.FullCapabilities()
}

func (RelayerFactory) Labels() []label.Relayer {
	return []label.Relayer{label.Rly}
}

func (RelayerFactory) Name() string { return "github.com/cosmos/relayer" }
