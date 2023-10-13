package interchaintest_test

import (
	"context"
	"testing"

	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/conformance"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestScenarioTendermint37Boundary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	nv := 1
	nf := 0

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "gaia",
			ChainName:     "gaia",
			Version:       "v7.0.3",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			// TODO update with mainnet SDK v0.47+ chain version once available
			Name:          "ibc-go-simd",
			ChainName:     "ibc-go-simd",
			Version:       "andrew-47-rc1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	client, network := interchaintest.DockerSetup(t)

	chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	const (
		path        = "ibc-path"
		relayerName = "relayer"
	)

	rf := relayerinterchaintest.NewRelayerFactory(relayerinterchaintest.RelayerConfig{
		InitialBlockHistory: 50,
	})

	r := rf.Build(t, client, network)

	t.Parallel()

	ic := interchaintest.NewInterchain().
		AddChain(chain).
		AddChain(counterpartyChain).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  chain,
			Chain2:  counterpartyChain,
			Relayer: r,
			Path:    path,
		})

	ctx := context.Background()

	rep := testreporter.NewNopReporter()

	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// Uncomment this to load blocks, txs, msgs, and events into sqlite db as test runs
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// test IBC conformance
	conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)
}
