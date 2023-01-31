package stride

import (
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	rlystride "github.com/cosmos/relayer/v2/relayer/chains/cosmos/stride"
	"github.com/strangelove-ventures/ibctest/v5/chain/cosmos"
)

func Encoding() *simappparams.EncodingConfig {
	cfg := cosmos.DefaultEncoding()

	rlystride.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
