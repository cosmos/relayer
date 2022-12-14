package substrate

import (
	"context"
	"fmt"
	"io"
	"math"
	"path"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/substrate/finality"

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
	RelayChain           int32   `json:"relay-chain" yaml:"relay-chain"`
	FinalityGadget       string  `json:"finality-gadget" yaml:"finality-gadget"`
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

	client, err := rpcclient.NewSubstrateAPI(sp.Config.RPCAddr)
	if err != nil {
		return err
	}

	relaychainClient, err := rpcclient.NewSubstrateAPI(sp.Config.RelayRPCAddr)
	if err != nil {
		return err
	}

	switch sp.Config.FinalityGadget {
	case finality.BeefyFinalityGadget:
		sp.FinalityGadget = finality.NewBeefy(client, relaychainClient, sp.Config.ParaID,
			sp.Config.BeefyActivationBlock, sp.Memdb)
	case finality.GrandpaFinalityGadget:
		sp.FinalityGadget = finality.NewGrandpa(client, relaychainClient, sp.Config.ParaID, sp.Config.sp.Memdb)
	default:
		return fmt.Errorf("unsupported finality gadget")
	}

	sp.Keybase = keybase
	sp.RPCClient = client
	sp.RelayChainRPCClient = relaychainClient
	return nil
}

type SubstrateProvider struct {
	log     *zap.Logger
	Config  *SubstrateProviderConfig
	Keybase keystore.Keyring
	Memdb   *chaindb.BadgerDB
	Input   io.Reader

	RPCClient                     *rpcclient.SubstrateAPI
	RelayChainRPCClient           *rpcclient.SubstrateAPI
	LatestQueriedRelayChainHeight int64
	FinalityGadget                finality.FinalityGadget
}

type SubstrateIBCHeader struct {
	height       uint64
	SignedHeader *beefyclienttypes.Header
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
	return sp.Config.ChainName
}

func (sp *SubstrateProvider) ChainId() string {
	return sp.Config.ChainID
}

func (sp *SubstrateProvider) Type() string {
	return "substrate"
}

func (sp *SubstrateProvider) ProviderConfig() provider.ProviderConfig {
	return sp.Config
}

func (sp *SubstrateProvider) Key() string {
	return sp.Config.Key
}

func (sp *SubstrateProvider) Address() (string, error) {
	info, err := sp.Keybase.Key(sp.Key())
	if err != nil {
		return "", nil
	}

	return info.GetAddress(), nil
}

func (sp *SubstrateProvider) Timeout() string {
	return sp.Config.Timeout
}

func (sp *SubstrateProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	// TODO: implement a proper trusting period
	return time.Duration(math.MaxInt), nil
}

func (sp *SubstrateProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Sprint(toPrint proto.Message) (string, error) {
	//TODO implement me
	panic("implement me")
}
