module github.com/iqlusioninc/relayer

go 1.14

require (
	github.com/DataDog/datadog-go v3.7.1+incompatible
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c // indirect
	github.com/cosmos/cosmos-sdk v0.34.4-0.20200511222341-80be50319ca5
	github.com/cosmos/gaia v0.0.1-0.20200511233019-cbc33219c3d9
	github.com/cosmos/go-bip39 v0.0.0-20180819234021-555e2067c45d
	github.com/gorilla/mux v1.7.4
	github.com/ory/dockertest/v3 v3.5.5
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/sirupsen/logrus v1.5.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.5.1
	github.com/tendermint/tendermint v0.33.4
	github.com/tendermint/tm-db v0.5.1
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4
