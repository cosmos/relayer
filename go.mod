module github.com/iqlusioninc/relayer

go 1.14

require (
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c // indirect
	github.com/cosmos/cosmos-sdk v0.34.4-0.20200423152229-f1fdde5d1b18
	github.com/cosmos/go-bip39 v0.0.0-20180819234021-555e2067c45d
	github.com/gorilla/mux v1.7.4
	github.com/ory/dockertest/v3 v3.5.5
	github.com/sirkon/goproxy v1.4.8
	github.com/sirupsen/logrus v1.5.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.6.3
	github.com/stretchr/testify v1.5.1
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tendermint v0.33.4
	github.com/tendermint/tm-db v0.5.1
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4
