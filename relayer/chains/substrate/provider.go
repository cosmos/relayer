package substrate

import (
	"context"
	"fmt"
	"io"
	"time"

	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	"github.com/cosmos/cosmos-sdk/types/module"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
	lens "github.com/strangelove-ventures/lens/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &SubstrateProvider{}
	_ provider.KeyProvider    = &SubstrateProvider{}
	_ provider.ProviderConfig = &SubstrateProviderConfig{}
)

type SubstrateProviderConfig struct {
	Key            string  `json:"key" yaml:"key"`
	ChainName      string  `json:"-" yaml:"-"`
	ChainID        string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr        string  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix  string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string  `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory   string  `json:"key-directory" yaml:"key-directory"`
	GasAdjustment  float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices      string  `json:"gas-prices" yaml:"gas-prices"`
	Debug          bool    `json:"debug" yaml:"debug"`
	Timeout        string  `json:"timeout" yaml:"timeout"`
	OutputFormat   string  `json:"output-format" yaml:"output-format"`
	SignModeStr    string  `json:"sign-mode" yaml:"sign-mode"`
}

func (pc SubstrateProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

// NewProvider validates the SubstrateProviderConfig, instantiates a ChainClient and then instantiates a SubstrateProvider
func (pc SubstrateProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	return &SubstrateProvider{}, nil
}

// ChainClientConfig builds a ChainClientConfig struct from a SubstrateProviderConfig, this is used
// to instantiate an instance of ChainClient from lens which is how we build the SubstrateProvider
func ChainClientConfig(pcfg *SubstrateProviderConfig) *lens.ChainClientConfig {
	return &lens.ChainClientConfig{
		Key:            pcfg.Key,
		ChainID:        pcfg.ChainID,
		RPCAddr:        pcfg.RPCAddr,
		AccountPrefix:  pcfg.AccountPrefix,
		KeyringBackend: pcfg.KeyringBackend,
		GasAdjustment:  pcfg.GasAdjustment,
		GasPrices:      pcfg.GasPrices,
		Debug:          pcfg.Debug,
		Timeout:        pcfg.Timeout,
		OutputFormat:   pcfg.OutputFormat,
		SignModeStr:    pcfg.SignModeStr,
		Modules:        append([]module.AppModuleBasic{}, lens.ModuleBasics...),
	}
}

type SubstrateProvider struct {
	log *zap.Logger

	RPCClient *rpcclient.SubstrateAPI
	Config    *SubstrateProviderConfig
	Keybase   keystore.Keyring
	Input     io.Reader

	PCfg SubstrateProviderConfig
}

type SubstrateIBCHeader struct {
	SignedHeader *tmtypes.SignedHeader
	ValidatorSet *tmtypes.ValidatorSet
}

// noop to implement processor.IBCHeader
func (h SubstrateIBCHeader) IBCHeaderIndicator() {}

func (h SubstrateIBCHeader) Height() uint64 {
	return uint64(h.SignedHeader.Height)
}

func (h SubstrateIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return nil
}

func (sc *SubstrateProvider) ProviderConfig() provider.ProviderConfig {
	return sc.PCfg
}

func (sc *SubstrateProvider) ChainId() string {
	return sc.PCfg.ChainID
}

func (sc *SubstrateProvider) ChainName() string {
	return sc.PCfg.ChainName
}

func (sc *SubstrateProvider) Type() string {
	return "substrate"
}

func (sc *SubstrateProvider) Key() string {
	return sc.PCfg.Key
}

func (sc *SubstrateProvider) Timeout() string {
	return sc.PCfg.Timeout
}

// Address returns the chains configured address as a string
func (sc *SubstrateProvider) Address() (string, error) { return "", nil }

func (sc *SubstrateProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

// Sprint returns the json representation of the specified proto message.
func (sc *SubstrateProvider) Sprint(toPrint proto.Message) (string, error) { return "", nil }

// WaitForNBlocks blocks until the next block on a given chain
func (sc *SubstrateProvider) WaitForNBlocks(ctx context.Context, n int64) error { return nil }

func (sc *SubstrateProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	return time.Time{}, nil
}
