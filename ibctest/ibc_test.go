package ibctest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/conformance"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/label"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/relayer"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestRelayer runs the ibctest conformance tests against
// the current state of this relayer implementation.
//
// This is meant to be a relatively quick sanity check,
// so it uses only one pair of chains.
//
// The canonical set of test chains are defined in the ibctest repository.
func TestRelayer(t *testing.T) {
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.1", ChainConfig: ibc.ChainConfig{ChainID: "cosmoshub-1004"}},
		{Name: "osmosis", Version: "v7.2.0", ChainConfig: ibc.ChainConfig{ChainID: "osmosis-1001"}},
	})
	conformance.Test(
		t,
		[]ibctest.ChainFactory{cf},
		[]ibctest.RelayerFactory{relayerFactory{}},
		// The nop write closer means no test report will be generated,
		// which is fine for these tests for now.
		testreporter.NewReporter(newNopWriteCloser()),
	)
}

// relayerFactory implements the ibctest RelayerFactory interface.
type relayerFactory struct{}

// Build returns a relayer interface
func (relayerFactory) Build(
	t *testing.T,
	pool *dockertest.Pool,
	networkID string,
	home string,
) ibc.Relayer {
	r := &relayer{
		t: t,

		pool: pool,

		home: home,
	}

	res := r.sys().Run(zaptest.NewLogger(t), "config", "init")
	if res.Err != nil {
		panic(fmt.Errorf("failed to rly config init: %w", res.Err))
	}

	return r
}

func (relayerFactory) Capabilities() map[ibctestrelayer.Capability]bool {
	// It is currently expected that the main branch of the relayer supports all tested features.
	return ibctestrelayer.FullCapabilities()
}

func (relayerFactory) Labels() []label.Relayer {
	return []label.Relayer{label.Rly}
}

func (relayerFactory) Name() string { return "github.com/cosmos/relayer" }

type relayer struct {
	t *testing.T

	pool *dockertest.Pool

	home string

	// Set during StartRelayer.
	errCh  chan error
	cancel context.CancelFunc
}

func (r *relayer) sys() *relayertest.System {
	return &relayertest.System{HomeDir: r.home}
}

func (r *relayer) log() *zap.Logger {
	return zaptest.NewLogger(r.t)
}

func (r *relayer) AddChainConfiguration(ctx context.Context, _ ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	sys := &relayertest.System{HomeDir: r.home}
	sys.MustAddChain(r.t, cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			Key:     keyName,
			ChainID: chainConfig.ChainID,
			RPCAddr: rpcAddr,
			// GRPCAddr: grpcAddr, // Not part of relayer cosmos provider config (yet)
			AccountPrefix:  chainConfig.Bech32Prefix,
			KeyringBackend: keyring.BackendTest,
			GasAdjustment:  chainConfig.GasAdjustment,
			GasPrices:      chainConfig.GasPrices,
			Debug:          true,
			Timeout:        "10s",
			OutputFormat:   "json",
			SignModeStr:    "direct",
		},
	})

	return nil
}

func (r *relayer) AddKey(ctx context.Context, _ ibc.RelayerExecReporter, chainID, keyName string) (ibc.RelayerWallet, error) {
	res := r.sys().RunC(ctx, r.log(), "keys", "add", chainID, keyName)
	if res.Err != nil {
		return ibc.RelayerWallet{}, res.Err
	}

	var w ibc.RelayerWallet
	if err := json.Unmarshal(res.Stdout.Bytes(), &w); err != nil {
		return ibc.RelayerWallet{}, err
	}

	return w, nil
}

func (r *relayer) RestoreKey(ctx context.Context, _ ibc.RelayerExecReporter, chainID, keyName, mnemonic string) error {
	res := r.sys().RunC(ctx, r.log(), "keys", "restore", chainID, keyName, mnemonic)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) GeneratePath(ctx context.Context, _ ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "paths", "new", srcChainID, dstChainID, pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) ClearQueue(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, channelID string) error {
	panic("not yet implemented")
}

func (r *relayer) GetChannels(ctx context.Context, _ ibc.RelayerExecReporter, chainID string) ([]ibc.ChannelOutput, error) {
	res := r.sys().RunC(ctx, r.log(), "q", "channels", chainID)
	if res.Err != nil {
		return nil, res.Err
	}

	var channels []ibc.ChannelOutput
	for _, channel := range strings.Split(res.Stdout.String(), "\n") {
		if strings.TrimSpace(channel) == "" {
			continue
		}
		var channelOutput ibc.ChannelOutput
		if err := json.Unmarshal([]byte(channel), &channelOutput); err != nil {
			return nil, fmt.Errorf("failed to parse channel %q: %w", channel, err)
		}
		channels = append(channels, channelOutput)
	}

	return channels, nil
}

func (r *relayer) LinkPath(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "link", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) GetConnections(ctx context.Context, _ ibc.RelayerExecReporter, chainID string) (ibc.ConnectionOutputs, error) {
	res := r.sys().RunC(ctx, r.log(), "q", "connections", chainID)
	if res.Err != nil {
		return nil, res.Err
	}

	var connections ibc.ConnectionOutputs
	for _, connection := range strings.Split(res.Stdout.String(), "\n") {
		if strings.TrimSpace(connection) == "" {
			continue
		}

		connectionOutput := ibc.ConnectionOutput{}
		err := json.Unmarshal([]byte(connection), &connectionOutput)
		if err != nil {
			r.log().Error(
				"Error parsing connection json",
				zap.Error(err),
			)

			continue
		}
		connections = append(connections, &connectionOutput)
	}

	return connections, nil
}

func (r *relayer) CreateChannel(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	res := r.sys().RunC(
		ctx, r.log(),
		"tx", "channel", pathName,
		"--src-port", opts.SourcePortName,
		"--dst-port", opts.DestPortName,
		"--order", opts.Order,
		"--version", opts.Version,
	)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) CreateConnections(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "connection", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) CreateClients(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "clients", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) UpdateClients(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "update-clients", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) StartRelayer(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	if r.errCh != nil || r.cancel != nil {
		panic(fmt.Errorf("StartRelayer called multiple times without being stopped"))
	}

	r.errCh = make(chan error, 1)
	ctx, r.cancel = context.WithCancel(ctx)

	go r.start(ctx, pathName)
	return nil
}

func (r *relayer) StopRelayer(ctx context.Context, _ ibc.RelayerExecReporter) error {
	r.cancel()
	err := <-r.errCh

	r.cancel = nil
	r.errCh = nil
	return err
}

// start runs in its own goroutine, blocking until "rly start" finishes.
func (r *relayer) start(ctx context.Context, pathName string) {
	// Start the debug server on a random port.
	// It won't be reachable without introspecting the output,
	// but this will allow catching any possible data races around the debug server.
	res := r.sys().RunC(ctx, r.log(), "start", pathName, "--debug-addr", "localhost:0")
	if res.Err != nil {
		r.errCh <- res.Err
		return
	}
	r.errCh <- nil
}

func (r *relayer) UseDockerNetwork() bool { return false }

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
