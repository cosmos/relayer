package ibctest

import (
	"fmt"
	"testing"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/label"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/v5/relayer"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// RelayerFactory implements the ibctest RelayerFactory interface.
type RelayerFactory struct{}

// Build returns a relayer interface
func (RelayerFactory) Build(
	t *testing.T,
	_ *client.Client,
	networkID string,
) ibc.Relayer {
	r := &Relayer{
		t:    t,
		home: t.TempDir(),
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

// LocalRelayerFactory implements the ibctest RelayerFactory interface.
type LocalRelayerFactory struct {
	log    *zap.Logger
	config LocalRelayerConfig
}

// LocalRelayerConfig defines parameters for customizing a LocalRelayer.
type LocalRelayerConfig struct {
	Processor           string
	Memo                string
	MaxTxSize           uint64
	MaxMsgLength        uint64
	InitialBlockHistory uint64
}

// NewLocalRelayerFactory returns a factory for creating LocalRelayers for tests
func NewLocalRelayerFactory(log *zap.Logger, config LocalRelayerConfig) *LocalRelayerFactory {
	return &LocalRelayerFactory{log: log, config: config}
}

// Build returns a Relayer associated with the given arguments.
func (rf *LocalRelayerFactory) Build(
	t *testing.T,
	cli *client.Client,
	networkID string,
) ibc.Relayer {
	return NewLocalRelayer(rf.log, t.TempDir(), rf.config)
}

// Name returns a descriptive name of the factory,
// indicating details of the Relayer that will be built.
func (rf *LocalRelayerFactory) Name() string {
	return "LocalRelayer"
}

// Labels are reported to allow simple filtering of tests depending on this Relayer.
// While the Name should be fully descriptive,
// the Labels are intended to be short and fixed.
//
// Most relayers will probably only have one label indicative of its name,
// but we allow multiple labels for future compatibility.
func (rf *LocalRelayerFactory) Labels() []label.Relayer {
	return []label.Relayer{label.Rly}
}

// Capabilities is an indication of the features this relayer supports.
// Tests for any unsupported features will be skipped rather than failed.
func (LocalRelayerFactory) Capabilities() map[ibctestrelayer.Capability]bool {
	// It is currently expected that the main branch of the relayer supports all tested features.
	return ibctestrelayer.FullCapabilities()
}
