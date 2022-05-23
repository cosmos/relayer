package ibctest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	itfrelayer "github.com/strangelove-ventures/ibc-test-framework/relayer"
	itfrelayertest "github.com/strangelove-ventures/ibc-test-framework/relayertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestRelayer runs the ibc-test-framework relayer tests against
// the current state of this relayer implementation.
//
// This is meant to be a relatively quick sanity check,
// so it uses only one pair of chains.
//
// The canonical set of tests are defined in the ibc-test-framework repository.
func TestRelayer(t *testing.T) {
	cf := ibctest.NewBuiltinChainFactory([]ibctest.BuiltinChainFactoryEntry{
		{Name: "gaia", Version: "v6.0.4", ChainID: "cosmoshub-1004", NumValidators: 2, NumFullNodes: 1},
		{Name: "osmosis", Version: "v7.0.4", ChainID: "osmosis-1001", NumValidators: 2, NumFullNodes: 1},
	})
	itfrelayertest.TestRelayer(t, cf, relayerFactory{})
}

// relayerFactory implements the ibc-test-framework RelayerFactory interface.
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

func (relayerFactory) UseDockerNetwork() bool { return false }

func (relayerFactory) Capabilities() map[itfrelayer.Capability]bool {
	// As of the current version of ibc-testing-framework's relayer tests,
	// this version of the relayer can support everything but the timestamp timeout.
	m := itfrelayer.FullCapabilities()
	// m[itfrelayer.TimestampTimeout] = false
	return m
}

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

func (r *relayer) AddChainConfiguration(ctx context.Context, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
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

func (r *relayer) AddKey(ctx context.Context, chainID, keyName string) (ibc.RelayerWallet, error) {
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

func (r *relayer) RestoreKey(ctx context.Context, chainID, keyName, mnemonic string) error {
	res := r.sys().RunC(ctx, r.log(), "keys", "restore", chainID, keyName, mnemonic)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) GeneratePath(ctx context.Context, srcChainID, dstChainID, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "paths", "new", srcChainID, dstChainID, pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) ClearQueue(ctx context.Context, pathName string, channelID string) error {
	panic("not yet implemented")
}

func (r *relayer) GetChannels(ctx context.Context, chainID string) ([]ibc.ChannelOutput, error) {
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

func (r *relayer) LinkPath(ctx context.Context, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "link", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *relayer) StartRelayer(ctx context.Context, pathName string) error {
	if r.errCh != nil || r.cancel != nil {
		panic(fmt.Errorf("StartRelayer called multiple times without being stopped"))
	}

	r.errCh = make(chan error, 1)
	ctx, r.cancel = context.WithCancel(ctx)

	go r.start(ctx, pathName)
	return nil
}

func (r *relayer) StopRelayer(ctx context.Context) error {
	r.cancel()
	err := <-r.errCh

	r.cancel = nil
	r.errCh = nil
	return err
}

func (r *relayer) UpdateClients(ctx context.Context, pathName string) error {
	panic("not yet implemented")
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
