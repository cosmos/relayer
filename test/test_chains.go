package test

import (
	"fmt"
	"testing"
	"time"

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/go-amino"

	. "github.com/iqlusioninc/relayer/relayer"
)

var (
	// GAIA BLOCK TIMEOUTS on jackzampolin/gaiatest:jack_relayer-testing
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	gaiaTestConfig = testChainConfig{
		cdc:            codecstd.NewAppCodec(codecstd.MakeCodec(simapp.ModuleBasics)),
		amino:          codecstd.MakeCodec(simapp.ModuleBasics),
		dockerImage:    "jackzampolin/gaiatest",
		dockerTag:      "ibc-alpha",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		gas:            200000,
		gasPrices:      "0.025stake",
		defaultDenom:   "stake",
		trustingPeriod: "330h",
	}
)

type (
	// testChain represents the different configuration options for spinning up a test
	// cosmos-sdk based blockchain
	testChain struct {
		chainID string
		t       testChainConfig
	}

	// testChainConfig represents the chain specific docker and codec configurations
	// required.
	testChainConfig struct {
		dockerImage    string
		dockerTag      string
		cdc            *codecstd.Codec
		amino          *amino.Codec
		rpcPort        string
		timeout        time.Duration
		accountPrefix  string
		gas            uint64
		gasPrices      string
		defaultDenom   string
		trustingPeriod string
	}
)

// newTestChain generates a new instance of *Chain with a free TCP port configured as the RPC port
func newTestChain(t *testing.T, tc testChain) *Chain {
	_, port, err := server.FreeTCPAddr()
	require.NoError(t, err)
	return &Chain{
		Key:            "testkey",
		ChainID:        tc.chainID,
		RPCAddr:        fmt.Sprintf("http://localhost:%s", port),
		AccountPrefix:  tc.t.accountPrefix,
		Gas:            tc.t.gas,
		GasPrices:      tc.t.gasPrices,
		DefaultDenom:   tc.t.defaultDenom,
		TrustingPeriod: tc.t.trustingPeriod,
	}
}
