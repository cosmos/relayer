package relayer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/simapp/params"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	"github.com/cosmos/relayer/relayer/provider"
	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
)

// Chain represents the necessary data for connecting to and identifying a chain and its counterparties
// TODO revise Chain struct
type Chain struct {
	ChainProvider provider.ChainProvider
	Chainid       string `yaml:"chain-id" json:"chain-id"`
	RPCAddr       string `yaml:"rpc-addr" json:"rpc-addr"`

	PathEnd  *PathEnd              `yaml:"-" json:"-"`
	Encoding params.EncodingConfig `yaml:"-" json:"-"`

	logger log.Logger
	debug  bool
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

// Init initializes the pieces of a chain that aren't set when it parses a config
// NOTE: All validation of the chain should happen here.
// TODO chain init needs revised to account for Provider abstraction
func (c *Chain) Init(logger log.Logger, debug bool) {
	c.logger = logger
	c.debug = debug
	if c.logger == nil {
		c.logger = defaultChainLogger()
	}
}

func defaultChainLogger() log.Logger {
	return log.NewTMLogger(log.NewSyncWriter(os.Stdout))
}

func (c *Chain) ChainID() string {
	return c.ChainProvider.ChainId()
}

func (c *Chain) ChannelID() string {
	return c.PathEnd.ChannelID
}

func (c *Chain) ConnectionID() string {
	return c.PathEnd.ConnectionID
}

func (c *Chain) ClientID() string {
	return c.PathEnd.ClientID
}

func (c *Chain) PortID() string {
	return c.PathEnd.PortID
}

func (c *Chain) Version() string {
	return c.PathEnd.Version
}

func (c *Chain) Order() string {
	return c.PathEnd.Order
}

// GetSelfVersion returns the version of the given chain
func (c *Chain) GetSelfVersion() uint64 {
	return clienttypes.ParseChainID(c.ChainID())
}

// GetTrustingPeriod returns the trusting period for the chain
func (c *Chain) GetTrustingPeriod() (time.Duration, error) {
	trustStr, err := c.ChainProvider.TrustingPeriod()
	if err != nil {
		return 0, err
	}
	tp, _ := time.ParseDuration(trustStr)
	return tp, nil
}

// Log takes a string and logs the data
func (c *Chain) Log(s string) {
	c.logger.Info(s)
}

// Error takes an error, wraps it in the chainID and logs the error
func (c *Chain) Error(err error) {
	c.logger.Error(fmt.Sprintf("%s: err(%s)", c.ChainID(), err.Error()))
}

func (c *Chain) String() string {
	out, _ := json.Marshal(c)
	return string(out)
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

//// SendAndPrint sends a transaction and prints according to the passed args
//func (c *Chain) SendAndPrint(txs []sdk.Msg, text, indent bool) (err error) {
//	if c.debug {
//		for _, msg := range txs {
//			if err = c.Print(msg, text, indent); err != nil {
//				return err
//			}
//		}
//	}
//	// SendAndPrint sends the transaction with printing options from the CLI
//	res, _, err := c.SendMsgs(txs)
//	if err != nil {
//		return err
//	}
//
//	return c.Print(res, text, indent)
//
//}

// Chains is a collection of Chain
type Chains []*Chain

// Get returns the configuration for a given chain
func (c Chains) Get(chainID string) (*Chain, error) {
	for _, chain := range c {
		if chainID == chain.ChainProvider.ChainId() {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("chain with ID %s is not configured", chainID)
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
// TODO this needs to be exposed on the ChainProvider or rpc-addr needs to be persisted on the Chain struct still
func (c *Chain) GetRPCPort() string {
	u, _ := url.Parse(c.RPCAddr)
	return u.Port()
}

// CreateTestKey creates a key for test chain
func (c *Chain) CreateTestKey() error {
	if c.ChainProvider.KeyExists(c.ChainProvider.Key()) {
		return fmt.Errorf("key %s exists for chain %s", c.ChainID(), c.ChainProvider.Key())
	}
	_, err := c.ChainProvider.AddKey(c.ChainProvider.Key())
	return err
}

// GetTimeout returns the chain's configured timeout
func (c *Chain) GetTimeout() (time.Duration, error) {
	timeout, err := time.ParseDuration(c.ChainProvider.Timeout())
	if err != nil {
		return 0, fmt.Errorf("failed to parse timeout (%s) for chain %s. Err: %w", c.ChainProvider.Timeout(), c.ChainID(), err)
	}
	return timeout, nil
}

//// UpgradeChain submits and upgrade proposal using a zero'd out client state with an updated unbonding period.
//func (c *Chain) UpgradeChain(dst *Chain, plan *upgradetypes.Plan, deposit sdk.Coin,
//	unbondingPeriod time.Duration) error {
//	height, err := dst.ChainProvider.QueryLatestHeight()
//	if err != nil {
//		return err
//	}
//
//	clientState, err := dst.ChainProvider.QueryClientState(height, c.PathEnd.ClientID)
//	if err != nil {
//		return err
//	}
//
//	upgradedClientState := clientState.ZeroCustomFields().(*ibctmtypes.ClientState)
//	upgradedClientState.LatestHeight.RevisionHeight = uint64(plan.Height + 1)
//	upgradedClientState.UnbondingPeriod = unbondingPeriod
//
//	// TODO: make cli args for title and description
//	upgradeProposal, err := clienttypes.NewUpgradeProposal("upgrade",
//		"upgrade the chain's software and unbonding period", *plan, upgradedClientState)
//	if err != nil {
//		return err
//	}
//
//	addr, err := c.ChainProvider.ShowAddress(c.ChainProvider.Key())
//	if err != nil {
//		return err
//	}
//
//	msg, err := govtypes.NewMsgSubmitProposal(upgradeProposal, sdk.NewCoins(deposit), addr)
//	if err != nil {
//		return err
//	}
//
//	_, _, err = c.ChainProvider.SendMessage(msg)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
