package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/stretchr/testify/require"

	ry "github.com/ovrclk/relayer/relayer"
)

var (
	// GAIA BLOCK TIMEOUTS on jackzampolin/gaiatest:master
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	gaiaTestConfig = testChainConfig{
		dockerImage:    "jackzampolin/gaiatest",
		dockerTag:      "gaiav3.0",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		gas:            200000,
		gasPrices:      "0.025stake",
		defaultDenom:   "stake",
		trustingPeriod: "330h",
	}

	// MTD BLOCK TIMEOUTS on microtick/mtzonetest:ibc-alpha
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	mtdTestConfig = testChainConfig{
		cdc:            cdc,
		amino:          amino,
		dockerImage:    "microtick/mtzonetest",
		dockerTag:      "ibc-alpha",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		gas:            200000,
		gasPrices:      "0.025stake",
		defaultDenom:   "stake",
		trustingPeriod: "330h",
	}

	// RocketZone
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	rocketTestConfig = testChainConfig{
		cdc:            cdc,
		amino:          amino,
		dockerImage:    "rocketprotocol/rocketzone-relayer-test",
		dockerTag:      "latest",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		gas:            200000,
		gasPrices:      "0.025ufuel",
		defaultDenom:   "ufuel",
		trustingPeriod: "330h",
	}

	// Agoric Chain
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	agoricTestConfig = testChainConfig{
		cdc:            cdc,
		amino:          amino,
		dockerImage:    "agoric/agoric-sdk",
		dockerTag:      "ibc-alpha",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "agoric",
		gas:            200000,
		gasPrices:      "",
		defaultDenom:   "uag",
		trustingPeriod: "330h",
	}

	// CoCo Chain  saisunkari19/coco:ibc-alpha
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	cocoTestConfig = testChainConfig{
		cdc:            cdc,
		amino:          amino,
		dockerImage:    "saisunkari19/coco",
		dockerTag:      "ibc-alpha",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmic",
		gas:            200000,
		gasPrices:      "0.025coco",
		defaultDenom:   "coco",
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
		cdc            *std.Codec
		amino          *codec.Codec
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
func newTestChain(t *testing.T, tc testChain) *ry.Chain {
	_, port, err := server.FreeTCPAddr()
	require.NoError(t, err)

	return &ry.Chain{
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
