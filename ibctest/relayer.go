package ibctest

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
	"github.com/strangelove-ventures/ibctest/ibc"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type Relayer struct {
	t *testing.T

	home string

	// Set during StartRelayer.
	errCh  chan error
	cancel context.CancelFunc
}

func (r *Relayer) sys() *relayertest.System {
	return &relayertest.System{HomeDir: r.home}
}

func (r *Relayer) log() *zap.Logger {
	return zaptest.NewLogger(r.t)
}

func (r *Relayer) AddChainConfiguration(ctx context.Context, _ ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	sys := &relayertest.System{HomeDir: r.home}
	sys.MustAddChain(r.t, chainConfig.ChainID, cmd.ProviderConfigWrapper{
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

func (r *Relayer) AddKey(ctx context.Context, _ ibc.RelayerExecReporter, chainID, keyName string) (ibc.RelayerWallet, error) {
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

func (r *Relayer) RestoreKey(ctx context.Context, _ ibc.RelayerExecReporter, chainID, keyName, mnemonic string) error {
	res := r.sys().RunC(ctx, r.log(), "keys", "restore", chainID, keyName, mnemonic)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) GeneratePath(ctx context.Context, _ ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "paths", "new", srcChainID, dstChainID, pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) GetChannels(ctx context.Context, _ ibc.RelayerExecReporter, chainID string) ([]ibc.ChannelOutput, error) {
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

func (r *Relayer) LinkPath(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "link", pathName,
		"--src-port", opts.SourcePortName,
		"--dst-port", opts.DestPortName,
		"--order", opts.Order.String(),
		"--version", opts.Version)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) GetConnections(ctx context.Context, _ ibc.RelayerExecReporter, chainID string) (ibc.ConnectionOutputs, error) {
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

func (r *Relayer) CreateChannel(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	res := r.sys().RunC(
		ctx, r.log(),
		"tx", "channel", pathName,
		"--src-port", opts.SourcePortName,
		"--dst-port", opts.DestPortName,
		"--order", opts.Order.String(),
		"--version", opts.Version,
	)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) CreateConnections(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "connection", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) CreateClients(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "clients", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) UpdateClients(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "update-clients", pathName)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) StartRelayer(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	if r.errCh != nil || r.cancel != nil {
		panic(fmt.Errorf("StartRelayer called multiple times without being stopped"))
	}

	r.errCh = make(chan error, 1)
	ctx, r.cancel = context.WithCancel(ctx)

	go r.start(ctx, pathName)
	return nil
}

func (r *Relayer) StopRelayer(ctx context.Context, _ ibc.RelayerExecReporter) error {
	r.cancel()
	err := <-r.errCh

	r.cancel = nil
	r.errCh = nil
	return err
}

// start runs in its own goroutine, blocking until "rly start" finishes.
func (r *Relayer) start(ctx context.Context, pathName string) {
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

func (r *Relayer) UseDockerNetwork() bool { return false }

func (r *Relayer) FlushAcknowledgements(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, channelID string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "relay-acks", pathName, channelID)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) FlushPackets(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, channelID string) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "relay-pkts", pathName, channelID)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) GetWallet(chainID string) (ibc.RelayerWallet, bool) {
	res := r.sys().RunC(context.Background(), r.log(), "keys", "show", chainID)
	if res.Err != nil {
		return ibc.RelayerWallet{}, false
	}
	address := strings.TrimSpace(res.Stdout.String())
	return ibc.RelayerWallet{Address: address}, true
}
