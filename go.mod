module github.com/iqlusioninc/relayer

go 1.14

require (
	github.com/DataDog/datadog-go v3.7.1+incompatible
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c // indirect
	github.com/cosmos/cosmos-sdk v0.34.4-0.20200825201020-d9fd4d2ca9a3
	github.com/cosmos/gaia v0.0.1-0.20200522222820-2d61264338e5
	github.com/cosmos/go-bip39 v0.0.0-20180819234021-555e2067c45d
	github.com/gorilla/mux v1.8.0
	github.com/ory/dockertest/v3 v3.5.5
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/tendermint/tendermint v0.34.0-rc3
	github.com/tendermint/tm-db v0.6.1
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4
replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4

replace github.com/cosmos/cosmos-sdk => /Users/johnzampolin/go/src/github.com/cosmos/cosmos-sdk
replace github.com/cosmos/gaia => /Users/johnzampolin/go/src/github.com/cosmos/gaia