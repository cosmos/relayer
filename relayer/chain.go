package relayer

import (
	"context"
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
	"gopkg.in/yaml.v2"
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
}

// Init initializes the pieces of a chain that aren't set when it parses a config
// NOTE: All validation of the chain should happen here.
func (c *Chain) Init(homePath string, cdc *codecstd.Codec, amino *aminocodec.Codec, timeout time.Duration) (*Chain, error) {
	keybase, err := keys.NewKeyring(c.ChainID, "test", keysDir(homePath), nil)
	if err != nil {
		return nil, err
	}

	client, err := newRPCClient(c.RPCAddr, timeout)
	if err != nil {
		return nil, err
	}

	_, err = sdk.ParseDecCoins(c.GasPrices)
	if err != nil {
		return nil, err
	}

	_, err = time.ParseDuration(c.TrustingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse trusting period (%s) for chain %s", c.TrustingPeriod, c.ChainID)
	}

	return &Chain{
		Key:            c.Key,
		ChainID:        c.ChainID,
		RPCAddr:        c.RPCAddr,
		AccountPrefix:  c.AccountPrefix,
		Gas:            c.Gas,
		GasAdjustment:  c.GasAdjustment,
		GasPrices:      c.GasPrices,
		DefaultDenom:   c.DefaultDenom,
		Memo:           c.Memo,
		TrustingPeriod: c.TrustingPeriod,
		Keybase:        keybase,
		Client:         client,
		Cdc:            cdc,
		Amino:          amino,
		HomePath:       homePath,
		logger:         log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		timeout:        timeout,
	}, nil
}

func (c *Chain) getGasPrices() sdk.DecCoins {
	gp, _ := sdk.ParseDecCoins(c.GasPrices)
	return gp
}

func (c *Chain) getTrustingPeriod() time.Duration {
	tp, _ := time.ParseDuration(c.TrustingPeriod)
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

// BuildAndSignTx takes messages and builds, signs and marshals a sdk.Tx to prepare it for broadcast
func (c *Chain) BuildAndSignTx(datagram []sdk.Msg) ([]byte, error) {
	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(c.Cdc, c).GetAccount(c.MustGetAddress())
	if err != nil {
		return nil, err
	}

	return auth.NewTxBuilder(
		auth.DefaultTxEncoder(c.Amino), acc.GetAccountNumber(),
		acc.GetSequence(), c.Gas, c.GasAdjustment, false, c.ChainID,
		c.Memo, sdk.NewCoins(), c.getGasPrices()).WithKeybase(c.Keybase).
		BuildAndSign(c.Key, ckeys.DefaultKeyPass, datagram)
}

// BroadcastTxCommit takes the marshaled transaction bytes and broadcasts them
func (c *Chain) BroadcastTxCommit(txBytes []byte) (sdk.TxResponse, error) {
	res, err := sdkCtx.CLIContext{Client: c.Client}.BroadcastTxCommit(txBytes)
	// NOTE: The raw is a recreation of the log and unused in this context. It is also
	// quite noisy so we remove it to simplify the output
	res.RawLog = ""
	return res, err
}

// Log takes a string and logs the data
func (c *Chain) Log(s string) {
	c.logger.Info(s)
}

// Error takes an error, wraps it in the chainID and logs the error
func (c *Chain) Error(err error) {
	c.logger.Error(fmt.Sprintf("%s: err(%s)", c.ChainID, err.Error()))
}

// Subscribe returns channel of events given a query
func (c *Chain) Subscribe(query string) (<-chan ctypes.ResultEvent, context.CancelFunc, error) {
	if err := c.Client.OnStart(); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	eventChan, err := c.Client.Subscribe(ctx, fmt.Sprintf("%s-subscriber", c.ChainID), query)
	return eventChan, cancel, err
}

// KeysDir returns the path to the keys for this chain
func keysDir(home string) string {
	return path.Join(home, "keys")
}

func liteDir(home string) string {
	return path.Join(home, "lite")
}

// GetAddress returns the sdk.AccAddress associated with the configred key
func (c *Chain) GetAddress() (sdk.AccAddress, error) {
	if c.address != nil {
		return c.address, nil
	}
	// Signing key for src chain
	srcAddr, err := c.Keybase.Get(c.Key)
	if err != nil {
		return nil, err
	}
	c.address = srcAddr.GetAddress()
	return srcAddr.GetAddress(), nil
}

// MustGetAddress used for brevity
func (c *Chain) MustGetAddress() sdk.AccAddress {
	srcAddr, err := c.Keybase.Get(c.Key)
	if err != nil {
		panic(err)
	}
	return srcAddr.GetAddress()
}

func (c *Chain) String() string {
	return c.ChainID
}

// Update returns a new chain with updated values
func (c *Chain) Update(key, value string) (out *Chain, err error) {
	out = c
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
func (c *Chain) Print(toPrint interface{}, text, indent bool) error {
	var (
		out []byte
		err error
	)

	switch {
	case indent:
		out, err = c.Amino.MarshalJSONIndent(toPrint, "", "  ")
	case text:
		out, err = yaml.Marshal(&toPrint)
	default:
		out, err = c.Amino.MarshalJSON(toPrint)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
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
