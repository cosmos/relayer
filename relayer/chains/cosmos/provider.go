package cosmos

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	cosmosmodule "github.com/cosmos/relayer/v2/relayer/chains/cosmos/module"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos/stride"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	lens "github.com/strangelove-ventures/lens/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

var (
	_ provider.ChainProvider  = &CosmosProvider{}
	_ provider.KeyProvider    = &CosmosProvider{}
	_ provider.ProviderConfig = &CosmosProviderConfig{}
)

const tendermintEncodingThreshold = "v0.37.0-alpha"

type CosmosProviderConfig struct {
	Key            string   `json:"key" yaml:"key"`
	ChainName      string   `json:"-" yaml:"-"`
	ChainID        string   `json:"chain-id" yaml:"chain-id"`
	RPCAddr        string   `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix  string   `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string   `json:"keyring-backend" yaml:"keyring-backend"`
	GasAdjustment  float64  `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices      string   `json:"gas-prices" yaml:"gas-prices"`
	MinGasAmount   uint64   `json:"min-gas-amount" yaml:"min-gas-amount"`
	Debug          bool     `json:"debug" yaml:"debug"`
	Timeout        string   `json:"timeout" yaml:"timeout"`
	OutputFormat   string   `json:"output-format" yaml:"output-format"`
	SignModeStr    string   `json:"sign-mode" yaml:"sign-mode"`
	ExtraCodecs    []string `json:"extra-codecs" yaml:"extra-codecs"`
}

func (pc CosmosProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

// NewProvider validates the CosmosProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc CosmosProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}
	cc, err := lens.NewChainClient(
		log.With(zap.String("sys", "chain_client")),
		ChainClientConfig(&pc),
		homepath,
		os.Stdin,
		os.Stdout,
	)
	if err != nil {
		return nil, err
	}
	pc.ChainName = chainName

	return &CosmosProvider{
		log:         log,
		ChainClient: *cc,
		PCfg:        pc,
	}, nil
}

// ChainClientConfig builds a ChainClientConfig struct from a CosmosProviderConfig, this is used
// to instantiate an instance of ChainClient from lens which is how we build the CosmosProvider
func ChainClientConfig(pcfg *CosmosProviderConfig) *lens.ChainClientConfig {
	modules := lens.ModuleBasics
	modules = append(modules, cosmosmodule.AppModuleBasic{})
	modules = append(modules, stride.AppModuleBasic{})
	return &lens.ChainClientConfig{
		Key:            pcfg.Key,
		ChainID:        pcfg.ChainID,
		RPCAddr:        pcfg.RPCAddr,
		AccountPrefix:  pcfg.AccountPrefix,
		KeyringBackend: pcfg.KeyringBackend,
		GasAdjustment:  pcfg.GasAdjustment,
		GasPrices:      pcfg.GasPrices,
		MinGasAmount:   pcfg.MinGasAmount,
		Debug:          pcfg.Debug,
		Timeout:        pcfg.Timeout,
		OutputFormat:   pcfg.OutputFormat,
		SignModeStr:    pcfg.SignModeStr,
		ExtraCodecs:    pcfg.ExtraCodecs,
		Modules:        modules,
	}
}

type CosmosProvider struct {
	log *zap.Logger

	PCfg CosmosProviderConfig

	lens.ChainClient
	nextAccountSeq uint64
	txMu           sync.Mutex

	// metrics to monitor the provider
	TotalFees   sdk.Coins
	totalFeesMu sync.Mutex

	metrics *processor.PrometheusMetrics

	// for tendermint < v0.37, decode tm events as base64
	tendermintLegacyEncoding bool
}

type CosmosIBCHeader struct {
	SignedHeader *tmtypes.SignedHeader
	ValidatorSet *tmtypes.ValidatorSet
}

func (h CosmosIBCHeader) Height() uint64 {
	return uint64(h.SignedHeader.Height)
}

func (h CosmosIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return &tmclient.ConsensusState{
		Timestamp:          h.SignedHeader.Time,
		Root:               commitmenttypes.NewMerkleRoot(h.SignedHeader.AppHash),
		NextValidatorsHash: h.ValidatorSet.Hash(),
	}
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

func (cc *CosmosProvider) AddKey(name string, coinType uint32) (*provider.KeyOutput, error) {
	// The lens client returns an equivalent KeyOutput type,
	// but that type is declared in the lens module,
	// and relayer's KeyProvider interface references the relayer KeyOutput.
	//
	// Translate the lens KeyOutput to a relayer KeyOutput here to satisfy the interface.

	ko, err := cc.ChainClient.AddKey(name, coinType)
	if err != nil {
		return nil, err
	}
	return &provider.KeyOutput{
		Mnemonic: ko.Mnemonic,
		Address:  ko.Address,
	}, nil
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

func (cc *CosmosProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	res, err := cc.QueryStakingParams(ctx)

	var unbondingTime time.Duration
	if err != nil {
		// Attempt ICS query
		consumerUnbondingPeriod, consumerErr := cc.queryConsumerUnbondingPeriod(ctx)
		if consumerErr != nil {
			return 0,
				fmt.Errorf("failed to query unbonding period as both standard and consumer chain: %s: %w", err.Error(), consumerErr)
		}
		unbondingTime = consumerUnbondingPeriod
	} else {
		unbondingTime = res.UnbondingTime
	}

	// We want the trusting period to be 85% of the unbonding time.
	// Go mentions that the time.Duration type can track approximately 290 years.
	// We don't want to lose precision if the duration is a very long duration
	// by converting int64 to float64.
	// Use integer math the whole time, first reducing by a factor of 100
	// and then re-growing by 85x.
	tp := unbondingTime / 100 * 85

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
	out, err := cc.Codec.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (cc *CosmosProvider) Init(ctx context.Context) error {
	status, err := cc.QueryStatus(ctx)
	if err != nil {
		// Operations can occur before the node URL is added to the config, so noop here.
		return nil
	}

	cc.setTendermintVersion(cc.log, status.NodeInfo.Version)

	return nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (cc *CosmosProvider) WaitForNBlocks(ctx context.Context, n int64) error {
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

func (cc *CosmosProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	resultBlock, err := cc.RPCClient.Block(ctx, &height)
	if err != nil {
		return time.Time{}, err
	}
	return resultBlock.Block.Time, nil
}

func (cc *CosmosProvider) SetMetrics(m *processor.PrometheusMetrics) {
	cc.metrics = m
}

func (cc *CosmosProvider) updateNextAccountSequence(seq uint64) {
	if seq > cc.nextAccountSeq {
		cc.nextAccountSeq = seq
	}
}

func (cc *CosmosProvider) setTendermintVersion(log *zap.Logger, version string) {
	cc.tendermintLegacyEncoding = cc.legacyEncodedEvents(log, version)
}

func (cc *CosmosProvider) legacyEncodedEvents(log *zap.Logger, version string) bool {
	return semver.Compare("v"+version, tendermintEncodingThreshold) < 0
}
