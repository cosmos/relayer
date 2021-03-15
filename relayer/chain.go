package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	retry "github.com/avast/retry-go"
	sdkCtx "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	tx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	keys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	connectiontypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	ibctmtypes "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/cosmos/go-bip39"
	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"golang.org/x/sync/errgroup"
)

var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)

	AllowUpdateAfterExpiry       = true
	AllowUpdateAfterMisbehaviour = true
)

// Chain represents the necessary data for connecting to and indentifying a chain and its counterparites
type Chain struct {
	Key            string  `yaml:"key" json:"key"`
	ChainID        string  `yaml:"chain-id" json:"chain-id"`
	RPCAddr        string  `yaml:"rpc-addr" json:"rpc-addr"`
	AccountPrefix  string  `yaml:"account-prefix" json:"account-prefix"`
	GasAdjustment  float64 `yaml:"gas-adjustment" json:"gas-adjustment"`
	GasPrices      string  `yaml:"gas-prices" json:"gas-prices"`
	TrustingPeriod string  `yaml:"trusting-period" json:"trusting-period"`

	// TODO: make these private
	HomePath string                `yaml:"-" json:"-"`
	PathEnd  *PathEnd              `yaml:"-" json:"-"`
	Keybase  keys.Keyring          `yaml:"-" json:"-"`
	Client   rpcclient.Client      `yaml:"-" json:"-"`
	Encoding params.EncodingConfig `yaml:"-" json:"-"`

	address sdk.AccAddress
	logger  log.Logger
	timeout time.Duration
	debug   bool

	// stores faucet addresses that have been used reciently
	faucetAddrs map[string]time.Time
}

// ValidatePaths takes two chains and validates their paths
func ValidatePaths(src, dst *Chain) error {
	if err := src.PathEnd.ValidateFull(); err != nil {
		return src.ErrCantSetPath(err)
	}
	if err := dst.PathEnd.ValidateFull(); err != nil {
		return dst.ErrCantSetPath(err)
	}
	return nil
}

// ValidateClientPaths takes two chains and validates their clients
func ValidateClientPaths(src, dst *Chain) error {
	if err := src.PathEnd.Vclient(); err != nil {
		return err
	}
	if err := dst.PathEnd.Vclient(); err != nil {
		return err
	}
	return nil
}

// ValidateConnectionPaths takes two chains and validates the connections
// and underlying client identifiers
func ValidateConnectionPaths(src, dst *Chain) error {
	if err := src.PathEnd.Vclient(); err != nil {
		return err
	}
	if err := dst.PathEnd.Vclient(); err != nil {
		return err
	}
	if err := src.PathEnd.Vconn(); err != nil {
		return err
	}
	if err := dst.PathEnd.Vconn(); err != nil {
		return err
	}
	return nil
}

// ValidateChannelParams takes two chains and validates their respective channel params
func ValidateChannelParams(src, dst *Chain) error {
	if err := src.PathEnd.ValidateBasic(); err != nil {
		return err
	}
	if err := dst.PathEnd.ValidateBasic(); err != nil {
		return err
	}
	//nolint:staticcheck
	if strings.ToUpper(src.PathEnd.Order) != strings.ToUpper(dst.PathEnd.Order) {
		return fmt.Errorf("src and dst path ends must have same ORDER. got src: %s, dst: %s",
			src.PathEnd.Order, dst.PathEnd.Order)
	}
	return nil
}

// ListenRPCEmitJSON listens for tx and block events from a chain and outputs them as JSON to stdout
func (c *Chain) ListenRPCEmitJSON(tx, block, data bool) func() {
	doneChan := make(chan struct{})
	go c.listenLoop(doneChan, tx, block, data)
	return func() { doneChan <- struct{}{} }
}

func (c *Chain) listenLoop(doneChan chan struct{}, tx, block, data bool) {
	// Subscribe to source chain
	if err := c.Start(); err != nil {
		c.Error(err)
		return
	}

	srcTxEvents, srcTxCancel, err := c.Subscribe(txEvents)
	if err != nil {
		c.Error(err)
		return
	}
	defer srcTxCancel()

	srcBlockEvents, srcBlockCancel, err := c.Subscribe(blEvents)
	if err != nil {
		c.Error(err)
		return
	}
	defer srcBlockCancel()

	// Listen to channels and take appropriate action
	var byt []byte
	var mar interface{}
	for {
		select {
		case srcMsg := <-srcTxEvents:
			switch {
			case tx:
				continue
			case data:
				mar = srcMsg
			default:
				mar = srcMsg.Events
			}
			if byt, err = json.Marshal(mar); err != nil {
				c.Error(err)
			}
			fmt.Println(string(byt))
		case srcMsg := <-srcBlockEvents:
			switch {
			case block:
				continue
			case data:
				mar = srcMsg
			default:
				mar = srcMsg.Events
			}
			if byt, err = json.Marshal(mar); err != nil {
				c.Error(err)
			}
			fmt.Println(string(byt))
		case <-doneChan:
			close(doneChan)
			return
		}
	}
}

// Init initializes the pieces of a chain that aren't set when it parses a config
// NOTE: All validation of the chain should happen here.
func (c *Chain) Init(homePath string, timeout time.Duration, logger log.Logger, debug bool) error {
	keybase, err := keys.New(c.ChainID, "test", keysDir(homePath, c.ChainID), nil)
	if err != nil {
		return err
	}

	client, err := newRPCClient(c.RPCAddr, timeout)
	if err != nil {
		return err
	}

	_, err = time.ParseDuration(c.TrustingPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse trusting period (%s) for chain %s", c.TrustingPeriod, c.ChainID)
	}

	_, err = sdk.ParseDecCoins(c.GasPrices)
	if err != nil {
		return fmt.Errorf("failed to parse gas prices (%s) for chain %s", c.GasPrices, c.ChainID)
	}

	encodingConfig := c.MakeEncodingConfig()

	c.Keybase = keybase
	c.Client = client
	c.HomePath = homePath
	c.Encoding = encodingConfig
	c.logger = logger
	c.timeout = timeout
	c.debug = debug
	c.faucetAddrs = make(map[string]time.Time)

	if c.logger == nil {
		c.logger = defaultChainLogger()
	}

	return nil
}

func defaultChainLogger() log.Logger {
	return log.NewTMLogger(log.NewSyncWriter(os.Stdout))
}

// KeyExists returns true if there is a specified key in chain's keybase
func (c *Chain) KeyExists(name string) bool {
	k, err := c.Keybase.Key(name)
	if err != nil {
		return false
	}

	return k.GetName() == name
}

// GetSelfVersion returns the version of the given chain
func (c *Chain) GetSelfVersion() uint64 {
	return clienttypes.ParseChainID(c.ChainID)
}

// GetTrustingPeriod returns the trusting period for the chain
func (c *Chain) GetTrustingPeriod() time.Duration {
	tp, _ := time.ParseDuration(c.TrustingPeriod)
	return tp
}

func newRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
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

// SendMsg wraps the msg in a stdtx, signs and sends it
func (c *Chain) SendMsg(datagram sdk.Msg) (*sdk.TxResponse, bool, error) {
	return c.SendMsgs([]sdk.Msg{datagram})
}

// SendMsgs wraps the msgs in a StdTx, signs and sends it. An error is returned if there
// was an issue sending the transaction. A successfully sent, but failed transaction will
// not return an error. If a transaction is successfully sent, the result of the execution
// of that transaction will be logged. A boolean indicating if a transaction was successfully
// sent and executed successfully is returned.
func (c *Chain) SendMsgs(msgs []sdk.Msg) (*sdk.TxResponse, bool, error) {
	// Instantiate the client context
	ctx := c.CLIContext(0)

	// Query account details
	txf, err := tx.PrepareFactory(ctx, c.TxFactory(0))
	if err != nil {
		return nil, false, err
	}

	// If users pass gas adjustment, then calculate gas
	_, adjusted, err := tx.CalculateGas(ctx.QueryWithData, txf, msgs...)
	if err != nil {
		return nil, false, err
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	// Build the transaction builder
	txb, err := tx.BuildUnsignedTx(txf, msgs...)
	if err != nil {
		return nil, false, err
	}

	// Attach the signature to the transaction
	err = tx.Sign(txf, c.Key, txb, false)
	if err != nil {
		return nil, false, err
	}

	// Generate the transaction bytes
	txBytes, err := ctx.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return nil, false, err
	}

	// Broadcast those bytes
	res, err := ctx.BroadcastTx(txBytes)
	if err != nil {
		return nil, false, err
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if res.Code != 0 {
		c.LogFailedTx(res, err, msgs)
		return res, false, nil
	}

	c.LogSuccessTx(res, msgs)
	return res, true, nil
}

// CLIContext returns an instance of client.Context derived from Chain
func (c *Chain) CLIContext(height int64) sdkCtx.Context {
	return sdkCtx.Context{}.
		WithChainID(c.ChainID).
		WithJSONMarshaler(newContextualStdCodec(c.Encoding.Marshaler, c.UseSDKContext)).
		WithInterfaceRegistry(c.Encoding.InterfaceRegistry).
		WithTxConfig(c.Encoding.TxConfig).
		WithLegacyAmino(c.Encoding.Amino).
		WithInput(os.Stdin).
		WithNodeURI(c.RPCAddr).
		WithClient(c.Client).
		WithAccountRetriever(authTypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastBlock).
		WithKeyring(c.Keybase).
		WithOutputFormat("json").
		WithFrom(c.Key).
		WithFromName(c.Key).
		WithFromAddress(c.MustGetAddress()).
		WithSkipConfirmation(true).
		WithNodeURI(c.RPCAddr).
		WithHeight(height)
}

// TxFactory returns an instance of tx.Factory derived from
func (c *Chain) TxFactory(height int64) tx.Factory {
	ctx := c.CLIContext(height)
	return tx.Factory{}.
		WithAccountRetriever(ctx.AccountRetriever).
		WithChainID(c.ChainID).
		WithTxConfig(ctx.TxConfig).
		WithGasAdjustment(c.GasAdjustment).
		WithGasPrices(c.GasPrices).
		WithKeybase(c.Keybase).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)
}

// Log takes a string and logs the data
func (c *Chain) Log(s string) {
	c.logger.Info(s)
}

// Error takes an error, wraps it in the chainID and logs the error
func (c *Chain) Error(err error) {
	c.logger.Error(fmt.Sprintf("%s: err(%s)", c.ChainID, err.Error()))
}

// Start the client service
func (c *Chain) Start() error {
	return c.Client.Start()
}

// Subscribe returns channel of events given a query
func (c *Chain) Subscribe(query string) (<-chan ctypes.ResultEvent, context.CancelFunc, error) {
	suffix, err := GenerateRandomString(8)
	if err != nil {
		return nil, nil, err
	}

	subscriber := fmt.Sprintf("%s-subscriber-%s", c.ChainID, suffix)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	eventChan, err := c.Client.Subscribe(ctx, subscriber, query, 1000)
	return eventChan, cancel, err
}

// KeysDir returns the path to the keys for this chain
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

func lightDir(home string) string {
	return path.Join(home, "light")
}

// GetAddress returns the sdk.AccAddress associated with the configured key
func (c *Chain) GetAddress() (sdk.AccAddress, error) {
	defer c.UseSDKContext()()
	if c.address != nil {
		return c.address, nil
	}

	// Signing key for c chain
	srcAddr, err := c.Keybase.Key(c.Key)
	if err != nil {
		return nil, err
	}

	return srcAddr.GetAddress(), nil
}

// MustGetAddress used for brevity
func (c *Chain) MustGetAddress() sdk.AccAddress {
	srcAddr, err := c.GetAddress()
	if err != nil {
		panic(err)
	}

	return srcAddr
}

var sdkContextMutex sync.Mutex

// UseSDKContext uses a custom Bech32 account prefix and returns a restore func
// CONTRACT: When using this function, caller must ensure that lock contention
// doesn't cause program to hang. This function is only for use in codec calls
func (c *Chain) UseSDKContext() func() {
	// Ensure we're the only one using the global context,
	// lock context to begin function
	sdkContextMutex.Lock()

	// Mutate the sdkConf
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(c.AccountPrefix, c.AccountPrefix+"pub")
	sdkConf.SetBech32PrefixForValidator(c.AccountPrefix+"valoper", c.AccountPrefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(c.AccountPrefix+"valcons", c.AccountPrefix+"valconspub")

	// Return the unlock function, caller must lock and ensure that lock is released
	// before any other function needs to use c.UseSDKContext
	return sdkContextMutex.Unlock
}

func (c *Chain) String() string {
	out, _ := json.Marshal(c)
	return string(out)
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
		if _, err = rpchttp.New(value, "/websocket"); err != nil {
			return
		}
		out.RPCAddr = value
	case "gas-adjustment":
		adj, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		out.GasAdjustment = adj
	case "gas-prices":
		_, err = sdk.ParseDecCoins(value)
		if err != nil {
			return nil, err
		}
		out.GasPrices = value
	case "account-prefix":
		out.AccountPrefix = value
	case "trusting-period":
		if _, err = time.ParseDuration(value); err != nil {
			return
		}
		out.TrustingPeriod = value
	default:
		return out, fmt.Errorf("key %s not found", key)
	}

	return out, err
}

// Print fmt.Printlns the json or yaml representation of whatever is passed in
// CONTRACT: The cmd calling this function needs to have the "json" and "indent" flags set
// TODO: better "text" printing here would be a nice to have
// TODO: fix indenting all over the code base
func (c *Chain) Print(toPrint proto.Message, text, indent bool) error {
	var (
		out []byte
		err error
	)

	switch {
	case indent && text:
		return fmt.Errorf("must pass either indent or text, not both")
	case text:
		// TODO: This isn't really a good option,
		out = []byte(fmt.Sprintf("%v", toPrint))
	default:
		out, err = c.Encoding.Marshaler.MarshalJSON(toPrint)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

// SendAndPrint sends a transaction and prints according to the passed args
func (c *Chain) SendAndPrint(txs []sdk.Msg, text, indent bool) (err error) {
	if c.debug {
		for _, msg := range txs {
			if err = c.Print(msg, text, indent); err != nil {
				return err
			}
		}
	}
	// SendAndPrint sends the transaction with printing options from the CLI
	res, _, err := c.SendMsgs(txs)
	if err != nil {
		return err
	}

	return c.Print(res, text, indent)

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

// MustGet returns the chain and panics on any error
func (c Chains) MustGet(chainID string) *Chain {
	out, err := c.Get(chainID)
	if err != nil {
		panic(err)
	}
	return out
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

// GetRPCPort returns the port configured for the chain
func (c *Chain) GetRPCPort() string {
	u, _ := url.Parse(c.RPCAddr)
	return u.Port()
}

// CreateTestKey creates a key for test chain
func (c *Chain) CreateTestKey() error {
	if c.KeyExists(c.Key) {
		return fmt.Errorf("key %s exists for chain %s", c.ChainID, c.Key)
	}

	mnemonic, err := CreateMnemonic()
	if err != nil {
		return err
	}

	_, err = c.Keybase.NewAccount(c.Key, mnemonic, "", hd.CreateHDPath(118, 0, 0).String(), hd.Secp256k1)
	return err
}

// CreateMnemonic creates a new mnemonic
func CreateMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

// GetTimeout returns the chain's configured timeout
func (c *Chain) GetTimeout() time.Duration {
	return c.timeout
}

// StatusErr returns err unless the chain is ready to go
func (c *Chain) StatusErr() error {
	stat, err := c.Client.Status(context.Background())
	switch {
	case err != nil:
		return err
	case stat.SyncInfo.LatestBlockHeight < 3:
		return fmt.Errorf("haven't produced any blocks yet")
	default:
		return nil
	}
}

// GenerateConnHandshakeProof generates all the proofs needed to prove the existence of the
// connection state on this chain. A counterparty should use these generated proofs.
func (c *Chain) GenerateConnHandshakeProof(height uint64) (clientState ibcexported.ClientState,
	clientStateProof []byte, consensusProof []byte, connectionProof []byte,
	connectionProofHeight clienttypes.Height, err error) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *connectiontypes.QueryConnectionResponse

		eg = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = c.QueryClientState(int64(height))
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		consensusStateRes, err = c.QueryClientConsensusState(int64(height), clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		connectionStateRes, err = c.QueryConnection(int64(height))
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof,
		connectionStateRes.ProofHeight, nil

}

// UpgradeChain submits and upgrade proposal using a zero'd out client state with an updated unbonding period.
func (c *Chain) UpgradeChain(dst *Chain, plan *upgradetypes.Plan, deposit sdk.Coin,
	unbondingPeriod time.Duration) error {
	if _, _, err := UpdateLightClients(c, dst); err != nil {
		return err
	}
	height := int64(dst.MustGetLatestLightHeight())

	clientStateRes, err := dst.QueryClientState(height)
	if err != nil {
		return err
	}
	clientState, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return err
	}

	upgradedClientState := clientState.ZeroCustomFields().(*ibctmtypes.ClientState)
	upgradedClientState.LatestHeight.RevisionHeight = uint64(plan.Height + 1)
	upgradedClientState.UnbondingPeriod = unbondingPeriod
	upgradedAny, err := clienttypes.PackClientState(upgradedClientState)
	if err != nil {
		return err
	}

	plan.UpgradedClientState = upgradedAny

	// TODO: make cli args for title and description
	upgradeProposal := upgradetypes.NewSoftwareUpgradeProposal("upgrade",
		"upgrade the chain's software and unbonding period", *plan)
	msg, err := govtypes.NewMsgSubmitProposal(upgradeProposal, sdk.NewCoins(deposit), c.MustGetAddress())
	if err != nil {
		return err
	}

	_, _, err = c.SendMsg(msg)
	if err != nil {
		return err
	}

	return nil
}
