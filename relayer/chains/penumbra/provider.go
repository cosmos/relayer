package penumbra

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	provtypes "github.com/cometbft/cometbft/light/provider"
	prov "github.com/cometbft/cometbft/light/provider/http"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	jsonrpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/proto"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

var (
	_ provider.ChainProvider  = &PenumbraProvider{}
	_ provider.KeyProvider    = &PenumbraProvider{}
	_ provider.ProviderConfig = &PenumbraProviderConfig{}
)

const cometEncodingThreshold = "v0.37.0-alpha"

type PenumbraProviderConfig struct {
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
	Slip44           int                        `json:"coin-type" yaml:"coin-type"`
	Broadcast        provider.BroadcastMode     `json:"broadcast-mode" yaml:"broadcast-mode"`
	MinLoopDuration  time.Duration              `json:"min-loop-duration" yaml:"min-loop-duration"`
	ExtensionOptions []provider.ExtensionOption `json:"extension-options" yaml:"extension-options"`
}

func (pc PenumbraProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func (pc PenumbraProviderConfig) BroadcastMode() provider.BroadcastMode {
	return pc.Broadcast
}

// NewProvider validates the PenumbraProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc PenumbraProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}

	pc.KeyDirectory = keysDir(homepath, pc.ChainID)

	pc.ChainName = chainName
	pc.Modules = append([]module.AppModuleBasic{}, moduleBasics...)

	if pc.Broadcast == "" {
		pc.Broadcast = provider.BroadcastModeBatch
	}

	httpClient, err := jsonrpcclient.DefaultHTTPClient(pc.RPCAddr)
	if err != nil {
		return nil, err
	}

	rc, err := jsonrpcclient.NewWithHTTPClient(pc.RPCAddr, httpClient)
	if err != nil {
		return nil, err
	}

	return &PenumbraProvider{
		log:            log,
		PCfg:           pc,
		KeyringOptions: []keyring.Option{ethermint.EthSecp256k1Option()},
		Input:          os.Stdin,
		Output:         os.Stdout,

		// TODO: this is a bit of a hack, we should probably have a better way to inject modules
		Codec:     makeCodec(pc.Modules, pc.ExtraCodecs),
		RPCCaller: rc,
	}, nil
}

type PenumbraIBCHeader struct {
	SignedHeader *tmtypes.SignedHeader
	ValidatorSet *tmtypes.ValidatorSet
}

func (h PenumbraIBCHeader) Height() uint64 {
	return uint64(h.SignedHeader.Height)
}

func (h PenumbraIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return &tmclient.ConsensusState{
		Timestamp:          h.SignedHeader.Time,
		Root:               commitmenttypes.NewMerkleRoot(h.SignedHeader.AppHash),
		NextValidatorsHash: h.ValidatorSet.Hash(),
	}
}

func (h PenumbraIBCHeader) NextValidatorsHash() []byte {
	return h.SignedHeader.NextValidatorsHash
}

type PenumbraProvider struct {
	log *zap.Logger

	PCfg           PenumbraProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	RPCClient      rpcclient.Client
	LightProvider  provtypes.Provider
	Input          io.Reader
	Output         io.Writer
	Codec          Codec
	RPCCaller      jsonrpcclient.Caller

	// for comet < v0.37, decode tm events as base64
	cometLegacyEncoding bool
}

func (cc *PenumbraProvider) ProviderConfig() provider.ProviderConfig {
	return cc.PCfg
}

func (cc *PenumbraProvider) ChainId() string {
	return cc.PCfg.ChainID
}

func (cc *PenumbraProvider) ChainName() string {
	return cc.PCfg.ChainName
}

func (cc *PenumbraProvider) Type() string {
	return "penumbra"
}

func (cc *PenumbraProvider) Key() string {
	return cc.PCfg.Key
}

func (cc *PenumbraProvider) Timeout() string {
	return cc.PCfg.Timeout
}

func (cc *PenumbraProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	return commitmenttypes.NewMerklePrefix([]byte("PenumbraAppHash"))
}

// Address returns the chains configured address as a string
func (cc *PenumbraProvider) Address() (string, error) {
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

func (cc *PenumbraProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	// TODO
	return time.Hour * 2, nil
	/*
		res, err := cc.QueryStakingParams(ctx)
		if err != nil {
			return 0, err
		}

		// We want the trusting period to be 85% of the unbonding time.
		// Go mentions that the time.Duration type can track approximately 290 years.
		// We don't want to lose precision if the duration is a very long duration
		// by converting int64 to float64.
		// Use integer math the whole time, first reducing by a factor of 100
		// and then re-growing by 85x.
		tp := res.UnbondingTime / 100 * 85

		// And we only want the trusting period to be whole hours.
		return tp.Truncate(time.Hour), nil
	*/
}

// Sprint returns the json representation of the specified proto message.
func (cc *PenumbraProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := cc.Codec.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetPCAddr sets the rpc-addr for the chain.
// It will fail if the rpcAddr is invalid(not a url).
func (cc *PenumbraProvider) SetRpcAddr(rpcAddr string) error {
	cc.PCfg.RPCAddr = rpcAddr
	return nil
}

// Init initializes the keystore, RPC client, amd light client provider.
// Once initialization is complete an attempt to query the underlying node's tendermint version is performed.
// NOTE: Init must be called after creating a new instance of CosmosProvider.
func (cc *PenumbraProvider) Init(ctx context.Context) error {
	keybase, err := keyring.New(cc.PCfg.ChainID, cc.PCfg.KeyringBackend, cc.PCfg.KeyDirectory, cc.Input, cc.Codec.Marshaler, cc.KeyringOptions...)
	if err != nil {
		return err
	}
	// TODO: figure out how to deal with input or maybe just make all keyring backends test?

	timeout, err := time.ParseDuration(cc.PCfg.Timeout)
	if err != nil {
		return err
	}

	rpcClient, err := newRPCClient(cc.PCfg.RPCAddr, timeout)
	if err != nil {
		return err
	}

	lightprovider, err := prov.New(cc.PCfg.ChainID, cc.PCfg.RPCAddr)
	if err != nil {
		return err
	}

	cc.RPCClient = rpcClient
	cc.LightProvider = lightprovider
	cc.Keybase = keybase

	status, err := cc.QueryStatus(ctx)
	if err != nil {
		// Operations can occur before the node URL is added to the config, so noop here.
		return nil
	}

	cc.setCometVersion(cc.log, status.NodeInfo.Version)

	return nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (cc *PenumbraProvider) WaitForNBlocks(ctx context.Context, n int64) error {
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

func (cc *PenumbraProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	resultBlock, err := cc.RPCClient.Block(ctx, &height)
	if err != nil {
		return time.Time{}, err
	}
	return resultBlock.Block.Time, nil
}

func toPenumbraPacket(pi provider.PacketInfo) chantypes.Packet {
	return chantypes.Packet{
		Sequence:           pi.Sequence,
		SourcePort:         pi.SourcePort,
		SourceChannel:      pi.SourceChannel,
		DestinationPort:    pi.DestPort,
		DestinationChannel: pi.DestChannel,
		Data:               pi.Data,
		TimeoutHeight:      pi.TimeoutHeight,
		TimeoutTimestamp:   pi.TimeoutTimestamp,
	}
}

func (cc *PenumbraProvider) setCometVersion(log *zap.Logger, version string) {
	cc.cometLegacyEncoding = cc.legacyEncodedEvents(log, version)
}

func (cc *PenumbraProvider) legacyEncodedEvents(log *zap.Logger, version string) bool {
	return semver.Compare("v"+version, cometEncodingThreshold) < 0
}

// keysDir returns a string representing the path on the local filesystem where the keystore will be initialized.
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// newRPCClient initializes a new tendermint RPC client connected to the specified address.
func newRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
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
