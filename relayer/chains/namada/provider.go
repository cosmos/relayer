package cosmos

import (
	"context"
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
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/proto"
	commitmenttypes "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"
	cwrapper "github.com/cosmos/relayer/v2/client"
	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/strangelove-ventures/cometbft-client/client"
	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &NamadaProvider{}
	_ provider.KeyProvider    = &NamadaProvider{}
	_ provider.ProviderConfig = &NamadaProviderConfig{}
)

type NamadaProviderConfig struct {
	KeyDirectory     string                     `json:"key-directory" yaml:"key-directory"`
	Key              string                     `json:"key" yaml:"key"`
	ChainName        string                     `json:"-" yaml:"-"`
	ChainID          string                     `json:"chain-id" yaml:"chain-id"`
	RPCAddr          string                     `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix    string                     `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend   string                     `json:"keyring-backend" yaml:"keyring-backend"`
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
	Broadcast        provider.BroadcastMode     `json:"broadcast-mode" yaml:"broadcast-mode"`
	MinLoopDuration  time.Duration              `json:"min-loop-duration" yaml:"min-loop-duration"`
	ExtensionOptions []provider.ExtensionOption `json:"extension-options" yaml:"extension-options"`
}

func (pc NamadaProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func (pc NamadaProviderConfig) BroadcastMode() provider.BroadcastMode {
	return pc.Broadcast
}

// NewProvider validates the CosmosProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc NamadaProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}

	pc.KeyDirectory = keysDir(homepath, pc.ChainID)

	pc.ChainName = chainName
	pc.Modules = append([]module.AppModuleBasic{}, ModuleBasics...)

	if pc.Broadcast == "" {
		pc.Broadcast = provider.BroadcastModeBatch
	}

	cp := &NamadaProvider{
		log:            log,
		PCfg:           pc,
		KeyringOptions: []keyring.Option{ethermint.EthSecp256k1Option()},
		Input:          os.Stdin,
		Output:         os.Stdout,
		walletStateMap: map[string]*WalletState{},

		// TODO: this is a bit of a hack, we should probably have a better way to inject modules
		Cdc: MakeCodec(pc.Modules, pc.ExtraCodecs, pc.AccountPrefix, pc.AccountPrefix+"valoper"),
	}

	return cp, nil
}

type NamadaIBCHeader struct {
	SignedHeader *tmtypes.SignedHeader
	ValidatorSet *tmtypes.ValidatorSet
}

func (h NamadaIBCHeader) Height() uint64 {
	return uint64(h.SignedHeader.Height)
}

func (h NamadaIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return &tmclient.ConsensusState{
		Timestamp:          h.SignedHeader.Time,
		Root:               commitmenttypes.NewMerkleRoot(h.SignedHeader.AppHash),
		NextValidatorsHash: h.ValidatorSet.Hash(),
	}
}

type NamadaProvider struct {
	log *zap.Logger

	PCfg           NamadaProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	RPCClient      cwrapper.RPCClient
	LightProvider  provtypes.Provider
	Input          io.Reader
	Output         io.Writer
	Codec          Codec
	RPCCaller      jsonrpcclient.Caller
}

type WalletState struct {
	NextAccountSequence uint64
	Mu                  sync.Mutex
}

func (cc *NamadaProvider) ProviderConfig() provider.ProviderConfig {
	return cc.PCfg
}

func (cc *NamadaProvider) ChainId() string {
	return cc.PCfg.ChainID
}

func (cc *NamadaProvider) ChainName() string {
	return cc.PCfg.ChainName
}

func (cc *NamadaProvider) Type() string {
	return "namada"
}

func (cc *NamadaProvider) Key() string {
	return cc.PCfg.Key
}

func (cc *NamadaProvider) Timeout() string {
	return cc.PCfg.Timeout
}

// CommitmentPrefix returns the commitment prefix for Cosmos
func (cc *NamadaProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	return defaultChainPrefix
}

// Address returns the chains configured address as a string
func (cc *NamadaProvider) Address() (string, error) {
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

func (cc *NamadaProvider) MustEncodeAccAddr(addr sdk.AccAddress) string {
	enc, err := cc.EncodeBech32AccAddr(addr)
	if err != nil {
		panic(err)
	}
	return enc
}

// AccountFromKeyOrAddress returns an account from either a key or an address.
// If 'keyOrAddress' is the empty string, this returns the default key's address.
func (cc *NamadaProvider) AccountFromKeyOrAddress(keyOrAddress string) (out sdk.AccAddress, err error) {
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

func (cc *NamadaProvider) TrustingPeriod(ctx context.Context, overrideUnbondingPeriod time.Duration, percentage int64) (time.Duration, error) {

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
func (cc *NamadaProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := cc.Cdc.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetPCAddr sets the rpc-addr for the chain.
// It will fail if the rpcAddr is invalid(not a url).
func (cc *NamadaProvider) SetRpcAddr(rpcAddr string) error {
	cc.PCfg.RPCAddr = rpcAddr
	return nil
}

// Init initializes the keystore, RPC client, amd light client provider.
// Once initialization is complete an attempt to query the underlying node's tendermint version is performed.
// NOTE: Init must be called after creating a new instance of CosmosProvider.
func (cc *NamadaProvider) Init(ctx context.Context) error {
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

	c, err := client.NewClient(cc.PCfg.RPCAddr, timeout)
	if err != nil {
		return err
	}

	lightprovider, err := prov.New(cc.PCfg.ChainID, cc.PCfg.RPCAddr)
	if err != nil {
		return err
	}

	rpcClient := cwrapper.NewRPCClient(c)

	cc.RPCClient = rpcClient
	cc.LightProvider = lightprovider
	cc.Keybase = keybase

	return nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (cc *NamadaProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	var initial int64
	h, err := cc.RPCClient.Status(ctx)
	if err != nil {
		return err
	}
	if h.SyncInfo.CatchingUp {
		return fmt.Errorf("chain catching up")
	}
	initial = h.SyncInfo.LatestBlockHeight
	for {
		h, err = cc.RPCClient.Status(ctx)
		if err != nil {
			return err
		}
		if h.SyncInfo.LatestBlockHeight > initial+n {
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

func (cc *NamadaProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	resultBlock, err := cc.RPCClient.Block(ctx, &height)
	if err != nil {
		return time.Time{}, err
	}
	return resultBlock.Block.Time, nil
}

func (cc *NamadaProvider) SetMetrics(m *processor.PrometheusMetrics) {
	cc.metrics = m
}

func (cc *NamadaProvider) updateNextAccountSequence(sequenceGuard *WalletState, seq uint64) {
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
