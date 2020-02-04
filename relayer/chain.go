package relayer

import (
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/tendermint/tendermint/libs/log"
	lite "github.com/tendermint/tendermint/lite2"
	dbm "github.com/tendermint/tm-db"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/ibc"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tendermint "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
)

// Exists Returns true if the chain is configured
func Exists(chainID string, c []*Chain) bool {
	for _, chain := range c {
		if chainID == chain.ChainID {
			return true
		}
	}
	return false
}

// GetChain returns the configuration for a given chain
func GetChain(chainID string, c []*Chain) (*Chain, error) {
	for _, chain := range c {
		if chainID == chain.ChainID {
			return chain, nil
		}
	}
	return &Chain{}, fmt.Errorf("chain with ID %s is not configured", chainID)
}

// NewChain returns a new instance of Chain
// NOTE: It does not by default create the verifier. This needs a working connection
// and blocks running the app if NewChain does this by default.
func NewChain(key, chainID, rpcAddr, accPrefix string,
	counterparties []Counterparty, gas uint64, gasAdj float64,
	gasPrices sdk.DecCoins, defaultDenom, memo, homePath string,
	liteCacheSize int, trustingPeriod string, dir string,
) (*Chain, error) {
	keybase, err := keys.NewKeyring(chainID, "test", keysDir(homePath, chainID), nil)
	if err != nil {
		return &Chain{}, err
	}

	client, err := rpcclient.NewHTTP(rpcAddr, "/websocket")
	if err != nil {
		return &Chain{}, err
	}

	cdc := codec.New()
	ibc.AppModuleBasic{}.RegisterCodec(cdc)

	return &Chain{
		Key: key, ChainID: chainID, RPCAddr: rpcAddr, AccountPrefix: accPrefix, Counterparties: counterparties, Gas: gas,
		GasAdjustment: gasAdj, GasPrices: gasPrices, DefaultDenom: defaultDenom, Memo: memo, Keybase: keybase,
		Client: client, Cdc: cdc, TrustingPeriod: trustingPeriod, ChainDir: dir}, nil
}

// Chain represents the necessary data for connecting to and indentifying a chain and its counterparites
type Chain struct {
	Key            string         `yaml:"key"`
	ChainID        string         `yaml:"chain-id"`
	RPCAddr        string         `yaml:"rpc-addr"`
	AccountPrefix  string         `yaml:"account-prefix"`
	Counterparties []Counterparty `yaml:"counterparties"`
	Gas            uint64         `yaml:"gas,omitempty"`
	GasAdjustment  float64        `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins   `yaml:"gas-prices,omitempty"`
	DefaultDenom   string         `yaml:"default-denom,omitempty"`
	Memo           string         `yaml:"memo,omitempty"`
	TrustingPeriod string         `yaml:"trusting-period"`

	Keybase  keys.Keybase
	Client   *rpcclient.HTTP
	Cdc      *codec.Codec
	ChainDir string
	logger   log.Logger
}

// Get gets the lite.TrustOptions struct from this one
func (c *Chain) GetEmptyTrustOptions() lite.TrustOptions {
	return c.GetTrustOptions(-1, nil)
}

func (c *Chain) GetTrustOptions(height int64, hash []byte) lite.TrustOptions {
	dur, err := time.ParseDuration(c.TrustingPeriod)
	if err != nil {
		panic(err)
	}
	return lite.TrustOptions{
		Period: dur,
		Height: height,
		Hash:   hash,
	}
}

// NewCounterparty returns a new instance of Counterparty
func NewCounterparty(chainID, clientID string) Counterparty {
	return Counterparty{chainID, clientID}
}

// Counterparty represents the counterparty to relay against for
type Counterparty struct {
	ChainID  string `yaml:"chain-id"`
	ClientID string `yaml:"client-id"`
}

// GetCounterparty returns the specified counterparty from a given chain
func (c *Chain) GetCounterparty(chainID string) (Counterparty, error) {
	for _, cp := range c.Counterparties {
		if cp.ChainID == chainID {
			return cp, nil
		}
	}
	return Counterparty{}, fmt.Errorf("chain %s has no counterparty with id %s", c.ChainID, chainID)
}

// LiteDir returns the proper directory for the lite client for a given chain
func liteDir(home, chainID string) string {
	return filepath.Join(home, "lite", chainID)
}

// KeysDir returns the path to the keys for this chain
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// QueryWithData allows for running ABCI queries in a similar manner to CLIContext
func (c *Chain) QueryWithData(path string, data []byte) ([]byte, int64, error) {
	resp, err := c.Client.ABCIQuery(path, data)
	if err != nil {
		return []byte{}, 0, err
	}

	return resp.Response.GetValue(), resp.Response.GetHeight(), nil
}

// SendMsgs sends the standard transactions to the individual chain
func (c *Chain) SendMsgs(datagram []sdk.Msg) error {
	// Fetch key address
	info, err := c.Keybase.Get(c.Key)
	if err != nil {
		return err
	}

	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(c).GetAccount(info.GetAddress())
	if err != nil {
		return err
	}

	// Calculate fess from the gas and gas prices
	// TODO: Incorporate c.GasAdjustment here?
	fees := make(sdk.Coins, len(c.GasPrices))
	for i, gp := range c.GasPrices {
		fee := gp.Amount.Mul(sdk.NewDec(int64(c.Gas)))
		fees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
	}

	// Build the StdSignMsg
	sign := auth.StdSignMsg{
		ChainID:       c.ChainID,
		AccountNumber: acc.GetSequence(),
		Sequence:      acc.GetSequence(),
		Memo:          c.Memo,
		Msgs:          datagram,
		Fee:           auth.NewStdFee(c.Gas, fees),
	}

	// Create signature for transaction
	stdSignature, err := auth.MakeSignature(c.Keybase, c.Key, "", sign)

	// Create the StdTx for broadcast
	stdTx := auth.NewStdTx(datagram, sign.Fee, []auth.StdSignature{stdSignature}, c.Memo)

	// Marshal amino
	out, err := c.Cdc.MarshalBinaryLengthPrefixed(stdTx)
	if err != nil {
		return err
	}

	// Broadcast transaction
	res, err := c.Client.BroadcastTxCommit(out)
	if err != nil {
		return err
	}

	// TODO: Figure out what to do with the response
	fmt.Println(res)
	return nil
}

// NewLiteDB returns a new instance of the liteclient database connection
// CONTRACT: must close the database connection when done with it (defer db.Close())
func (c *Chain) NewLiteDB() (*dbm.GoLevelDB, error) {
	return dbm.NewGoLevelDB(c.ChainID, path.Join(c.dir, "db"))
}

// HELP WANTED!!!
// NOTE: Below this line everything is stubbed out

// GetLatestHeight queries the chain for the latest height and returns it
func (c *Chain) GetLatestHeight() (int64, error) {
	res, err := c.Client.Status()
	if err != nil {
		return -1, err
	}

	if res.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node %s running chain %s not caught up", c.RPCAddr, c.ChainID)
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

// GetHeaderAtHeight returns the header at a given height
func (c *Chain) GetHeaderAtHeight(height int64) (*tmclient.Header, error) {
	if height <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", height)
	}

	res, err := c.Client.Commit(&height)
	if err != nil {
		return nil, err
	}

	header := &tmclient.Header{
		// NOTE: This is not a SignedHeader
		// We are missing a lite.Commit type here
		SignedHeader: res.SignedHeader,
		ValidatorSet: &tmtypes.ValidatorSet{},
	}

	return header, nil
}

// GetLatestHeader returns the latest header from the chain
func (c *Chain) GetLatestHeader() (*tmclient.Header, error) {
	h, err := c.GetLatestHeight()
	if err != nil {
		return nil, err
	}

	out, err := c.GetHeaderAtHeight(h)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (c *Chain) QueryConsensusState(height int64) (*tmclient.ConsensusState, error) {
	if height == 0 {
		h, err := c.GetLatestHeight()
		if err != nil {
			return nil, err
		}
		height = h
	}

	commit, err := c.Client.Commit(&height)
	if err != nil {
		return nil, err
	}

	validators, err := c.Client.Validators(&height, 0, 10000)
	if err != nil {
		return nil, err
	}

	state := &tendermint.ConsensusState{
		Root:             commitment.NewRoot(commit.AppHash),
		ValidatorSetHash: tmtypes.NewValidatorSet(validators.Validators).Hash(),
	}

	return state, nil
}

func (c *Chain) GetConsensusState() clientTypes.ConsensusStateResponse {
	return clientTypes.ConsensusStateResponse{}
}

// GetConnectionsUsingClient gets any connections that exist between chain and counterparty
func (c *Chain) GetConnectionsUsingClient(counterparty *Chain) []connTypes.ConnectionEnd {
	return []connTypes.ConnectionEnd{}
}

// GetConnection returns the remote end of a given connection
func (c *Chain) GetConnection(connectionID string) connTypes.ConnectionEnd {
	return connTypes.ConnectionEnd{}
}

// GetChannelsUsingConnections returns all channels associated with a given set of connections
func (c *Chain) GetChannelsUsingConnections(connections []connTypes.ConnectionEnd) []chanTypes.Channel {
	return []chanTypes.Channel{}
}

// GetChannel returns the channel associated with a channelID
func (c *Chain) GetChannel(channelID string) chanTypes.Channel {
	return chanTypes.Channel{}
}

// QueryTxs returns an array of transactions given a tag
func (c *Chain) QueryTxs(height uint64, tag string) []auth.StdTx {
	return []auth.StdTx{}
}
