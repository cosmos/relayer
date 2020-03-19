package relayer

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	sdkCtx "github.com/cosmos/cosmos-sdk/client/context"
	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	aminocodec "github.com/cosmos/cosmos-sdk/codec"
	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/tendermint/tendermint/libs/log"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	libclient "github.com/tendermint/tendermint/rpc/lib/client"
)

// Chain represents the necessary data for connecting to and indentifying a chain and its counterparites
type Chain struct {
	Key            string  `yaml:"key" json:"key"`
	ChainID        string  `yaml:"chain-id" json:"chain-id"`
	RPCAddr        string  `yaml:"rpc-addr" json:"rpc-addr"`
	AccountPrefix  string  `yaml:"account-prefix" json:"account-prefix"`
	Gas            uint64  `yaml:"gas,omitempty" json:"gas,omitempty"`
	GasAdjustment  float64 `yaml:"gas-adjustment,omitempty" json:"gas-adjustment,omitempty"`
	GasPrices      string  `yaml:"gas-prices,omitempty" json:"gas-prices,omitempty"`
	DefaultDenom   string  `yaml:"default-denom,omitempty" json:"default-denom,omitempty"`
	Memo           string  `yaml:"memo,omitempty" json:"memo,omitempty"`
	TrustingPeriod string  `yaml:"trusting-period" json:"trusting-period"`

	// TODO: make these private
	HomePath string            `yaml:"-" json:"-"`
	PathEnd  *PathEnd          `yaml:"-" json:"-"`
	Keybase  keys.Keybase      `yaml:"-" json:"-"`
	Client   rpcclient.Client  `yaml:"-" json:"-"`
	Cdc      *codecstd.Codec   `yaml:"-" json:"-"`
	Amino    *aminocodec.Codec `yaml:"-" json:"-"`

	address sdk.AccAddress
	logger  log.Logger
	timeout time.Duration
	debug   bool

	// stores facuet addresses that have been used reciently
	faucetAddrs map[string]time.Time
}

// Init initializes the pieces of a chain that aren't set when it parses a config
// NOTE: All validation of the chain should happen here.
func (src *Chain) Init(homePath string, cdc *codecstd.Codec, amino *aminocodec.Codec, timeout time.Duration, debug bool) (*Chain, error) {
	keybase, err := keys.NewKeyring(src.ChainID, "test", keysDir(homePath), nil)
	if err != nil {
		return nil, err
	}

	client, err := newRPCClient(src.RPCAddr, timeout)
	if err != nil {
		return nil, err
	}

	_, err = sdk.ParseDecCoins(src.GasPrices)
	if err != nil {
		return nil, err
	}

	_, err = time.ParseDuration(src.TrustingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse trusting period (%s) for chain %s", src.TrustingPeriod, src.ChainID)
	}

	return &Chain{
		Key:            src.Key,
		ChainID:        src.ChainID,
		RPCAddr:        src.RPCAddr,
		AccountPrefix:  src.AccountPrefix,
		Gas:            src.Gas,
		GasAdjustment:  src.GasAdjustment,
		GasPrices:      src.GasPrices,
		DefaultDenom:   src.DefaultDenom,
		Memo:           src.Memo,
		TrustingPeriod: src.TrustingPeriod,
		Keybase:        keybase,
		Client:         client,
		Cdc:            cdc,
		Amino:          amino,
		HomePath:       homePath,
		logger:         log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		timeout:        timeout,
		debug:          debug,
		faucetAddrs:    make(map[string]time.Time),
	}, nil
}

func (src *Chain) getGasPrices() sdk.DecCoins {
	gp, _ := sdk.ParseDecCoins(src.GasPrices)
	return gp
}

// GetTrustingPeriod returns the trusting period for the chain
func (src *Chain) GetTrustingPeriod() time.Duration {
	tp, _ := time.ParseDuration(src.TrustingPeriod)
	return tp
}

func newRPCClient(addr string, timeout time.Duration) (*rpcclient.HTTP, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}

	// TODO: Replace with the global timeout value?
	httpClient.Timeout = timeout
	rpcClient, err := rpcclient.NewHTTPWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}

// SendMsg wraps the msg in a stdtx, signs and sends it
func (src *Chain) SendMsg(datagram sdk.Msg) (sdk.TxResponse, error) {
	return src.SendMsgs([]sdk.Msg{datagram})
}

// SendMsgs wraps the msgs in a stdtx, signs and sends it
func (src *Chain) SendMsgs(datagrams []sdk.Msg) (res sdk.TxResponse, err error) {
	var out []byte
	if out, err = src.BuildAndSignTx(datagrams); err != nil {
		return res, err
	}
	return src.BroadcastTxCommit(out)
}

// BuildAndSignTx takes messages and builds, signs and marshals a sdk.Tx to prepare it for broadcast
func (src *Chain) BuildAndSignTx(datagram []sdk.Msg) ([]byte, error) {
	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(src.Cdc, src).GetAccount(src.MustGetAddress())
	if err != nil {
		return nil, err
	}

	return auth.NewTxBuilder(
		auth.DefaultTxEncoder(src.Amino), acc.GetAccountNumber(),
		acc.GetSequence(), src.Gas, src.GasAdjustment, false, src.ChainID,
		src.Memo, sdk.NewCoins(), src.getGasPrices()).WithKeybase(src.Keybase).
		BuildAndSign(src.Key, ckeys.DefaultKeyPass, datagram)
}

// BroadcastTxCommit takes the marshaled transaction bytes and broadcasts them
func (src *Chain) BroadcastTxCommit(txBytes []byte) (sdk.TxResponse, error) {
	res, err := sdkCtx.CLIContext{Client: src.Client}.BroadcastTxCommit(txBytes)

	if !src.debug {
		res.RawLog = ""
	}

	return res, err
}

// Log takes a string and logs the data
func (src *Chain) Log(s string) {
	src.logger.Info(s)
}

// Error takes an error, wraps it in the chainID and logs the error
func (src *Chain) Error(err error) {
	src.logger.Error(fmt.Sprintf("%s: err(%s)", src.ChainID, err.Error()))
}

// Subscribe returns channel of events given a query
func (src *Chain) Subscribe(query string) (<-chan ctypes.ResultEvent, context.CancelFunc, error) {
	if err := src.Client.Start(); err != nil {
		return nil, nil, err
	}

	suffix, err := GenerateRandomString(8)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	eventChan, err := src.Client.Subscribe(ctx, fmt.Sprintf("%s-subscriber-%s", src.ChainID, suffix), query)
	return eventChan, cancel, err
}

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomString(s int) (string, error) {
	b, err := GenerateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

// KeysDir returns the path to the keys for this chain
func keysDir(home string) string {
	return path.Join(home, "keys")
}

func liteDir(home string) string {
	return path.Join(home, "lite")
}

// GetAddress returns the sdk.AccAddress associated with the configred key
func (src *Chain) GetAddress() (sdk.AccAddress, error) {
	if src.address != nil {
		return src.address, nil
	}
	// Signing key for src chain
	srcAddr, err := src.Keybase.Get(src.Key)
	if err != nil {
		return nil, err
	}
	src.address = srcAddr.GetAddress()
	return srcAddr.GetAddress(), nil
}

// MustGetAddress used for brevity
func (src *Chain) MustGetAddress() sdk.AccAddress {
	srcAddr, err := src.GetAddress()
	if err != nil {
		panic(err)
	}
	return srcAddr
}

func (src *Chain) String() string {
	return src.ChainID
}

// Update returns a new chain with updated values
func (src *Chain) Update(key, value string) (out *Chain, err error) {
	out = src
	switch key {
	case "key":
		out.Key = value
	case "chain-id":
		out.ChainID = value
	case "rpc-addr":
		if _, err = rpcclient.NewHTTP(value, "/websocket"); err != nil {
			return
		}
		out.RPCAddr = value
	case "account-prefix":
		out.AccountPrefix = value
	case "gas":
		var gas uint64
		gas, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return
		}
		out.Gas = gas
	case "gas-prices":
		if _, err = sdk.ParseDecCoins(value); err != nil {
			return
		}
		out.GasPrices = value
	case "default-denom":
		out.DefaultDenom = value
	case "memo":
		out.Memo = value
	case "trusting-period":
		if _, err = time.ParseDuration(value); err != nil {
			return
		}
		out.TrustingPeriod = value
	default:
		return out, fmt.Errorf("key %s not found", key)
	}

	return
}

// Print fmt.Printlns the json or yaml representation of whatever is passed in
// CONTRACT: The cmd calling this function needs to have the "json" and "indent" flags set
// TODO: better "text" printing here would be a nice to have
func (src *Chain) Print(toPrint interface{}, text, indent bool) error {
	var (
		out []byte
		err error
	)

	switch {
	case indent:
		out, err = src.Amino.MarshalJSONIndent(toPrint, "", "  ")
	case text:
		out = []byte(fmt.Sprintf("%v", toPrint))
	default:
		out, err = src.Amino.MarshalJSON(toPrint)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

// SendAndPrint sends a transaction and prints according to the passed args
func (src *Chain) SendAndPrint(txs []sdk.Msg, text, indent bool) (err error) {
	if src.debug {
		if err = src.Print(txs, text, indent); err != nil {
			return err
		}
	}
	// SendAndPrint sends the transaction with printing options from the CLI
	res, err := src.SendMsgs(txs)
	if err != nil {
		return err
	}

	return src.Print(res, text, indent)

}

// Chains is a collection of Chain
type Chains []*Chain

// Get returns the configuration for a given chain
func (c Chains) Get(chainID string) (*Chain, error) {
	for _, chain := range c {
		if chainID == chain.ChainID {
			addr, _ := chain.GetAddress()
			chain.address = addr
			return chain, nil
		}
	}
	return &Chain{}, fmt.Errorf("chain with ID %s is not configured", chainID)
}

// Gets returns a map chainIDs to their chains
func (c Chains) Gets(chainIDs ...string) (map[string]*Chain, error) {
	out := make(map[string]*Chain)
	for _, cid := range chainIDs {
		chain, err := c.Get(cid)
		if err != nil {
			return out, err
		}
		out[cid] = chain
	}
	return out, nil
}
