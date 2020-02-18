package relayer

import (
	"fmt"
	"path"
	"time"

	"github.com/cosmos/cosmos-sdk/client/context"
	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/tendermint/tendermint/libs/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

// NewChain returns a new instance of Chain
// NOTE: It does not by default create the verifier. This needs a working connection
// and blocks running the app if NewChain does this by default.
func NewChain(key, chainID, rpcAddr, accPrefix string, gas uint64, gasAdj float64,
	gasPrices, defaultDenom, memo, homePath string, liteCacheSize int, trustingPeriod,
	dir string, cdc *codec.Codec) (*Chain, error) {
	keybase, err := keys.NewKeyring(chainID, "test", keysDir(homePath), nil)
	if err != nil {
		return &Chain{}, err
	}

	client, err := rpcclient.NewHTTP(rpcAddr, "/websocket")
	if err != nil {
		return &Chain{}, err
	}

	gp, err := sdk.ParseDecCoins(gasPrices)
	if err != nil {
		return &Chain{}, err
	}

	tp, err := time.ParseDuration(trustingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration (%s) for chain %s", trustingPeriod, chainID)
	}

	return &Chain{
		Key: key, ChainID: chainID, RPCAddr: rpcAddr, AccountPrefix: accPrefix, Gas: gas,
		GasAdjustment: gasAdj, GasPrices: gp, DefaultDenom: defaultDenom, Memo: memo, Keybase: keybase,
		Client: client, Cdc: cdc, TrustingPeriod: tp, HomePath: homePath}, nil
}

// Chain represents the necessary data for connecting to and indentifying a chain and its counterparites
type Chain struct {
	Key            string        `yaml:"key"`
	ChainID        string        `yaml:"chain-id"`
	RPCAddr        string        `yaml:"rpc-addr"`
	AccountPrefix  string        `yaml:"account-prefix"`
	Gas            uint64        `yaml:"gas,omitempty"`
	GasAdjustment  float64       `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins  `yaml:"gas-prices,omitempty"`
	DefaultDenom   string        `yaml:"default-denom,omitempty"`
	Memo           string        `yaml:"memo,omitempty"`
	TrustingPeriod time.Duration `yaml:"trusting-period"`
	HomePath       string
	PathEnd        *PathEnd

	Keybase keys.Keybase
	Client  *rpcclient.HTTP
	Cdc     *codec.Codec

	address sdk.AccAddress
	logger  log.Logger
}

// Chains is a collection of Chain
type Chains []*Chain

// Exists Returns true if the chain is configured
func (c Chains) Exists(chainID string) bool {
	for _, chain := range c {
		if chainID == chain.ChainID {
			return true
		}
	}
	return false
}

// GetChain returns the configuration for a given chain
func (c Chains) GetChain(chainID string) (*Chain, error) {
	for _, chain := range c {
		if chainID == chain.ChainID {
			addr, _ := chain.GetAddress()
			chain.address = addr
			return chain, nil
		}
	}
	return &Chain{}, fmt.Errorf("chain with ID %s is not configured", chainID)
}

// GetChains returns a map chainIDs to their chains
func (c Chains) GetChains(chainIDs ...string) (map[string]*Chain, error) {
	out := make(map[string]*Chain)
	for _, cid := range chainIDs {
		chain, err := c.GetChain(cid)
		if err != nil {
			return out, err
		}
		out[cid] = chain
	}
	return out, nil
}

func (c *Chain) BuildAndSignTx(datagram []sdk.Msg) ([]byte, error) {
	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(c).GetAccount(c.MustGetAddress())
	if err != nil {
		return nil, err
	}

	return auth.NewTxBuilder(
		auth.DefaultTxEncoder(c.Cdc), acc.GetAccountNumber(),
		acc.GetSequence(), c.Gas, c.GasAdjustment, false, c.ChainID,
		c.Memo, sdk.NewCoins(), c.GasPrices).WithKeybase(c.Keybase).
		BuildAndSign(c.Key, ckeys.DefaultKeyPass, datagram)
}

// BroadcastTxCommit takes the marshaled transaction bytes and broadcasts them
func (c *Chain) BroadcastTxCommit(txBytes []byte) (sdk.TxResponse, error) {
	return context.CLIContext{Client: c.Client}.BroadcastTxCommit(txBytes)
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

type Paths []Path

// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src PathEnd `yaml:"src" json:"src"`
	Dst PathEnd `yaml:"dst" json:"dst"`
}

func (p Path) String() string {
	return fmt.Sprintf("%s -> %s", p.Src.String(), p.Dst.String())
}

// PathEnd represents the local connection identifers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
}

func (p PathEnd) String() string {
	return fmt.Sprintf("client{%s}-conn{%s}-chan{%s}@chain{%s}:port{%s}", p.ClientID, p.ConnectionID, p.ChannelID, p.ChainID, p.PortID)
}

func (p *PathEnd) Validate() error {
	// TODO: use this validate function to perform all
	// ICS24 identifier validation, something like ->
	// if err := ics24.ValidateIdentifier(p.ClientID); p.ClientID != nil && err != nil {
	// 	return err
	// }
	return nil
}

// SetNewPathClient used to set the path for client creation commands
func (c *Chain) SetNewPathClient(clientID string) error {
	return c.setPath(&PathEnd{
		ChainID:  c.ChainID,
		ClientID: clientID,
	})
}

// SetNewPathConnection used to set the path for the connection creation commands
func (c *Chain) SetNewPathConnection(clientID, connectionID string) error {
	return c.setPath(&PathEnd{
		ChainID:      c.ChainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
	})
}

// SetFullPath sets all of the properties on the path
func (c *Chain) SetNewFullPath(clientID, connectionID, channelID, portID string) error {
	return c.setPath(&PathEnd{
		ChainID:      c.ChainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
		ChannelID:    channelID,
		PortID:       portID,
	})
}

// PathSet check if the chain has a path set
func (c *Chain) PathSet() bool {
	if c.PathEnd == nil {
		return false
	}
	return true
}

// PathsSet checks if the chains have their paths set
func PathsSet(chains ...*Chain) bool {
	for _, c := range chains {
		if !c.PathSet() {
			return false
		}
	}
	return true
}

func (c *Chain) setPath(p *PathEnd) error {
	err := p.Validate()
	if err != nil {
		return err
	}
	c.PathEnd = p
	return nil
}
