package relayer

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/tendermint/tendermint/libs/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	auth "github.com/cosmos/cosmos-sdk/x/auth"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmlite "github.com/tendermint/tendermint/lite"
	tmliteproxy "github.com/tendermint/tendermint/lite/proxy"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
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
func NewChain(key, chainID, rpcAddr, accPrefix string,
	counterparties []string, gas uint64, gasAdj float64,
	gasPrices sdk.DecCoins, defaultDenom, memo, homePath string,
	liteCacheSize int,
) (*Chain, error) {
	keybase, err := keys.NewTestKeyring(chainID, keysDir(homePath, chainID))
	if err != nil {
		return &Chain{}, err
	}

	client := rpcclient.NewHTTP(rpcAddr, "/websocket")

	if err != nil {
		return &Chain{}, err
	}

	return &Chain{
		Key: key, ChainID: chainID, RPCAddr: rpcAddr, AccountPrefix: accPrefix, Counterparties: counterparties, Gas: gas, GasAdjustment: gasAdj, GasPrices: gasPrices, DefaultDenom: defaultDenom, Memo: memo, Keybase: keybase, Client: client}, nil
}

// Chain represents the necessary data for connecting to and indentifying a chain and its counterparites
type Chain struct {
	Key            string       `yaml:"key"`
	ChainID        string       `yaml:"chain-id"`
	RPCAddr        string       `yaml:"rpc-addr"`
	AccountPrefix  string       `yaml:"account-prefix"`
	Counterparties []string     `yaml:"counterparties"`
	Gas            uint64       `yaml:"gas,omitempty"`
	GasAdjustment  float64      `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins `yaml:"gas-prices,omitempty"`
	DefaultDenom   string       `yaml:"default-denom,omitempty"`
	Memo           string       `yaml:"memo,omitempty"`

	Keybase keys.Keybase
	Client  *rpcclient.HTTP
}

// Verifier returns the lite client verifier for the Chain
func (c *Chain) Verifier(homePath string, liteCacheSize int) (tmlite.Verifier, error) {
	verifier, err := tmliteproxy.NewVerifier(
		c.ChainID, liteDir(homePath, c.ChainID),
		c.Client, log.NewNopLogger(), liteCacheSize,
	)
	if err != nil {
		return nil, err
	}
	return verifier, nil
}

// LiteDir returns the proper directory for the lite client for a given chain
func liteDir(home, chainID string) string {
	return filepath.Join(home, "lite", chainID)
}

// KeysDir returns the path to the keys for this chain
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// SendMsgs sends the standard transactions to the individual chain
func (c *Chain) SendMsgs(datagram []sdk.Msg) error {
	// TODO: reuse CLI code and the data from Chain to send the signed transactions to the individual chains
	// TODO: figure out key management for the relayer
	return nil
}

// LatestHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) LatestHeight() uint64 {
	return 0
}

// LatestHeader returns the header to be used for client creation
func (c *Chain) LatestHeader() clientExported.Header {
	return nil
}

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain
func (c *Chain) QueryConsensusState() clientTypes.ConsensusStateResponse {
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
