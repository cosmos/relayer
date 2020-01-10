# Relayer

The `relayer` package contains some basic relayer implemenations that are meant to be used by users wishing to relay packets between IBC enalbed chains. It is also well documented and intended as a place where users who are interested in building their own relayer can come for working examples.

### Relayer Home Folder Layout 

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
}
```

### Notes:
- Relayer can manage keys with the test keybase backend currently. It should be easy to switch this out for another keybase in the future. Have a meeting with @alessio to talk about details here.
    * When running (`relayer start`) Relayer needs to be able to use the keys each `Chain` configured in the `~/.relayer/config/config.yaml`
    * Relayers can be kept off the open internet as there is no need to send them messages, this should help improve the scurity of the relayer even though it must keep "hot" keys.
- Relayer should have ability to do CRUD on each of the lite clients it manages (e.g. `relayer lite update <chain-id>`). The groundwork is laid for this, just needs to be hooked up a bit more. 
    * Q: Should we have commands to manage the lite clients independent of the `relay start` command?
- Relayer should have ability to list the chains/connection it is managing (`relayer chains list`)
    * TODO
- Relayers should gracefully handle `ErrInsufficentFunds` when/if accounts run out of funds
    * _Stretch_: Relayer should notify a configurable endpoint when it hits `ErrInsufficentFunds`
- Many of the messages in the `relayer/strategies.go` file are not implemented. This work needs to be completed accoriding to the relayer spec.
- The packet transfer section of the `naive` algo is currently worse than a stub and will need to wait on the completion of ADR 15.

### Open Questions
- Do we want to force users to name their `ibc.Client`s, `ibc.Connection`s, `ibc.Channel`s and `ibc.Port`s? Can we use randomly generated identifiers instead?
