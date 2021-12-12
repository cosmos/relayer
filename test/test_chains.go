package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/relayer/relayer"
	dc "github.com/ory/dockertest/v3/docker"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/stretchr/testify/require"
)

const (
	// SEED1 is a mnenomic
	//nolint:lll
	SEED1 = "cake blossom buzz suspect image view round utility meat muffin humble club model latin similar glow draw useless kiwi snow laugh gossip roof public"
	// SEED2 is a mnemonic
	//nolint:lll
	SEED2 = "near little movie lady moon fuel abandon gasp click element muscle elbow taste indoor soft soccer like occur legend coin near random normal adapt"
)

var (
	// GAIA BLOCK TIMEOUTS are located in the gaia setup script in the
	// setup directory.
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	gaiaTestConfig = testChainConfig{
		dockerfile:     "docker/gaiad/Dockerfile",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		trustingPeriod: "330h",
		buildArgs: []dc.BuildArg{
			{Name: "VERSION", Value: "v5.0.8"},
		},
	}

	// AKASH BLOCK TIMEOUTS on jackzampolin/akashtest:master
	// timeout_commit = "1000ms"
	// timeout_propose = "1000ms"
	// 3 second relayer timeout works well with these block times
	// This is built from contrib/Dockerfile.test from the akash repository:
	akashTestConfig = testChainConfig{
		dockerfile:     "docker/akash/Dockerfile",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "akash",
		trustingPeriod: "330h",
		buildArgs: []dc.BuildArg{
			{Name: "VERSION", Value: "v0.12.1"},
		},
	}

	seeds = []string{SEED1, SEED2}
)

type (
	// testChain represents the different configuration options for spinning up a test
	// cosmos-sdk based blockchain
	testChain struct {
		chainID string
		seed    int
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
		buildArgs      []dc.BuildArg
	}
)

// newTestChain generates a new instance of *Chain with a free TCP port configured as the RPC port
func newTestChain(t *testing.T, tc testChain) *relayer.Chain {
	_, port, err := server.FreeTCPAddr()
	require.NoError(t, err)
	return &relayer.Chain{
		Key:            "testkey",
		ChainID:        tc.chainID,
		RPCAddr:        fmt.Sprintf("http://localhost:%s", port),
		AccountPrefix:  tc.t.accountPrefix,
		GasAdjustment:  1.3,
		TrustingPeriod: tc.t.trustingPeriod,
	}
}
