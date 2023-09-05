package interchaintest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cosmos/relayer/v2/cmd"
	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v7/relayer"
	interchaintestrly "github.com/strangelove-ventures/interchaintest/v7/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestClientOverrideFlag tests that the --override flag is being respected when attempting to create new light clients.
// If the override flag is not present, the relayer should attempt to look for an existing light client if there
// is a client-id present in the relative path config. If the override flag is present, the relayer should always
// attempt to create a new light client and then overwrite the config file if successful.
func TestClientOverrideFlag(t *testing.T) {
	image := relayerinterchaintest.BuildRelayerImage(t)

	client, network := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		interchaintestrelayer.CustomDockerImage(image, "latest", "100:1000"),
		interchaintestrelayer.ImagePull(false),
	).Build(t, client, network)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "gaia",
			ChainName: "gaia",
			Version:   "v7.0.3",
		},
		{
			Name:      "osmosis",
			ChainName: "osmosis",
			Version:   "v11.0.1",
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia, osmosis := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	// Build the network; spin up the chains and configure the relayer
	const pathGaiaOsmosis = "gaia-osmosis"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    pathGaiaOsmosis,
		})

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// Uncomment this to load blocks, txs, msgs, and events into sqlite db as test runs
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Start the relayer
	err = r.StartRelayer(ctx, eRep, pathGaiaOsmosis)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Wait a few blocks for the relayer to start.
	err = testutil.WaitForBlocks(ctx, 2, gaia, osmosis)
	require.NoError(t, err)

	// Generate a new IBC path
	err = r.GeneratePath(ctx, eRep, gaia.Config().ChainID, osmosis.Config().ChainID, pathGaiaOsmosis)
	require.NoError(t, err)

	// Create clients and wait a few blocks for the clients to be created
	err = r.CreateClients(ctx, eRep, pathGaiaOsmosis, ibc.DefaultClientOpts())
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, gaia, osmosis)
	require.NoError(t, err)

	// Dump relayer config and verify client IDs are written to path config
	rly := r.(*interchaintestrly.CosmosRelayer)

	showConfig := []string{"rly", "config", "show", "-j", "--home", rly.HomeDir()}
	res := r.Exec(ctx, eRep, showConfig, nil)
	require.NoError(t, res.Err)

	config := &cmd.Config{}
	err = json.Unmarshal(res.Stdout, config)
	require.NoError(t, err)

	rlyPath, err := config.Paths.Get(pathGaiaOsmosis)
	require.NoError(t, err)

	srcClientID := rlyPath.Src.ClientID
	dstClientID := rlyPath.Dst.ClientID

	require.NotEmpty(t, srcClientID)
	require.NotEmpty(t, dstClientID)

	// Create clients again without override and verify it was a noop
	err = r.CreateClients(ctx, eRep, pathGaiaOsmosis, ibc.DefaultClientOpts())
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, gaia, osmosis)
	require.NoError(t, err)

	res = r.Exec(ctx, eRep, showConfig, nil)
	require.NoError(t, res.Err)

	err = json.Unmarshal(res.Stdout, config)
	require.NoError(t, err)

	rlyPath, err = config.Paths.Get(pathGaiaOsmosis)
	require.NoError(t, err)

	newSrcClientID := rlyPath.Src.ClientID
	newDstClientID := rlyPath.Dst.ClientID

	require.NotEmpty(t, newSrcClientID)
	require.NotEmpty(t, newDstClientID)
	require.Equal(t, srcClientID, newSrcClientID)
	require.Equal(t, dstClientID, newDstClientID)

	// Create clients again with override and verify new client IDs are generated and added to config
	clientsOverride := []string{"rly", "tx", "clients", pathGaiaOsmosis, "--override", "--home", rly.HomeDir()}
	res = r.Exec(ctx, eRep, clientsOverride, nil)
	require.NoError(t, res.Err)

	err = testutil.WaitForBlocks(ctx, 5, gaia, osmosis)
	require.NoError(t, err)

	res = r.Exec(ctx, eRep, showConfig, nil)
	require.NoError(t, res.Err)

	err = json.Unmarshal(res.Stdout, config)
	require.NoError(t, err)

	rlyPath, err = config.Paths.Get(pathGaiaOsmosis)
	require.NoError(t, err)

	newSrcClientID = rlyPath.Src.ClientID
	newDstClientID = rlyPath.Dst.ClientID

	require.NotEmpty(t, newSrcClientID)
	require.NotEmpty(t, newDstClientID)
	require.NotEqual(t, srcClientID, newSrcClientID)
	require.NotEqual(t, dstClientID, newDstClientID)
}
