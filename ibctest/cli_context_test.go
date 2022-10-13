package ibctest_test

import (
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

func CliContext(chain ibc.Chain) client.Context {
	const secondsTimeout uint = 30
	c, err := rpchttp.NewWithTimeout(chain.GetHostRPCAddress(), "/websocket", secondsTimeout)
	if err != nil {
		panic(err)
	}

	cfg := chain.Config()
	return client.Context{
		Client:            c,
		ChainID:           cfg.ChainID,
		InterfaceRegistry: cfg.EncodingConfig.InterfaceRegistry,
		Input:             os.Stdin,
		Output:            os.Stdout,
		OutputFormat:      "json",
		LegacyAmino:       cfg.EncodingConfig.Amino,
		TxConfig:          cfg.EncodingConfig.TxConfig,
	}
}
