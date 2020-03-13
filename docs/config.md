### Relayer Home Folder Layout 

The following is the folder structure for the relayer `--home` directory

```bash
~/.relayer
├── config
│   └── config.yaml
├── keys
│   ├── keyring-test-ibc0
│   └── keyring-test-ibc1
└── lite
    ├── ibc0.db
    └── ibc1.db
```

### Configuring the Relayer

There are three major parts of `relayer` configuration:

```go
type Config struct {
	Global GlobalConfig    `yaml:"global"`
	Chains []ChainConfig   `yaml:"chains"`
	Paths  []relayer.Paths `yaml:"paths"`
}
```

#### Global Configuration

- Amount of time to sleep between relayer loops
- Which strategy to use for your relayer (`naieve` is the only planned for implemenation)
- Number of block headers to cache for the lite client

> NOTE: Additional global configuration will be added in this section, also items may be removed

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

> NOTE: We need to add the chain unbonding time to this struct and `relayer.Chain` as well: https://github.com/cosmos/relayer/issues/56

```go
// ChainConfig describes the config necessary for an individual chain
type ChainConfig struct {
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
}
```

> NOTE: This may be a redundent struct. A refactor that could be undertaken would be to replace this with the `relayer.Chain` in the config parsing see: https://github.com/cosmos/relayer/issues/31

#### Paths

The `Paths` in the configuration define which `Path`s the relayer will move packets between. It contains all the identifers necessary to connect two chains:

> NOTE: the `Index` field is used when displaying a list to users to allow them to specify which path they desire.

```go
// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src   *PathEnd `yaml:"src" json:"src"`
	Dst   *PathEnd `yaml:"dst" json:"dst"`
	Index int      `yaml:"index,omitempty" json:"index,omitempty"`
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
```

> NOTE: We need to add a `Order` field to this struct: https://github.com/cosmos/relayer/issues/52