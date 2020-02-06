package relayer

import (
	"fmt"
	"path"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/ibc"
	"github.com/tendermint/tendermint/libs/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

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

	tp, err := time.ParseDuration(trustingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration (%s) for chain %s", trustingPeriod, chainID)
	}

	cdc := codec.New()
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	codec.RegisterEvidences(cdc)
	authvesting.RegisterCodec(cdc)
	ibc.AppModuleBasic{}.RegisterCodec(cdc)
	cdc.Seal()

	return &Chain{
		Key: key, ChainID: chainID, RPCAddr: rpcAddr, AccountPrefix: accPrefix, Counterparties: counterparties, Gas: gas,
		GasAdjustment: gasAdj, GasPrices: gasPrices, DefaultDenom: defaultDenom, Memo: memo, Keybase: keybase,
		Client: client, Cdc: cdc, TrustingPeriod: tp, ChainDir: dir}, nil
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
	TrustingPeriod time.Duration  `yaml:"trusting-period"`

	Keybase  keys.Keybase
	Client   *rpcclient.HTTP
	Cdc      *codec.Codec
	ChainDir string
	logger   log.Logger
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
			return chain, nil
		}
	}
	return &Chain{}, fmt.Errorf("chain with ID %s is not configured", chainID)
}

// KeysDir returns the path to the keys for this chain
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// GetAddress returns the sdk.AccAddress associated with the configred key
func (c *Chain) GetAddress() (sdk.AccAddress, error) {
	// Signing key for src chain
	srcAddr, err := c.Keybase.Get(c.Key)
	if err != nil {
		return nil, err
	}
	return srcAddr.GetAddress(), nil
}
