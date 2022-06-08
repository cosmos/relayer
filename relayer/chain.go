package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var (
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)

	defaultCoinType uint32 = 118
)

// Chain represents the necessary data for connecting to and identifying a chain and its counterparties
// TODO revise Chain struct
type Chain struct {
	log *zap.Logger

	ChainProvider provider.ChainProvider
	Chainid       string `yaml:"chain-id" json:"chain-id"`
	RPCAddr       string `yaml:"rpc-addr" json:"rpc-addr"`

	PathEnd *PathEnd `yaml:"-" json:"-"`

	debug bool
}

// Chains is a collection of Chain (mapped by chain_name)
type Chains map[string]*Chain

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

func NewChain(log *zap.Logger, prov provider.ChainProvider, debug bool) *Chain {
	return &Chain{
		log:           log,
		ChainProvider: prov,
		debug:         debug,
	}
}

func (c *Chain) ChainID() string {
	return c.ChainProvider.ChainId()
}

func (c *Chain) ConnectionID() string {
	return c.PathEnd.ConnectionID
}

func (c *Chain) ClientID() string {
	return c.PathEnd.ClientID
}

// GetSelfVersion returns the version of the given chain
func (c *Chain) GetSelfVersion() uint64 {
	return clienttypes.ParseChainID(c.ChainID())
}

// GetTrustingPeriod returns the trusting period for the chain
func (c *Chain) GetTrustingPeriod(ctx context.Context) (time.Duration, error) {
	return c.ChainProvider.TrustingPeriod(ctx)
}

func (c *Chain) String() string {
	out, _ := json.Marshal(c)
	return string(out)
}

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
func (c *Chain) GetRPCPort() string {
	u, _ := url.Parse(c.RPCAddr)
	return u.Port()
}

// CreateTestKey creates a key for test chain
func (c *Chain) CreateTestKey() error {
	if c.ChainProvider.KeyExists(c.ChainProvider.Key()) {
		return fmt.Errorf("key {%s} exists for chain {%s}", c.ChainProvider.Key(), c.ChainID())
	}
	_, err := c.ChainProvider.AddKey(c.ChainProvider.Key(), defaultCoinType)
	return err
}

// GetTimeout returns the chain's configured timeout
func (c *Chain) GetTimeout() (time.Duration, error) {
	timeout, err := time.ParseDuration(c.ChainProvider.Timeout())
	if err != nil {
		return 0, fmt.Errorf("failed to parse timeout (%s) for chain %s: %w", c.ChainProvider.Timeout(), c.ChainID(), err)
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
