package penumbra

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cosmos/cosmos-sdk/types/module"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v5/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v5/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
	lens "github.com/strangelove-ventures/lens/client"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &PenumbraProvider{}
	_ provider.KeyProvider    = &PenumbraProvider{}
	_ provider.ProviderConfig = &PenumbraProviderConfig{}
)

type PenumbraProviderConfig struct {
	Key            string  `json:"key" yaml:"key"`
	ChainName      string  `json:"-" yaml:"-"`
	ChainID        string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr        string  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix  string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string  `json:"keyring-backend" yaml:"keyring-backend"`
	GasAdjustment  float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices      string  `json:"gas-prices" yaml:"gas-prices"`
	Debug          bool    `json:"debug" yaml:"debug"`
	Timeout        string  `json:"timeout" yaml:"timeout"`
	OutputFormat   string  `json:"output-format" yaml:"output-format"`
	SignModeStr    string  `json:"sign-mode" yaml:"sign-mode"`
}

func (pc PenumbraProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
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

// NewProvider validates the PenumbraProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc PenumbraProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
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

	httpClient, err := jsonrpcclient.DefaultHTTPClient(pc.RPCAddr)
	if err != nil {
		return nil, err
	}
	//	httpClient.Timeout = time.Duration(pc.Timeout)
	rc, err := jsonrpcclient.NewWithHTTPClient(pc.RPCAddr, httpClient)
	if err != nil {
		return nil, err
	}

	return &PenumbraProvider{
		log: log,

		ChainClient: *cc,
		PCfg:        pc,
		RPCCaller:   rc,
	}, nil
}

// ChainClientConfig builds a ChainClientConfig struct from a PenumbraProviderConfig, this is used
// to instantiate an instance of ChainClient from lens which is how we build the PenumbraProvider
func ChainClientConfig(pcfg *PenumbraProviderConfig) *lens.ChainClientConfig {
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

type PenumbraProvider struct {
	log *zap.Logger

	lens.ChainClient
	PCfg      PenumbraProviderConfig
	RPCCaller jsonrpcclient.Caller
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

func (cc *PenumbraProvider) AddKey(name string, coinType uint32) (*provider.KeyOutput, error) {
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
	fmt.Println("getting penumbra trusting period")
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
