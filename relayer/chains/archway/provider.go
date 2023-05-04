package archway

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/gogoproto/proto"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"

	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &ArchwayProvider{}
	_ provider.KeyProvider    = &ArchwayProvider{}
	_ provider.ProviderConfig = &ArchwayProviderConfig{}
)

type ArchwayProviderConfig struct {
	KeyDirectory      string                 `json:"key-directory" yaml:"key-directory"`
	Key               string                 `json:"key" yaml:"key"`
	ChainName         string                 `json:"-" yaml:"-"`
	ChainID           string                 `json:"chain-id" yaml:"chain-id"`
	RPCAddr           string                 `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix     string                 `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend    string                 `json:"keyring-backend" yaml:"keyring-backend"`
	GasAdjustment     float64                `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices         string                 `json:"gas-prices" yaml:"gas-prices"`
	MinGasAmount      uint64                 `json:"min-gas-amount" yaml:"min-gas-amount"`
	Timeout           string                 `json:"timeout" yaml:"timeout"`
	Keystore          string                 `json:"keystore" yaml:"keystore"`
	Password          string                 `json:"password" yaml:"password"`
	IbcHandlerAddress string                 `json:"ibc-handler-address" yaml:"ibc-handler-address"`
	Broadcast         provider.BroadcastMode `json:"broadcast-mode" yaml:"broadcast-mode"`
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

func (pp *ArchwayProviderConfig) Set(field string, value interface{}) error {
	// TODO: implement
	return nil
}

func (pp *ArchwayProviderConfig) getRPCAddr() string {
	return pp.RPCAddr
}

func (pp *ArchwayProviderConfig) BroadcastMode() provider.BroadcastMode {
	return pp.Broadcast
}

func (pp ArchwayProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {

	if err := pp.Validate(); err != nil {
		return nil, err
	}

	pp.KeyDirectory = keysDir(homepath, pp.ChainID)

	pp.ChainName = chainName

	if pp.Broadcast == "" {
		pp.Broadcast = provider.BroadcastModeBatch
	}

	codec := MakeCodec(ModuleBasics, []string{})

	return &ArchwayProvider{
		log:    log.With(zap.String("sys", "chain_client")),
		PCfg:   &pp,
		Cdc:    codec,
		Input:  os.Stdin,
		Output: os.Stdout,
	}, nil
}

type ArchwayProvider struct {
	log *zap.Logger

	PCfg           *ArchwayProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	RPCClient      rpcclient.Client //TODO: check the client
	Input          io.Reader
	Output         io.Writer

	Cdc Codec

	txMu sync.Mutex

	metrics *processor.PrometheusMetrics
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

func (ap *ArchwayProvider) ProviderConfig() provider.ProviderConfig {
	return ap.PCfg
}

func (ap *ArchwayProvider) Timeout() string {
	return ap.PCfg.Timeout
}

// CommitmentPrefix returns the commitment prefix for Cosmos
func (ap *ArchwayProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	return defaultChainPrefix
}

func (ap *ArchwayProvider) Init(ctx context.Context) error {
	// TODO:
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

func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}
