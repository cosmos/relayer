package archway

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/CosmWasm/wasmd/app"
	provtypes "github.com/cometbft/cometbft/light/provider"
	comettypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/cosmos-sdk/client"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	prov "github.com/cometbft/cometbft/light/provider/http"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/proto"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"

	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &ArchwayProvider{}
	_ provider.KeyProvider    = &ArchwayProvider{}
	_ provider.ProviderConfig = &ArchwayProviderConfig{}
)

type ArchwayProviderConfig struct {
	KeyDirectory         string                  `json:"key-directory" yaml:"key-directory"`
	Key                  string                  `json:"key" yaml:"key"`
	ChainName            string                  `json:"-" yaml:"-"`
	ChainID              string                  `json:"chain-id" yaml:"chain-id"`
	RPCAddr              string                  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix        string                  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend       string                  `json:"keyring-backend" yaml:"keyring-backend"`
	GasAdjustment        float64                 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices            string                  `json:"gas-prices" yaml:"gas-prices"`
	MinGasAmount         uint64                  `json:"min-gas-amount" yaml:"min-gas-amount"`
	Debug                bool                    `json:"debug" yaml:"debug"`
	Timeout              string                  `json:"timeout" yaml:"timeout"`
	BlockTimeout         string                  `json:"block-timeout" yaml:"block-timeout"`
	OutputFormat         string                  `json:"output-format" yaml:"output-format"`
	SignModeStr          string                  `json:"sign-mode" yaml:"sign-mode"`
	ExtraCodecs          []string                `json:"extra-codecs" yaml:"extra-codecs"`
	Modules              []module.AppModuleBasic `json:"-" yaml:"-"`
	Slip44               int                     `json:"coin-type" yaml:"coin-type"`
	Broadcast            provider.BroadcastMode  `json:"broadcast-mode" yaml:"broadcast-mode"`
	IbcHandlerAddress    string                  `json:"ibc-handler-address" yaml:"ibc-handler-address"`
	FirstRetryBlockAfter uint64                  `json:"first-retry-block-after" yaml:"first-retry-block-after"`
}

type ArchwayIBCHeader struct {
	SignedHeader *itm.SignedHeader
	ValidatorSet *itm.ValidatorSet
}

func NewArchwayIBCHeader(header *itm.SignedHeader, validators *itm.ValidatorSet) ArchwayIBCHeader {
	return ArchwayIBCHeader{
		SignedHeader: header,
		ValidatorSet: validators,
	}
}

func NewArchwayIBCHeaderFromLightBlock(lightBlock *comettypes.LightBlock) ArchwayIBCHeader {
	vSets := make([]*itm.Validator, 0)
	for _, v := range lightBlock.ValidatorSet.Validators {
		_v := &itm.Validator{
			Address: v.Address,
			PubKey: &itm.PublicKey{
				Sum: itm.GetPubKeyFromTx(v.PubKey.Type(), v.PubKey.Bytes()),
			},
			VotingPower:      v.VotingPower,
			ProposerPriority: v.ProposerPriority,
		}

		vSets = append(vSets, _v)
	}

	signatures := make([]*itm.CommitSig, 0)
	for _, d := range lightBlock.Commit.Signatures {

		_d := &itm.CommitSig{
			BlockIdFlag:      itm.BlockIDFlag(d.BlockIDFlag),
			ValidatorAddress: d.ValidatorAddress,
			Timestamp: &itm.Timestamp{
				Seconds: int64(d.Timestamp.Unix()),
				Nanos:   int32(d.Timestamp.Nanosecond()),
			},
			Signature: d.Signature,
		}
		signatures = append(signatures, _d)
	}

	return ArchwayIBCHeader{
		SignedHeader: &itm.SignedHeader{
			Header: &itm.LightHeader{
				Version: &itm.Consensus{
					Block: lightBlock.Version.Block,
					App:   lightBlock.Version.App,
				},
				ChainId: lightBlock.ChainID,

				Height: lightBlock.Height,
				Time: &itm.Timestamp{
					Seconds: int64(lightBlock.Time.Unix()),
					Nanos:   int32(lightBlock.Time.Nanosecond()), // this is the offset after the nanosecond
				},
				LastBlockId: &itm.BlockID{
					Hash: lightBlock.LastBlockID.Hash,
					PartSetHeader: &itm.PartSetHeader{
						Total: lightBlock.LastBlockID.PartSetHeader.Total,
						Hash:  lightBlock.LastBlockID.PartSetHeader.Hash,
					},
				},
				LastCommitHash:     lightBlock.LastCommitHash,
				DataHash:           lightBlock.DataHash,
				ValidatorsHash:     lightBlock.ValidatorsHash,
				NextValidatorsHash: lightBlock.NextValidatorsHash,
				ConsensusHash:      lightBlock.ConsensusHash,
				AppHash:            lightBlock.AppHash,
				LastResultsHash:    lightBlock.LastResultsHash,
				EvidenceHash:       lightBlock.EvidenceHash,
				ProposerAddress:    lightBlock.ProposerAddress,
			},
			Commit: &itm.Commit{
				Height: lightBlock.Commit.Height,
				Round:  lightBlock.Commit.Round,
				BlockId: &itm.BlockID{
					Hash: lightBlock.Commit.BlockID.Hash,
					PartSetHeader: &itm.PartSetHeader{
						Total: lightBlock.Commit.BlockID.PartSetHeader.Total,
						Hash:  lightBlock.Commit.BlockID.PartSetHeader.Hash,
					},
				},
				Signatures: signatures,
			},
		},
		ValidatorSet: &itm.ValidatorSet{
			Validators: vSets,
		},
	}
}

func (h ArchwayIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return &itm.ConsensusState{
		Timestamp:          h.SignedHeader.Header.Time,
		Root:               &itm.MerkleRoot{Hash: h.SignedHeader.Header.AppHash},
		NextValidatorsHash: h.SignedHeader.Header.NextValidatorsHash,
	}
}

func (a ArchwayIBCHeader) Height() uint64 {
	return uint64(a.SignedHeader.Header.Height)
}

func (a ArchwayIBCHeader) IsCompleteBlock() bool {
	return true
}

func (a ArchwayIBCHeader) NextValidatorsHash() []byte {
	return a.SignedHeader.Header.NextValidatorsHash
}

func (a ArchwayIBCHeader) ShouldUpdateWithZeroMessage() bool {
	return false
}

func (pp *ArchwayProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pp.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}

	if pp.IbcHandlerAddress == "" {
		return fmt.Errorf("Ibc handler contract cannot be empty")
	}
	return nil
}

func (pp *ArchwayProviderConfig) getRPCAddr() string {
	return pp.RPCAddr
}

func (pp *ArchwayProviderConfig) BroadcastMode() provider.BroadcastMode {
	return pp.Broadcast
}

func (pc *ArchwayProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}

	pc.KeyDirectory = keysDir(homepath, pc.ChainID)

	pc.ChainName = chainName
	pc.Modules = append([]module.AppModuleBasic{}, ModuleBasics...)

	if pc.Broadcast == "" {
		pc.Broadcast = provider.BroadcastModeBatch
	}

	cp := &ArchwayProvider{
		log:            log,
		PCfg:           pc,
		KeyringOptions: []keyring.Option{ethermint.EthSecp256k1Option()},
		Input:          os.Stdin,
		Output:         os.Stdout,

		// TODO: this is a bit of a hack, we should probably have a better way to inject modules
		Cdc: MakeCodec(pc.Modules, pc.ExtraCodecs),
	}

	return cp, nil
}

type ArchwayProvider struct {
	log *zap.Logger

	PCfg           *ArchwayProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	RPCClient      rpcclient.Client
	QueryClient    wasmtypes.QueryClient
	LightProvider  provtypes.Provider
	Cdc            Codec
	Input          io.Reader
	Output         io.Writer
	ClientCtx      client.Context

	nextAccountSeq uint64
	txMu           sync.Mutex

	metrics *processor.PrometheusMetrics

	// for comet < v0.37, decode tm events as base64
	cometLegacyEncoding bool
}

func (ap *ArchwayProvider) ProviderConfig() provider.ProviderConfig {
	return ap.PCfg
}

func (ap *ArchwayProvider) ChainId() string {
	return ap.PCfg.ChainID
}

func (ap *ArchwayProvider) ChainName() string {
	return ap.PCfg.ChainName
}

func (ap *ArchwayProvider) Type() string {
	return "archway"
}

func (ap *ArchwayProvider) Key() string {
	return ap.PCfg.Key
}

func (ap *ArchwayProvider) Timeout() string {
	return ap.PCfg.Timeout
}

// CommitmentPrefix returns the commitment prefix for Cosmos
func (ap *ArchwayProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	ctx := context.Background()
	b, _ := ap.GetCommitmentPrefixFromContract(ctx)
	return commitmenttypes.NewMerklePrefix(b)
}

func (ap *ArchwayProvider) Init(ctx context.Context) error {
	keybase, err := keyring.New(ap.PCfg.ChainID, ap.PCfg.KeyringBackend, ap.PCfg.KeyDirectory, ap.Input, ap.Cdc.Marshaler, ap.KeyringOptions...)
	if err != nil {
		return err
	}
	ap.Keybase = keybase

	timeout, err := time.ParseDuration(ap.PCfg.Timeout)
	if err != nil {
		return err
	}

	rpcClient, err := NewRPCClient(ap.PCfg.RPCAddr, timeout)
	if err != nil {
		return err
	}
	ap.RPCClient = rpcClient

	lightprovider, err := prov.New(ap.PCfg.ChainID, ap.PCfg.RPCAddr)
	if err != nil {
		return err
	}
	ap.LightProvider = lightprovider

	ap.SetSDKContext()

	clientCtx := client.Context{}.
		WithClient(rpcClient).
		WithFromName(ap.PCfg.Key).
		WithTxConfig(app.MakeEncodingConfig().TxConfig).
		WithSkipConfirmation(true).
		WithBroadcastMode("sync").
		WithCodec(ap.Cdc.Marshaler).
		WithInterfaceRegistry(ap.Cdc.InterfaceRegistry).
		WithAccountRetriever(authtypes.AccountRetriever{})

	addr, _ := ap.GetKeyAddress()
	if addr != nil {
		clientCtx = clientCtx.
			WithFromAddress(addr)

	}

	ap.QueryClient = wasmtypes.NewQueryClient(clientCtx)
	ap.ClientCtx = clientCtx
	return nil
}

func (ap *ArchwayProvider) Address() (string, error) {
	info, err := ap.Keybase.Key(ap.PCfg.Key)
	if err != nil {
		return "", err
	}

	acc, err := info.GetAddress()
	if err != nil {
		return "", err
	}

	out, err := ap.EncodeBech32AccAddr(acc)
	if err != nil {
		return "", err
	}

	return out, err
}

func (cc *ArchwayProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	// res, err := cc.QueryStakingParams(ctx)

	// TODO: check and rewrite
	var unbondingTime time.Duration
	// if err != nil {
	// 	// Attempt ICS query
	// 	consumerUnbondingPeriod, consumerErr := cc.queryConsumerUnbondingPeriod(ctx)
	// 	if consumerErr != nil {
	// 		return 0,
	// 			fmt.Errorf("failed to query unbonding period as both standard and consumer chain: %s: %w", err.Error(), consumerErr)
	// 	}
	// 	unbondingTime = consumerUnbondingPeriod
	// } else {
	// 	unbondingTime = res.UnbondingTime
	// }

	// // We want the trusting period to be 85% of the unbonding time.
	// // Go mentions that the time.Duration type can track approximately 290 years.
	// // We don't want to lose precision if the duration is a very long duration
	// // by converting int64 to float64.
	// // Use integer math the whole time, first reducing by a factor of 100
	// // and then re-growing by 85x.
	tp := unbondingTime / 100 * 85

	// // And we only want the trusting period to be whole hours.
	// // But avoid rounding if the time is less than 1 hour
	// //  (otherwise the trusting period will go to 0)
	if tp > time.Hour {
		tp = tp.Truncate(time.Hour)
	}
	return tp, nil
}

func (cc *ArchwayProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := cc.Cdc.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (cc *ArchwayProvider) QueryStatus(ctx context.Context) (*ctypes.ResultStatus, error) {
	status, err := cc.RPCClient.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query node status: %w", err)
	}
	return status, nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (cc *ArchwayProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	// var initial int64
	// h, err := cc.RPCClient.Status(ctx)
	// if err != nil {
	// 	return err
	// }
	// if h.SyncInfo.CatchingUp {
	// 	return fmt.Errorf("chain catching up")
	// }
	// initial = h.SyncInfo.LatestBlockHeight
	// for {
	// 	h, err = cc.RPCClient.Status(ctx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if h.SyncInfo.LatestBlockHeight > initial+n {
	// 		return nil
	// 	}
	// 	select {
	// 	case <-time.After(10 * time.Millisecond):
	// 		// Nothing to do.
	// 	case <-ctx.Done():
	// 		return ctx.Err()
	// 	}
	// }
	return nil
}

func (ac *ArchwayProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	resultBlock, err := ac.RPCClient.Block(ctx, &height)
	if err != nil {
		return time.Time{}, err
	}
	return resultBlock.Block.Time, nil
}

func (ac *ArchwayProvider) Codec() Codec {
	return ac.Cdc
}

func (ap *ArchwayProvider) ClientContext() client.Context {
	return ap.ClientCtx
}

func (ap *ArchwayProvider) updateNextAccountSequence(seq uint64) {
	if seq > ap.nextAccountSeq {
		ap.nextAccountSeq = seq
	}
}

func (app *ArchwayProvider) MsgRegisterCounterpartyPayee(portID, channelID, relayerAddr, counterpartyPayeeAddr string) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented for Icon")
}

func (cc *ArchwayProvider) FirstRetryBlockAfter() uint64 {
	if cc.PCfg.FirstRetryBlockAfter != 0 {
		return cc.PCfg.FirstRetryBlockAfter
	}
	return 3
}

// keysDir returns a string representing the path on the local filesystem where the keystore will be initialized.
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// NewRPCClient initializes a new tendermint RPC client connected to the specified address.
func NewRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
	return client.NewClientFromNode(addr)
}