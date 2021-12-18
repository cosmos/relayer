package test

import (
	"fmt"
	"github.com/cosmos/relayer/relayer/provider/cosmos"
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
	gaiaProviderCfg = cosmos.CosmosProviderConfig{
		Key:            "testkey",
		ChainID:        "",
		RPCAddr:        "",
		AccountPrefix:  "cosmos",
		GasAdjustment:  1.3,
		GasPrices:      "0.00samoleans",
		TrustingPeriod: "330h",
		Timeout:        "10s",
	}
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

	akashProviderCfg = cosmos.CosmosProviderConfig{
		Key:            "testkey",
		ChainID:        "",
		RPCAddr:        "",
		AccountPrefix:  "akash",
		GasAdjustment:  1.3,
		GasPrices:      "0.00samoleans",
		TrustingPeriod: "330h",
		Timeout:        "10s",
	}
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

	osmosisProviderCfg = cosmos.CosmosProviderConfig{
		Key:            "testkey",
		ChainID:        "",
		RPCAddr:        "",
		AccountPrefix:  "osmo",
		GasAdjustment:  1.3,
		GasPrices:      "0.00samoleans",
		TrustingPeriod: "330h",
		Timeout:        "10s",
	}
	osmosisTestConfig = testChainConfig{
		dockerfile:     "docker/osmosis/Dockerfile",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "osmo",
		trustingPeriod: "330h",
		buildArgs: []dc.BuildArg{
			{Name: "VERSION", Value: "v4.2.0"},
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
		pcfg    cosmos.CosmosProviderConfig
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

	tc.pcfg.Key = "testkey-" + port
	tc.pcfg.RPCAddr = fmt.Sprintf("http://localhost:%s", port)
	tc.pcfg.ChainID = tc.chainID
	prov, err := tc.pcfg.NewProvider("", true)
	require.NoError(t, err)

	return &relayer.Chain{
		Chainid:       tc.chainID,
		RPCAddr:       fmt.Sprintf("http://localhost:%s", port),
		ChainProvider: prov,
	}
}
