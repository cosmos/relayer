package cosmos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	provtypes "github.com/cometbft/cometbft/light/provider"
	prov "github.com/cometbft/cometbft/light/provider/http"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/proto"
	commitmenttypes "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"
	cwrapper "github.com/cosmos/relayer/v2/client"
	relayerclient "github.com/cosmos/relayer/v2/client"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	cometbftclient "github.com/strangelove-ventures/cometbft-client/client"
	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &CosmosProvider{}
	_ provider.KeyProvider    = &CosmosProvider{}
	_ provider.ProviderConfig = &CosmosProviderConfig{}
)

type CosmosProviderConfig struct {
	KeyDirectory     string                     `json:"key-directory" yaml:"key-directory"`
	Key              string                     `json:"key" yaml:"key"`
	ChainName        string                     `json:"-" yaml:"-"`
	ChainID          string                     `json:"chain-id" yaml:"chain-id"`
	RPCAddr          string                     `json:"rpc-addr" yaml:"rpc-addr"`
	BackupRPCAddrs   []string                   `json:"backup-rpc-addrs" yaml:"backup-rpc-addrs"`
	AccountPrefix    string                     `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend   string                     `json:"keyring-backend" yaml:"keyring-backend"`
	DynamicGasPrice  bool                       `json:"dynamic-gas-price" yaml:"dynamic-gas-price"`
	GasAdjustment    float64                    `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices        string                     `json:"gas-prices" yaml:"gas-prices"`
	MinGasAmount     uint64                     `json:"min-gas-amount" yaml:"min-gas-amount"`
	MaxGasAmount     uint64                     `json:"max-gas-amount" yaml:"max-gas-amount"`
	Debug            bool                       `json:"debug" yaml:"debug"`
	Timeout          string                     `json:"timeout" yaml:"timeout"`
	BlockTimeout     string                     `json:"block-timeout" yaml:"block-timeout"`
	OutputFormat     string                     `json:"output-format" yaml:"output-format"`
	SignModeStr      string                     `json:"sign-mode" yaml:"sign-mode"`
	ExtraCodecs      []string                   `json:"extra-codecs" yaml:"extra-codecs"`
	Modules          []module.AppModuleBasic    `json:"-" yaml:"-"`
	Slip44           *int                       `json:"coin-type" yaml:"coin-type"`
	SigningAlgorithm string                     `json:"signing-algorithm" yaml:"signing-algorithm"`
	Broadcast        provider.BroadcastMode     `json:"broadcast-mode" yaml:"broadcast-mode"`
	MinLoopDuration  time.Duration              `json:"min-loop-duration" yaml:"min-loop-duration"`
	ExtensionOptions []provider.ExtensionOption `json:"extension-options" yaml:"extension-options"`

	// If FeeGrantConfiguration is set, TXs submitted by the ChainClient will be signed by the FeeGrantees in a round-robin fashion by default.
	FeeGrants *FeeGrantConfiguration `json:"feegrants" yaml:"feegrants"`
}

// By default, TXs will be signed by the feegrantees 'ManagedGrantees' keys in a round robin fashion.
// Clients can use other signing keys by invoking 'tx.SendMsgsWith' and specifying the signing key.
type FeeGrantConfiguration struct {
	GranteesWanted int `json:"num_grantees" yaml:"num_grantees"`
	// Normally this is the default ChainClient key
	GranterKeyOrAddr string `json:"granter" yaml:"granter"`
	// Whether we control the granter private key (if not, someone else must authorize our feegrants)
	IsExternalGranter bool `json:"external_granter" yaml:"external_granter"`
	// List of keys (by name) that this FeeGranter manages
	ManagedGrantees []string `json:"grantees" yaml:"grantees"`
	// Last checked on chain (0 means grants never checked and may not exist)
	BlockHeightVerified int64 `json:"block_last_verified" yaml:"block_last_verified"`
	// Index of the last ManagedGrantee used as a TX signer
	GranteeLastSignerIndex int
}

func (pc CosmosProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func (pc CosmosProviderConfig) BroadcastMode() provider.BroadcastMode {
	return pc.Broadcast
}

// NewProvider validates the CosmosProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc CosmosProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}

	pc.KeyDirectory = keysDir(homepath, pc.ChainID)

	pc.ChainName = chainName
	pc.Modules = append([]module.AppModuleBasic{}, ModuleBasics...)

	if pc.Broadcast == "" {
		pc.Broadcast = provider.BroadcastModeBatch
	}

	cp := &CosmosProvider{
		log:            log,
		PCfg:           pc,
		KeyringOptions: []keyring.Option{},
		Input:          os.Stdin,
		Output:         os.Stdout,
		walletStateMap: map[string]*WalletState{},

		// TODO: this is a bit of a hack, we should probably have a better way to inject modules
		Cdc: MakeCodec(pc.Modules, pc.ExtraCodecs, pc.AccountPrefix, pc.AccountPrefix+"valoper"),
	}

	return cp, nil
}

type CosmosProvider struct {
	log *zap.Logger

	PCfg            CosmosProviderConfig
	Keybase         keyring.Keyring
	KeyringOptions  []keyring.Option
	ConsensusClient relayerclient.ConsensusRelayerI

	LightProvider provtypes.Provider
	Input         io.Reader
	Output        io.Writer
	Cdc           Codec
	// TODO: GRPC Client type?

	//nextAccountSeq uint64
	feegrantMu sync.Mutex

	// the map key is the TX signer, which can either be 'default' (provider key) or a feegrantee
	// the purpose of the map is to lock on the signer from TX creation through submission,
	// thus making TX sequencing errors less likely.
	walletStateMap map[string]*WalletState

	// metrics to monitor the provider
	TotalFees   sdk.Coins
	totalFeesMu sync.Mutex

	metrics *processor.PrometheusMetrics

	// for comet < v0.37, decode tm events as base64
	cometLegacyEncoding bool

	// for comet < v0.38, use legacy RPC client for ResultsBlockResults
	cometLegacyBlockResults bool
}

type WalletState struct {
	NextAccountSequence uint64
	Mu                  sync.Mutex
}

func (cc *CosmosProvider) ProviderConfig() provider.ProviderConfig {
	return cc.PCfg
}

func (cc *CosmosProvider) ChainId() string {
	return cc.PCfg.ChainID
}

func (cc *CosmosProvider) ChainName() string {
	return cc.PCfg.ChainName
}

func (cc *CosmosProvider) Type() string {
	return "cosmos"
}

func (cc *CosmosProvider) Key() string {
	return cc.PCfg.Key
}

func (cc *CosmosProvider) Timeout() string {
	return cc.PCfg.Timeout
}

// CommitmentPrefix returns the commitment prefix for Cosmos
func (cc *CosmosProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	return defaultChainPrefix
}

// Address returns the chains configured address as a string
func (cc *CosmosProvider) Address() (string, error) {
	info, err := cc.Keybase.Key(cc.PCfg.Key)
	if err != nil {
		return "", err
	}

	acc, err := info.GetAddress()
	if err != nil {
		return "", err
	}

	out, err := cc.EncodeBech32AccAddr(acc)
	if err != nil {
		return "", err
	}

	return out, err
}

func (cc *CosmosProvider) MustEncodeAccAddr(addr sdk.AccAddress) string {
	enc, err := cc.EncodeBech32AccAddr(addr)
	if err != nil {
		panic(err)
	}
	return enc
}

// AccountFromKeyOrAddress returns an account from either a key or an address.
// If 'keyOrAddress' is the empty string, this returns the default key's address.
func (cc *CosmosProvider) AccountFromKeyOrAddress(keyOrAddress string) (out sdk.AccAddress, err error) {
	switch {
	case keyOrAddress == "":
		out, err = cc.GetKeyAddress(cc.PCfg.Key)
	case cc.KeyExists(keyOrAddress):
		out, err = cc.GetKeyAddress(keyOrAddress)
	default:
		out, err = sdk.GetFromBech32(keyOrAddress, cc.PCfg.AccountPrefix)
	}
	return
}

func (cc *CosmosProvider) TrustingPeriod(ctx context.Context, overrideUnbondingPeriod time.Duration, percentage int64) (time.Duration, error) {

	unbondingTime := overrideUnbondingPeriod
	var err error
	if unbondingTime == 0 {
		unbondingTime, err = cc.QueryUnbondingPeriod(ctx)
		if err != nil {
			return 0, err
		}
	}

	// We want the trusting period to be `percentage` of the unbonding time.
	// Go mentions that the time.Duration type can track approximately 290 years.
	// We don't want to lose precision if the duration is a very long duration
	// by converting int64 to float64.
	// Use integer math the whole time, first reducing by a factor of 100
	// and then re-growing by the `percentage` param.
	tp := time.Duration(int64(unbondingTime) / 100 * percentage)

	// And we only want the trusting period to be whole hours.
	// But avoid rounding if the time is less than 1 hour
	//  (otherwise the trusting period will go to 0)
	if tp > time.Hour {
		tp = tp.Truncate(time.Hour)
	}
	return tp, nil
}

// Sprint returns the json representation of the specified proto message.
func (cc *CosmosProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := cc.Cdc.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetPCAddr sets the rpc-addr for the chain.
// It will fail if the rpcAddr is invalid(not a url).
func (cc *CosmosProvider) SetRpcAddr(rpcAddr string) error {
	cc.PCfg.RPCAddr = rpcAddr
	return nil
}

// SetBackupRpcAddrs sets the backup rpc-addr for the chain.
// These addrs are used in the event that the primary rpc-addr is down.
func (cc *CosmosProvider) SetBackupRpcAddrs(rpcAddrs []string) error {
	cc.PCfg.BackupRPCAddrs = rpcAddrs
	return nil
}

// Init initializes the keystore, RPC client, amd light client provider.
// Once initialization is complete an attempt to query the underlying node's tendermint version is performed.
// NOTE: Init must be called after creating a new instance of CosmosProvider.
func (cc *CosmosProvider) Init(ctx context.Context) error {
	keybase, err := keyring.New(
		cc.PCfg.ChainID,
		cc.PCfg.KeyringBackend,
		cc.PCfg.KeyDirectory,
		cc.Input,
		cc.Cdc.Marshaler,
		cc.KeyringOptions...,
	)
	if err != nil {
		return err
	}
	// TODO: figure out how to deal with input or maybe just make all keyring backends test?

	timeout, err := time.ParseDuration(cc.PCfg.Timeout)
	if err != nil {
		return err
	}

	// set the RPC client
	err = cc.setConsensusClient(true, cc.PCfg.RPCAddr, timeout)
	if err != nil {
		return err
	}

	// set the light client provider
	err = cc.setLightProvider(cc.PCfg.RPCAddr)
	if err != nil {
		return err
	}

	// set keybase
	cc.Keybase = keybase

	// go routine to monitor RPC liveliness
	go cc.startLivelinessChecks(ctx, timeout)

	return nil
}

// startLivelinessChecks frequently checks the liveliness of an RPC client and resorts to backup rpcs if the active rpc is down.
// This is a blocking function; call this within a go routine.
func (cc *CosmosProvider) startLivelinessChecks(ctx context.Context, timeout time.Duration) {
	// list of rpcs & index to keep track of active rpc
	rpcs := append([]string{cc.PCfg.RPCAddr}, cc.PCfg.BackupRPCAddrs...)

	// exit routine if there is only one rpc client
	if len(rpcs) <= 1 {
		if cc.log != nil {
			cc.log.Debug("No backup RPCs defined", zap.String("chain", cc.ChainName()))
		}
		return
	}

	// log the number of available rpcs
	cc.log.Debug("Available RPC clients", zap.String("chain", cc.ChainName()), zap.Int("count", len(rpcs)))

	// tick every 10s to ensure rpc liveliness
	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := cc.ConsensusClient.GetStatus(ctx)
			if err != nil {
				cc.log.Error("RPC client disconnected", zap.String("chain", cc.ChainName()), zap.Error(err))

				index := -1
				attempts := 0

				// attempt to connect to the backup RPC client
				for {

					attempts++
					if attempts > len(rpcs) {
						cc.log.Error("All configured RPC endpoints return non-200 response", zap.String("chain", cc.ChainName()), zap.Error(err))
						break
					}

					// get next rpc
					index = (index + 1) % len(rpcs)
					rpcAddr := rpcs[index]

					cc.log.Info("Attempting to connect to new RPC", zap.String("chain", cc.ChainName()), zap.String("rpc", rpcAddr))

					// attempt to setup rpc client
					if err = cc.setConsensusClient(false, rpcAddr, timeout); err != nil {
						cc.log.Error("Failed to connect to RPC client", zap.String("chain", cc.ChainName()), zap.String("rpc", rpcAddr), zap.Error(err))
						continue
					}

					// attempt to setup light client
					if err = cc.setLightProvider(rpcAddr); err != nil {
						cc.log.Error("Failed to connect to light client provider", zap.String("chain", cc.ChainName()), zap.String("rpc", rpcAddr), zap.Error(err))
						continue
					}

					cc.log.Info("Successfully connected to new RPC", zap.String("chain", cc.ChainName()), zap.String("rpc", rpcAddr))

					// rpc found, escape
					break
				}
			}
		}
	}
}

// setConsensusClient sets the RPC client for the chain.
func (cc *CosmosProvider) setConsensusClient(onStartup bool, rpcAddr string, timeout time.Duration) error {
	c, err := cometbftclient.NewClient(rpcAddr, timeout)
	if err != nil {
		// TODO: try gordian here?
		return err
	}

	rpcClient := cwrapper.NewRPCClient(c)

	cc.ConsensusClient = rpcClient

	// Only check status if not on startup, to ensure the relayer will not block on startup.
	// All subsequent calls will perform the status check to ensure RPC endpoints are rotated
	// as necessary.
	if !onStartup {
		if _, err = cc.ConsensusClient.GetStatus(context.Background()); err != nil {
			return err
		}
	}

	return nil
}

// setLightProvider sets the light client provider for the chain.
func (cc *CosmosProvider) setLightProvider(rpcAddr string) error {
	lightprovider, err := prov.New(cc.PCfg.ChainID, rpcAddr)
	if err != nil {
		return err
	}

	cc.LightProvider = lightprovider
	return nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (cc *CosmosProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	var initial uint64
	h, err := cc.ConsensusClient.GetStatus(ctx)
	if err != nil {
		return err
	}
	if h.CatchingUp {
		return errors.New("chain catching up")
	}
	initial = h.LatestBlockHeight
	for {
		h, err = cc.ConsensusClient.GetStatus(ctx)
		if err != nil {
			return err
		}
		if h.LatestBlockHeight > initial+uint64(n) {
			return nil
		}
		select {
		case <-time.After(10 * time.Millisecond):
			// Nothing to do.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (cc *CosmosProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	blockTime, err := cc.ConsensusClient.GetBlockTime(ctx, uint64(height))
	if err != nil {
		return time.Time{}, err
	}
	return blockTime, nil
}

func (cc *CosmosProvider) SetMetrics(m *processor.PrometheusMetrics) {
	cc.metrics = m
}

func (cc *CosmosProvider) updateNextAccountSequence(sequenceGuard *WalletState, seq uint64) {
	if seq > sequenceGuard.NextAccountSequence {
		sequenceGuard.NextAccountSequence = seq
	}
}

// keysDir returns a string representing the path on the local filesystem where the keystore will be initialized.
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// NewRPCClient initializes a new tendermint RPC client connected to the specified address.
func NewRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}
	httpClient.Timeout = timeout
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}
	return rpcClient, nil
}
