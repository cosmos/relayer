package substrate

import (
	"io"

	"github.com/ChainSafe/gossamer/lib/keystore"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	"go.uber.org/zap"
)

type SubstrateProviderConfig struct {
	Key            string  `json:"key" yaml:"key"`
	ChainName      string  `json:"-" yaml:"-"`
	ChainID        string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr        string  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix  string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string  `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory   string  `json:"key-directory" yaml:"key-directory"`
	GasAdjustment  float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices      string  `json:"gas-prices" yaml:"gas-prices"`
	Debug          bool    `json:"debug" yaml:"debug"`
	Timeout        string  `json:"timeout" yaml:"timeout"`
	OutputFormat   string  `json:"output-format" yaml:"output-format"`
	SignModeStr    string  `json:"sign-mode" yaml:"sign-mode"`
}

type SubstrateProvider struct {
	log     *zap.Logger
	Config  *SubstrateProviderConfig
	Keybase keystore.Keyring
	Input   io.Reader

	RPCClient           *rpcclient.SubstrateAPI
	RelayChainRPCClient *rpcclient.SubstrateAPI
}
