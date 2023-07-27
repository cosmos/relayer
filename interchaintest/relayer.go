package interchaintest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	interchaintestcosmos "github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer/rly"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type Relayer struct {
	t *testing.T

	config RelayerConfig
	home   string

	// Set during StartRelayer.
	errCh  chan error
	cancel context.CancelFunc
}

// Build returns a relayer interface
func NewRelayer(
	t *testing.T,
	config RelayerConfig,
) ibc.Relayer {
	//prevent incorrect bech32 address prefixed addresses when calling AccAddress.String()
	types.SetAddrCacheEnabled(false)

	r := &Relayer{
		t:      t,
		home:   t.TempDir(),
		config: config,
	}

	res := r.sys().Run(zaptest.NewLogger(t), "config", "init", "--memo", config.Memo)
	if res.Err != nil {
		t.Fatalf("failed to rly config init: %v", res.Err)
	}

	return r
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
			// MinGasAmount: chainConfig.MinGasAmount, // TODO
			Debug:        true,
			Timeout:      "10s",
			OutputFormat: "json",
			SignModeStr:  "direct",
			Broadcast:    provider.BroadcastModeBatch,
		},
	})

	return nil
}

func (r *Relayer) AddKey(ctx context.Context, _ ibc.RelayerExecReporter, chainID, keyName, coinType string) (ibc.Wallet, error) {
	res := r.sys().RunC(ctx, r.log(), "keys", "add", chainID, keyName, "--coin-type", coinType)
	if res.Err != nil {
		return nil, res.Err
	}

	var w *interchaintestcosmos.CosmosWallet
	if err := json.Unmarshal(res.Stdout.Bytes(), &w); err != nil {
		return nil, err
	}

	return w, nil
}

func (r *Relayer) RestoreKey(ctx context.Context, _ ibc.RelayerExecReporter, cfg ibc.ChainConfig, keyName, mnemonic string) error {
	res := r.sys().RunC(ctx, r.log(), "keys", "restore", cfg.ChainID, keyName, mnemonic, "--coin-type", cfg.CoinType)
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

func (r *Relayer) UpdatePath(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, filter ibc.ChannelFilter) error {
	res := r.sys().RunC(ctx, r.log(), "paths", "update", pathName,
		"--filter-rule", filter.Rule,
		"--filter-channels", strings.Join(filter.ChannelList, ","),
	)
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

func (r *Relayer) GetClients(ctx context.Context, _ ibc.RelayerExecReporter, chainID string) (ibc.ClientOutputs, error) {
	res := r.sys().RunC(ctx, r.log(), "q", "clients", chainID)
	if res.Err != nil {
		return nil, res.Err
	}

	var clients []*ibc.ClientOutput
	for _, client := range strings.Split(res.Stdout.String(), "\n") {
		if strings.TrimSpace(client) == "" {
			continue
		}
		clientOutput := &ibc.ClientOutput{}
		if err := json.Unmarshal([]byte(client), clientOutput); err != nil {
			return nil, fmt.Errorf("failed to parse client %q: %w", client, err)
		}
		clients = append(clients, clientOutput)
	}

	return clients, nil
}

func (r *Relayer) LinkPath(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, chanOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "link", pathName,
		"--src-port", chanOpts.SourcePortName,
		"--dst-port", chanOpts.DestPortName,
		"--order", chanOpts.Order.String(),
		"--version", chanOpts.Version,
		"--client-tp", clientOpts.TrustingPeriod,
	)
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

func (r *Relayer) CreateClients(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, clientOpts ibc.CreateClientOptions) error {
	res := r.sys().RunC(ctx, r.log(), "tx", "clients", pathName, "--client-tp", clientOpts.TrustingPeriod)
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

func (r *Relayer) StartRelayer(ctx context.Context, _ ibc.RelayerExecReporter, pathNames ...string) error {
	if r.errCh != nil || r.cancel != nil {
		panic(fmt.Errorf("StartRelayer called multiple times without being stopped"))
	}

	r.errCh = make(chan error, 1)
	ctx, r.cancel = context.WithCancel(ctx)

	if r.config.Processor == "" {
		r.config.Processor = relayer.ProcessorEvents
	}
	args := append([]string{
		"--processor", r.config.Processor,
		"--block-history", strconv.FormatUint(r.config.InitialBlockHistory, 10),
	}, pathNames...)

	go r.start(ctx, args...)
	return nil
}

func (r *Relayer) StopRelayer(ctx context.Context, _ ibc.RelayerExecReporter) error {
	if r.cancel == nil {
		return nil
	}
	r.cancel()
	err := <-r.errCh

	r.cancel = nil
	r.errCh = nil
	return err
}

// start runs in its own goroutine, blocking until "rly start" finishes.
func (r *Relayer) start(ctx context.Context, remainingArgs ...string) {
	// Start the debug server on a random port.
	// It won't be reachable without introspecting the output,
	// but this will allow catching any possible data races around the debug server.
	args := append([]string{"start", "--debug-addr", "localhost:0"}, remainingArgs...)
	res := r.sys().RunC(ctx, r.log(), args...)
	if res.Err != nil {
		r.errCh <- res.Err
		return
	}
	r.errCh <- nil
}

func (r *Relayer) UseDockerNetwork() bool { return false }

func (r *Relayer) Exec(ctx context.Context, _ ibc.RelayerExecReporter, cmd, env []string) ibc.RelayerExecResult {
	// TODO: env would be ignored for now.
	// We may want to modify the call to sys() to accept environment overrides,
	// so this relayer can continue to be used in parallel without environment cross-contamination.
	res := r.sys().RunC(ctx, r.log(), cmd...)

	exitCode := 0
	if res.Err != nil {
		exitCode = 1
	}

	return ibc.RelayerExecResult{
		Err:      res.Err,
		ExitCode: exitCode,
		Stdout:   res.Stdout.Bytes(),
		Stderr:   res.Stderr.Bytes(),
	}
}

func (r *Relayer) Flush(ctx context.Context, _ ibc.RelayerExecReporter, pathName string, channelID string) error {
	cmd := []string{"tx", "flush"}
	if pathName != "" {
		cmd = append(cmd, pathName)
		if channelID != "" {
			cmd = append(cmd, channelID)
		}
	}
	res := r.sys().RunC(ctx, r.log(), cmd...)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) GetWallet(chainID string) (ibc.Wallet, bool) {
	res := r.sys().RunC(context.Background(), r.log(), "keys", "show", chainID)
	if res.Err != nil {
		return &interchaintestcosmos.CosmosWallet{}, false
	}
	address := strings.TrimSpace(res.Stdout.String())

	var keyName string
	config := r.sys().MustGetConfig(r.t)
	for _, v := range config.ProviderConfigs {
		if c, ok := v.Value.(cosmos.CosmosProviderConfig); ok {
			if c.ChainID == chainID {
				keyName = c.Key
			}
		}
	}
	return rly.NewWallet(keyName, address, ""), true
}

// SetClientContractHash sets the wasm client contract hash in the chain's config if the counterparty chain in a path used 08-wasm
// to instantiate the client.
func (r *Relayer) SetClientContractHash(ctx context.Context, rep ibc.RelayerExecReporter, cfg ibc.ChainConfig, hash string) error {
	//TODO implement me
	panic("implement me")
}
