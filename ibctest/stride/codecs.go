package stride

import (
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	rlystride "github.com/cosmos/relayer/v2/relayer/chains/cosmos/stride"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
)

func Encoding() *testutil.TestEncodingConfig {
	cfg := cosmos.DefaultEncoding()

	rlystride.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
