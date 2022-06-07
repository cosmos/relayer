package test

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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
		Key:            "",
		ChainName:      "",
		ChainID:        "",
		RPCAddr:        "",
		AccountPrefix:  "cosmos",
		KeyringBackend: "test",
		GasAdjustment:  1.3,
		GasPrices:      "0.00samoleans",
		Debug:          true,
		Timeout:        "10s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
	}
	gaiaTestConfig = testChainConfig{
		dockerfile: "docker/gaiad/Dockerfile",
		rpcPort:    "26657",
		buildArgs: []dc.BuildArg{
			{Name: "VERSION", Value: "v7.0.1"},
		},
	}

	akashProviderCfg = cosmos.CosmosProviderConfig{
		Key:            "",
		ChainName:      "",
		ChainID:        "",
		RPCAddr:        "",
		AccountPrefix:  "akash",
		KeyringBackend: "test",
		GasAdjustment:  1.3,
		GasPrices:      "0.00samoleans",
		Debug:          true,
		Timeout:        "10s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
	}
	akashTestConfig = testChainConfig{
		dockerfile: "docker/akash/Dockerfile",
		rpcPort:    "26657",
		buildArgs: []dc.BuildArg{
			{Name: "VERSION", Value: "v0.16.3"},
		},
	}

	osmosisProviderCfg = cosmos.CosmosProviderConfig{
		Key:            "",
		ChainName:      "",
		ChainID:        "",
		RPCAddr:        "",
		AccountPrefix:  "osmo",
		KeyringBackend: "test",
		GasAdjustment:  1.3,
		GasPrices:      "0.00samoleans",
		Debug:          true,
		Timeout:        "10s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
	}
	osmosisTestConfig = testChainConfig{
		dockerfile: "docker/osmosis/Dockerfile",
		rpcPort:    "26657",
		buildArgs: []dc.BuildArg{
			{Name: "VERSION", Value: "v8.0.0"},
		},
	}

	seeds = []string{SEED1, SEED2}
)

type (
	// testChain represents the different configuration options for spinning up a test
	// cosmos-sdk based blockchain
	testChain struct {
		chainName string
		chainID   string
		seed      int
		t         testChainConfig
		pcfg      provider.ProviderConfig
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

	switch tc.pcfg.(type) {
	case cosmos.CosmosProviderConfig:
		cosmosCfg, _ := tc.pcfg.(cosmos.CosmosProviderConfig)

		cosmosCfg.Key = "testkey-" + port
		cosmosCfg.RPCAddr = fmt.Sprintf("http://localhost:%s", port)
		cosmosCfg.ChainID = tc.chainID

		tc.pcfg = cosmosCfg
		// TODO add case for substrate provider configType here
	default:
		panic(fmt.Errorf("no case for type %T when trying to edit ProviderConfig", tc.pcfg))
	}

	prov, err := tc.pcfg.NewProvider(zaptest.NewLogger(t), "/tmp", true, tc.chainName)
	require.NoError(t, err)

	var debug bool
	// add extra logging if TEST_DEBUG=true
	if val, ok := os.LookupEnv("TEST_DEBUG"); ok {
		debug, err = strconv.ParseBool(val)
		if err != nil {
			debug = false
		}
	}

	c := relayer.NewChain(zaptest.NewLogger(t), prov, debug)
	c.Chainid = tc.chainID
	c.RPCAddr = fmt.Sprintf("http://localhost:%s", port)
	return c
}
