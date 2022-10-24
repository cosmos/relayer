package substrate

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/ChainSafe/chaindb"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &SubstrateProvider{}
	_ provider.KeyProvider    = &SubstrateProvider{}
	_ provider.ProviderConfig = &SubstrateProviderConfig{}
)

type SubstrateProviderConfig struct {
	Key                  string  `json:"key" yaml:"key"`
	ChainName            string  `json:"-" yaml:"-"`
	ChainID              string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr              string  `json:"rpc-addr" yaml:"rpc-addr"`
	RelayRPCAddr         string  `json:"relay-rpc-addr" yaml:"relay-rpc-addr"`
	AccountPrefix        string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend       string  `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory         string  `json:"key-directory" yaml:"key-directory"`
	GasPrices            string  `json:"gas-prices" yaml:"gas-prices"`
	GasAdjustment        float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	Debug                bool    `json:"debug" yaml:"debug"`
	Timeout              string  `json:"timeout" yaml:"timeout"`
	OutputFormat         string  `json:"output-format" yaml:"output-format"`
	SignModeStr          string  `json:"sign-mode" yaml:"sign-mode"`
	Network              uint16  `json:"network" yaml:"network"`
	ParaID               uint32  `json:"para-id" yaml:"para-id"`
	BeefyActivationBlock uint32  `json:"beefy-activation-block" yaml:"beefy-activation-block"`
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
	if err := spc.Validate(); err != nil {
		return nil, err
	}

	if len(spc.KeyDirectory) == 0 {
		spc.KeyDirectory = keysDir(homepath, spc.ChainID)
	}

	memdb, err := chaindb.NewBadgerDB(&chaindb.Config{
		InMemory: true,
		DataDir:  homepath,
	})
	if err != nil {
		return nil, err
	}

	sp := &SubstrateProvider{
		log:    log,
		Config: &spc,
		Memdb:  memdb,
	}

	err = sp.Init()
	if err != nil {
		return nil, err
	}

	return sp, nil
}

func (sp *SubstrateProvider) Init() error {
	keybase, err := keystore.New(sp.Config.ChainID, sp.Config.KeyringBackend, sp.Config.KeyDirectory, sp.Input)
	if err != nil {
		return err
	}

	sp.Keybase = keybase
	return nil
}

type SubstrateProvider struct {
	log     *zap.Logger
	Config  *SubstrateProviderConfig
	Keybase keystore.Keyring
	Memdb   *chaindb.BadgerDB
	Input   io.Reader

	RPCClient           *rpcclient.SubstrateAPI
	RelayChainRPCClient *rpcclient.SubstrateAPI
}

type SubstrateIBCHeader struct {
	height       uint64
	SignedHeader *beefyclienttypes.Header
}

// noop to implement processor.IBCHeader
func (h SubstrateIBCHeader) IBCHeaderIndicator() {
	//TODO implement me
	panic("implement me")
}

func (h SubstrateIBCHeader) Height() uint64 {
	return h.height
}

func (h SubstrateIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return h.SignedHeader.ConsensusState()
}

func (sp *SubstrateProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ChainName() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ChainId() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Type() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ProviderConfig() provider.ProviderConfig {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Key() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Address() (string, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Timeout() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Sprint(toPrint proto.Message) (string, error) {
	//TODO implement me
	panic("implement me")
}
