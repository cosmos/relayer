module github.com/cosmos/relayer

go 1.15

require (
	github.com/Microsoft/go-winio v0.4.15 // indirect
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/containerd/continuity v0.0.0-20200928162600-f2cc35102c2a // indirect
	github.com/cosmos/cosmos-sdk v0.43.0
	github.com/cosmos/go-bip39 v1.0.0
	github.com/cosmos/ibc-go v1.0.1
	github.com/gogo/protobuf v1.3.3
	github.com/gorilla/mux v1.8.0
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.9.0 // indirect
	github.com/moby/term v0.0.0-20201101162038-25d840ce174a // indirect
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.4 // indirect
	github.com/ory/dockertest/v3 v3.6.2
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.8.0
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.12
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4
