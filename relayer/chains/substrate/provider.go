package substrate

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
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
	RelayRPCAddr   string  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix  string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string  `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory   string  `json:"key-directory" yaml:"key-directory"`
	GasAdjustment  float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices      string  `json:"gas-prices" yaml:"gas-prices"`
	Debug          bool    `json:"debug" yaml:"debug"`
	Timeout        string  `json:"timeout" yaml:"timeout"`
	OutputFormat   string  `json:"output-format" yaml:"output-format"`
	SignModeStr    string  `json:"sign-mode" yaml:"sign-mode"`
	Network        uint8   `json:"network" yaml:"network"`
}

func (spc SubstrateProviderConfig) Validate() error {
	if _, err := time.ParseDuration(spc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// NewProvider validates the SubstrateProviderConfig, instantiates a ChainClient and then instantiates a SubstrateProvider
func (spc SubstrateProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	spc.KeyDirectory = keysDir(homepath, spc.ChainID)
	sp := &SubstrateProvider{
		Config: &spc,
	}

	err := sp.Init()
	if err != nil {
		return nil, err
	}

	return sp, nil
}

type SubstrateProvider struct {
	log *zap.Logger

	RPCClient      *rpcclient.SubstrateAPI
	RPCRelayClient *rpcclient.SubstrateAPI
	Config         *SubstrateProviderConfig
	Keybase        keystore.Keyring
	Input          io.Reader

	PCfg SubstrateProviderConfig
}

type SubstrateIBCHeader struct {
	SignedHeader *tmtypes.SignedHeader
	ValidatorSet *tmtypes.ValidatorSet
}

func (sp *SubstrateProvider) Init() error {
	keybase, err := keystore.New(sp.Config.ChainID, sp.Config.KeyringBackend, sp.Config.KeyDirectory, sp.Input)
	if err != nil {
		return err
	}

	sp.Keybase = keybase
	return nil
}

// noop to implement processor.IBCHeader
func (h SubstrateIBCHeader) IBCHeaderIndicator() {}

func (h SubstrateIBCHeader) Height() uint64 {
	return uint64(h.SignedHeader.Height)
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
