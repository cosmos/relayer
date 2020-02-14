module github.com/cosmos/relayer

go 1.13

require (
	github.com/cosmos/cosmos-sdk v0.38.1
	github.com/cosmos/go-bip39 v0.0.0-20180819234021-555e2067c45d
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.6.2
	github.com/tendermint/tendermint v0.33.0
	github.com/tendermint/tm-db v0.4.0
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4

replace github.com/tendermint/tendermint => github.com/tendermint/tendermint v0.33.1-dev0

replace github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.34.4-0.20200211210730-55d1aeaaa428
