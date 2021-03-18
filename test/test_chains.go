package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/stretchr/testify/require"

	ry "github.com/cosmos/relayer/relayer"
)

var (
	// GAIA BLOCK TIMEOUTS are located in the gaia setup script in the
	// setup directory.
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	gaiaTestConfig = testChainConfig{
		dockerfile:     "./setup/Dockerfile.gaiatest",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		trustingPeriod: "330h",
	}

	// AKASH BLOCK TIMEOUTS on jackzampolin/akashtest:master
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	// This is built from contrib/Dockerfile.test from the akash repository:
	akashTestConfig = testChainConfig{
		dockerfile:     "./setup/Dockerfile.akashtest",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "akash",
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
		dockerfile     string
		rpcPort        string
		timeout        time.Duration
		accountPrefix  string
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
		GasAdjustment:  1.3,
		TrustingPeriod: tc.t.trustingPeriod,
	}
}
