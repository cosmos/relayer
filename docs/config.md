### Relayer Home Folder Layout 

The following is the folder structure for the relayer `--home` directory

```bash
~/.relayer
├── config
├── keys
│   ├── testchain1
│   │   └── keyring-test
│   └── testchain2
│       └── keyring-test
└── lite
    ├── testchain1
    │   └── trust-base.db
    └── testchain1
        └── trust-base.db
```

### Configuring the Relayer

There are two major parts of `relayer` configuration:

```go
type Config struct {
	Global GlobalConfig  `yaml:"global"`
	Chains []ChainConfig `yaml:"chains"`

    // NOTE: Chain is type from the relayer package where functionality
    // is implemented. The ChainConfig type is just for parsing the config
    // The ChainConfig(s) are then passed to the relayer.NewChain constructor
    // and stored in the c property
	c []*relayer.Chain
}
```

#### Global Configuration

- Amount of time to sleep between relayer loops
- Which strategy to use for your relayer (`naieve` is the only planned for implemenation)
- Number of block headers to cache for the lite client

```go
// NOTE: are there any other items that could be useful here?
type Global struct {
	Strategy      string `yaml:"strategy"`
	Timeout       string `yaml:"timeout"`
	LiteCacheSize int    `yaml:"lite-cache-size"`
}
```

#### Chains config

The `ConfigChain` abstraction contains all the necessary data to connect to a given chain, query it's state, and send transactions to it. The config will contain an array of these chains (`[]ChainConfig`). These `ChainConfig` instances will then be converted into the `relayer.Chain` abstration to perform all the necessary tasks. The following data will be needed by each `relayer.Chain` and is passed in via `ChainConfig`s:

```go
type ChainConfig struct {
	Key            string               `yaml:"key"`
	ChainID        string               `yaml:"chain-id"`
	RPCAddr        string               `yaml:"rpc-addr"`
	AccountPrefix  string               `yaml:"account-prefix"`
	Counterparties []CounterpartyConfig `yaml:"counterparties"`
	Gas            uint64               `yaml:"gas,omitempty"`
	GasAdjustment  float64              `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins         `yaml:"gas-prices,omitempty"`
	DefaultDenom   string               `yaml:"default-denom,omitempty"`
	Memo           string               `yaml:"memo,omitempty"`
	TrustOptions   relayer.TrustOptions `yaml:"trust-options"`
}
```

#### Counterparty config

The `CounterPartyConfig` struct allows you to specify the `chain-id`(s) and `client-id`(s) that the relayer will 1. setup/repair `Connection`s across, 2. setup/repair `Channel`s across, and 3. relay `Packet`s across:

```go
type CounterpartyConfig struct {
	ChainID  string `yaml:"chain-id"`
	ClientID string `yaml:"client-id"`
}
```

### TrustOptions

TrustOptions allow you to define a `Height` and `Hash` to be a root of trust. If they are not given then the configured RPC endpoint for the chain will fetch the latest header 
