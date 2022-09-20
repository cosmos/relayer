package ibctest

import (
	"context"
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"go.uber.org/zap"
)

type LocalRelayer struct {
	log *zap.Logger

	processor           string
	memo                string
	maxTxSize           uint64
	maxMsgLength        uint64
	initialBlockHistory uint64

	// in-memory config only
	config *cmd.Config
	home   string

	// Set during StartRelayer.
	errCh  chan error
	cancel context.CancelFunc
}

// NewLocalRelayer returns an implementation of an ibctest ibc.Relayer that uses the local relayer code.
func NewLocalRelayer(log *zap.Logger, home string, config LocalRelayerConfig) *LocalRelayer {
	r := &LocalRelayer{
		log:          log,
		processor:    config.Processor,
		memo:         config.Memo,
		maxTxSize:    config.MaxTxSize,
		maxMsgLength: config.MaxMsgLength,
		config:       cmd.DefaultConfig("LocalRelayer"),
		home:         home,
	}
	if r.processor == "" {
		r.processor = relayer.ProcessorEvents
	}
	if r.memo == "" {
		r.memo = "LocalRelayer"
	}
	if r.maxTxSize == 0 {
		r.maxTxSize = 2 * cmd.MB
	}
	if r.maxMsgLength == 0 {
		r.maxMsgLength = 5
	}
	if r.initialBlockHistory == 0 {
		r.initialBlockHistory = 100
	}
	return r
}

// chainsFromPath fetches a path's chains, ensures the keys exists, then returns the chains.
func (r *LocalRelayer) chainsFromPath(pathName string) (*relayer.Chain, *relayer.Chain, error) {
	pth, err := r.config.Paths.Get(pathName)
	if err != nil {
		return nil, nil, fmt.Errorf("path not found: %s: %w", pathName, err)
	}

	src, dst := pth.Src.ChainID, pth.Dst.ChainID
	c, err := r.config.Chains.Gets(src, dst)
	if err != nil {
		return nil, nil, fmt.Errorf("chain(s) not found: %w", err)
	}

	srcChain := *c[src]
	dstChain := *c[dst]

	// ensure that keys exist
	if exists := srcChain.ChainProvider.KeyExists(srcChain.ChainProvider.Key()); !exists {
		return nil, nil, fmt.Errorf("key %s not found on src chain %s", srcChain.ChainProvider.Key(), srcChain.ChainID())
	}
	if exists := dstChain.ChainProvider.KeyExists(dstChain.ChainProvider.Key()); !exists {
		return nil, nil, fmt.Errorf("key %s not found on dst chain %s", dstChain.ChainProvider.Key(), dstChain.ChainID())
	}

	srcChain.PathEnd = pth.Src
	dstChain.PathEnd = pth.Dst

	return &srcChain, &dstChain, nil
}

func (r *LocalRelayer) AddChainConfiguration(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	chainConfig ibc.ChainConfig,
	keyName string,
	rpcAddr string,
	grpcAddr string) error {
	pcfg := cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			Key:            keyName,
			ChainName:      chainConfig.Name,
			ChainID:        chainConfig.ChainID,
			RPCAddr:        rpcAddr,
			AccountPrefix:  chainConfig.Bech32Prefix,
			KeyringBackend: keyring.BackendTest,
			GasAdjustment:  chainConfig.GasAdjustment,
			GasPrices:      chainConfig.GasPrices,
			// TODO
			// MinGasAmount: chainConfig.minGasAmount,
			Debug:        true,
			Timeout:      "30s",
			OutputFormat: "json",
			SignModeStr:  "direct",
		},
	}
	prov, err := pcfg.Value.NewProvider(r.log, r.home, true, chainConfig.Name)
	if err != nil {
		return fmt.Errorf("failed to build chain provider: %w", err)
	}
	c := relayer.NewChain(r.log, prov, true)
	return r.config.AddChain(c)
}

func (r *LocalRelayer) AddKey(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	chainID string,
	keyName string,
) (ibc.Wallet, error) {
	var zero ibc.Wallet
	chain, err := r.config.Chains.Get(chainID)
	if err != nil {
		return zero, fmt.Errorf("error getting chain configuration for %s: %w", chainID, err)
	}

	if chain.ChainProvider.KeyExists(keyName) {
		return zero, fmt.Errorf("key %s already exists for chain %s", keyName, chainID)
	}

	ko, err := chain.ChainProvider.AddKey(keyName, sdk.CoinType)
	if err != nil {
		return zero, fmt.Errorf("failed to add key %s for chain %s: %w", keyName, chainID, err)
	}

	return ibc.Wallet{
		Mnemonic: ko.Mnemonic,
		Address:  ko.Address,
	}, nil
}

func (r *LocalRelayer) RestoreKey(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	chainID string,
	keyName string,
	mnemonic string,
) error {

	fmt.Printf("RestoreKey for chain: %s keyName: %s\n", chainID, keyName)

	chain, err := r.config.Chains.Get(chainID)
	if err != nil {
		return fmt.Errorf("error getting chain configuration for %s: %w", chainID, err)
	}

	if chain.ChainProvider.KeyExists(keyName) {
		return fmt.Errorf("key %s already exists for chain %s", keyName, chainID)
	}

	_, err = chain.ChainProvider.RestoreKey(keyName, mnemonic, sdk.CoinType)
	if err != nil {
		return fmt.Errorf("failed to add key %s for chain %s: %w", keyName, chainID, err)
	}

	return nil
}

func (r *LocalRelayer) GeneratePath(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	srcChainID,
	dstChainID,
	pathName string,
) error {
	_, err := r.config.Chains.Gets(srcChainID, dstChainID)
	if err != nil {
		return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
	}

	p := &relayer.Path{
		Src: &relayer.PathEnd{ChainID: srcChainID},
		Dst: &relayer.PathEnd{ChainID: dstChainID},
	}

	return r.config.Paths.Add(pathName, p)
}

func (r *LocalRelayer) GetChannels(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	chainID string,
) (out []ibc.ChannelOutput, err error) {
	chain, err := r.config.Chains.Get(chainID)
	if err != nil {
		return nil, fmt.Errorf("error getting chain configuration for %s: %w", chainID, err)
	}

	res, err := chain.ChainProvider.QueryChannels(ctx)
	if err != nil {
		return nil, err
	}

	for _, c := range res {
		out = append(out, ibc.ChannelOutput{
			PortID:    c.PortId,
			ChannelID: c.ChannelId,
			State:     c.State.String(),
			Ordering:  c.Ordering.String(),
			Counterparty: ibc.ChannelCounterparty{
				PortID:    c.Counterparty.PortId,
				ChannelID: c.Counterparty.ChannelId,
			},
			ConnectionHops: c.ConnectionHops,
			Version:        c.Version,
		})
	}

	return out, nil
}

func (r *LocalRelayer) LinkPath(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	pathName string,
	chanOpts ibc.CreateChannelOptions,
	clientOpts ibc.CreateClientOptions,
) error {
	// TODO make these configurable
	const (
		allowUpdateAfterExpiry       = true
		allowUpdateAfterMisbehaviour = true
		override                     = false
		retries                      = uint64(3)
		to                           = 10 * time.Second
	)

	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	trustingPeriod, err := time.ParseDuration(clientOpts.TrustingPeriod)
	if err != nil {
		return fmt.Errorf("error parsing trusting period: %w", err)
	}

	// create clients if they aren't already created
	clientSrc, clientDst, err := srcChain.CreateClients(ctx, dstChain, allowUpdateAfterExpiry,
		allowUpdateAfterMisbehaviour, override, trustingPeriod, r.memo)
	if err != nil {
		return fmt.Errorf("error creating clients: %w", err)
	}
	path := r.config.Paths[pathName]
	if clientSrc != "" {
		path.Src.ClientID = clientSrc
	}
	if clientDst != "" {
		path.Dst.ClientID = clientDst
	}

	// create connection if it isn't already created
	connectionSrc, connectionDst, err := srcChain.CreateOpenConnections(ctx,
		dstChain, retries, to, r.memo, 0, pathName)

	if err != nil {
		return fmt.Errorf("error creating connections: %w", err)
	}
	path = r.config.Paths[pathName]
	if connectionSrc != "" {
		path.Src.ConnectionID = connectionSrc
	}
	if connectionDst != "" {
		path.Dst.ConnectionID = connectionSrc
	}

	// create channel if it isn't already created
	return srcChain.CreateOpenChannels(ctx, dstChain, retries, to, chanOpts.SourcePortName,
		chanOpts.DestPortName, chanOpts.Order.String(), chanOpts.Version, override, r.memo, pathName)
}

func (r *LocalRelayer) GetConnections(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	chainID string,
) (out ibc.ConnectionOutputs, err error) {
	chain, err := r.config.Chains.Get(chainID)
	if err != nil {
		return nil, fmt.Errorf("error getting chain configuration for %s: %w", chainID, err)
	}

	conns, err := chain.ChainProvider.QueryConnections(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying connections: %w", err)
	}

	for _, c := range conns {
		out = append(out, &ibc.ConnectionOutput{
			ID:       c.Id,
			ClientID: c.ClientId,
			Versions: c.Versions,
			State:    c.State.String(),
			Counterparty: &types.Counterparty{
				ConnectionId: c.Counterparty.ConnectionId,
				ClientId:     c.Counterparty.ClientId,
			},
			DelayPeriod: time.Duration(c.DelayPeriod).String(),
		})
	}

	return out, nil
}

func (r *LocalRelayer) CreateChannel(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	pathName string,
	opts ibc.CreateChannelOptions,
) error {
	// TODO make these configurable
	const (
		override = false
		retries  = uint64(3)
		to       = 10 * time.Second
	)

	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	// create channel if it isn't already created
	return srcChain.CreateOpenChannels(ctx, dstChain, retries, to, opts.SourcePortName,
		opts.DestPortName, opts.Order.String(), opts.Version, override, r.memo, pathName)
}

func (r *LocalRelayer) CreateConnections(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	// TODO make these configurable
	const (
		override            = false
		retries             = uint64(3)
		to                  = 10 * time.Second
		initialBlockHistory = uint64(0)
	)

	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	// create connection if it isn't already created
	connectionSrc, connectionDst, err := srcChain.CreateOpenConnections(ctx,
		dstChain, retries, to, r.memo, initialBlockHistory, pathName)

	if err != nil {
		return fmt.Errorf("error creating connections: %w", err)
	}
	path := r.config.Paths[pathName]
	if connectionSrc != "" {
		path.Src.ConnectionID = connectionSrc
	}
	if connectionDst != "" {
		path.Dst.ConnectionID = connectionSrc
	}
	return nil
}

func (r *LocalRelayer) CreateClients(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	pathName string,
	clientOpts ibc.CreateClientOptions,
) error {
	// TODO make these configurable
	const (
		allowUpdateAfterExpiry       = true
		allowUpdateAfterMisbehaviour = true
		override                     = false
		retries                      = uint64(3)
		to                           = 10 * time.Second
		initialBlockHistory          = uint64(0)
	)

	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	trustingPeriod, err := time.ParseDuration(clientOpts.TrustingPeriod)
	if err != nil {
		return fmt.Errorf("error parsing trusting period: %w", err)
	}

	// create clients if they aren't already created
	clientSrc, clientDst, err := srcChain.CreateClients(ctx, dstChain, allowUpdateAfterExpiry,
		allowUpdateAfterMisbehaviour, override, trustingPeriod, r.memo)

	if err != nil {
		return fmt.Errorf("error creating clients: %w", err)
	}
	path := r.config.Paths[pathName]
	if clientSrc != "" {
		path.Src.ClientID = clientSrc
	}
	if clientDst != "" {
		path.Dst.ClientID = clientDst
	}
	return nil
}

func (r *LocalRelayer) UpdateClients(ctx context.Context, _ ibc.RelayerExecReporter, pathName string) error {
	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	return relayer.UpdateClients(ctx, srcChain, dstChain, r.memo)
}

func (r *LocalRelayer) StartRelayer(ctx context.Context, _ ibc.RelayerExecReporter, pathNames ...string) error {
	chains := make(map[string]*relayer.Chain)
	paths := make([]relayer.NamedPath, len(pathNames))

	for i, pathName := range pathNames {
		path := r.config.Paths.MustGet(pathName)
		paths[i] = relayer.NamedPath{
			Name: pathName,
			Path: path,
		}

		// collect unique chain IDs
		chains[path.Src.ChainID] = nil
		chains[path.Dst.ChainID] = nil
	}

	chainIDs := make([]string, 0, len(chains))
	for chainID := range chains {
		chainIDs = append(chainIDs, chainID)
	}

	// get chain configurations
	chains, err := r.config.Chains.Gets(chainIDs...)
	if err != nil {
		return err
	}

	for _, c := range chains {
		if exists := c.ChainProvider.KeyExists(c.ChainProvider.Key()); !exists {
			return fmt.Errorf("key %s not found on src chain %s", c.ChainProvider.Key(), c.ChainID())
		}
	}

	ctx, r.cancel = context.WithCancel(ctx)

	go func() {
		r.errCh = relayer.StartRelayer(ctx, r.log, chains, paths, r.maxTxSize, r.maxMsgLength,
			r.memo, r.processor, r.initialBlockHistory, nil)
	}()

	return nil
}

func (r *LocalRelayer) StopRelayer(ctx context.Context, _ ibc.RelayerExecReporter) error {
	if r.cancel == nil {
		return nil
	}
	r.cancel()
	err := <-r.errCh

	r.cancel = nil
	r.errCh = nil
	return err
}

func (r *LocalRelayer) FlushAcknowledgements(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	pathName string,
	channelID string,
) error {
	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	channel, err := relayer.QueryChannel(ctx, srcChain, channelID)
	if err != nil {
		return err
	}

	sp := relayer.UnrelayedAcknowledgements(ctx, srcChain, dstChain, channel)

	return relayer.RelayAcknowledgements(ctx, r.log, srcChain, dstChain, sp, r.maxTxSize, r.maxMsgLength, r.memo, channel)
}

func (r *LocalRelayer) FlushPackets(
	ctx context.Context,
	_ ibc.RelayerExecReporter,
	pathName string,
	channelID string,
) error {
	srcChain, dstChain, err := r.chainsFromPath(pathName)
	if err != nil {
		return fmt.Errorf("error fetching chains: %w", err)
	}

	channel, err := relayer.QueryChannel(ctx, srcChain, channelID)
	if err != nil {
		return err
	}

	sp := relayer.UnrelayedSequences(ctx, srcChain, dstChain, channel)

	return relayer.RelayPackets(ctx, r.log, srcChain, dstChain, sp, r.maxTxSize, r.maxMsgLength, r.memo, channel)
}

func (r *LocalRelayer) GetWallet(chainID string) (ibc.Wallet, bool) {
	var zero ibc.Wallet
	chain, err := r.config.Chains.Get(chainID)
	if err != nil {
		return zero, false
	}

	keyName := chain.ChainProvider.Key()

	if !chain.ChainProvider.KeyExists(keyName) {
		return zero, false
	}

	address, err := chain.ChainProvider.ShowAddress(keyName)
	if err != nil {
		return zero, false
	}
	return ibc.Wallet{Address: address}, true
}

func (r *LocalRelayer) UseDockerNetwork() bool { return false }

func (r *LocalRelayer) Exec(ctx context.Context, _ ibc.RelayerExecReporter, cmd, env []string) ibc.RelayerExecResult {
	panic("not yet implemented")
}
