package ibctest_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	relayeribctest "github.com/cosmos/relayer/v2/ibctest"
	"github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/v5/relayer"
	"github.com/strangelove-ventures/ibctest/v5/test"
	"github.com/strangelove-ventures/ibctest/v5/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	g0ChainId = "gaia-0"
	g1ChainId = "gaia-1"

	ibcPath = "demo-path"

	// Count how many MsgUpdateClient messages were sent for each chain after specified height
	qUpdateClientBothChains = `SELECT
	 COUNT(*)
	 FROM v_cosmos_messages
	 WHERE (type = "/ibc.core.client.v1.MsgUpdateClient" AND chain_id = ? AND block_height >= ?)
	 OR (type = "/ibc.core.client.v1.MsgUpdateClient" AND chain_id = ? AND block_height >= ?)
	 `

	// Count how many MsgUpdateClient messages were sent for one chain after specified height
	qUpdateClientSingleChain = `SELECT
	 COUNT(*)
	 FROM v_cosmos_messages
	 WHERE type = "/ibc.core.client.v1.MsgUpdateClient" 
	 AND chain_id = ? AND block_height >= ?
	 `
)

func pollForClientStatus(ctx context.Context, chain *ibc.Chain, startHeight, maxHeight uint64, clientID string) (clientStatus string, err error) {
	cliContext := ibctest.CosmosChain.cliContext()

	queryClient := clienttypes.NewQueryClient(cliContext)

	req := &clienttypes.QueryClientStatusRequest{
		ClientId: clientID,
	}

	clientStatusRes, err := queryClient.ClientStatus(ctx, req)
	if err != nil {
		return "", err
	}

	return clientStatusRes, nil
}

// Tests that the Relayer will update light clients within a
// user specified time threshold.
// If the client is set to expire withing the threshold, the relayer should update the client.
func TestClientThresholdUpdate(t *testing.T) {

	ctx := context.Background()

	nv := 1
	nf := 0

	// Chain Factory
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		// Two otherwise identical chains that only differ by ChainName and ChainID.
		{Name: "gaia", ChainName: "g0", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{ChainID: g0ChainId}},
		{Name: "gaia", ChainName: "g1", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{ChainID: g1ChainId}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	g0, g1 := chains[0], chains[1]

	client, network := ibctest.DockerSetup(t)
	relayeribctest.BuildRelayerImage(t)

	// Relayer is set with "--time-threshold 5m"
	// The client being created below also has a trusting period of 5m.
	// The relayer should automatically update the client after chains re in sync.
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest", "100:1000"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--time-threshold", "5m"),
	).Build(t, client, network)

	// Prep Interchain with client trusting period of 5 min
	ic := ibctest.NewInterchain().
		AddChain(g0).
		AddChain(g1).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  g0,
			Chain2:  g1,
			Relayer: r,
			Path:    ibcPath,
			CreateClientOpts: ibc.CreateClientOptions{
				TrustingPeriod: "5m",
			},
		})

	// Reporter/logs
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	dbDir := ibctest.TempDir(t)
	dbPath := filepath.Join(dbDir, "blocks.db")

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,

		BlockDatabaseFile: dbPath,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// The database should exist on disk
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Copy the busy timeout from the migration
	// The journal_mode pragma should be persisted on disk, so we should not need to set that here
	_, err = db.Exec(`PRAGMA busy_timeout = 3000`)
	require.NoError(t, err)

	// Wait 2 blocks after building interchain
	test.WaitForBlocks(ctx, 2, g0, g1)

	// get height of chains
	g0_height, _ := g0.Height(ctx)
	g1_height, _ := g1.Height(ctx)

	// Verify there are no "MsgUpdateClient" messages after noted height for each chain.
	var count int
	row := db.QueryRow(qUpdateClientBothChains, g0ChainId, g0_height, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Start relayer
	r.StartRelayer(ctx, eRep, ibcPath)

	// Give relayer time to sync both chains
	test.WaitForBlocks(ctx, 4, g0, g1)

	// Verify there are MsgUpdateClient messages for g0
	row = db.QueryRow(qUpdateClientSingleChain, g0ChainId, g0_height)
	require.NoError(t, row.Scan(&count))
	require.Greater(t, count, 0)

	// Verify there are MsgUpdateClient messages for g1
	row = db.QueryRow(qUpdateClientSingleChain, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Greater(t, count, 0)

}

// Tests that passing in a "--time-threshold" of "0" to the relayer
// will not update the client if it nears expiration.
func TestClientThresholdNoUpdate(t *testing.T) {

	ctx := context.Background()

	nv := 1
	nf := 0

	// Chain Factory
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		// Two otherwise identical chains that only differ by ChainID.
		{Name: "gaia", ChainName: "g0", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{ChainID: g0ChainId}},
		{Name: "gaia", ChainName: "g1", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{ChainID: g1ChainId}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	g0, g1 := chains[0], chains[1]

	client, network := ibctest.DockerSetup(t)
	relayeribctest.BuildRelayerImage(t)

	// Relayer is set with "--time-threshold 0"
	// The Relayer should NOT continuously update clients
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest", "100:1000"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--time-threshold", "0"),
	).Build(t, client, network)

	// Prep Interchain with client trusting period of 5 min
	ic := ibctest.NewInterchain().
		AddChain(g0).
		AddChain(g1).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  g0,
			Chain2:  g1,
			Relayer: r,
			Path:    ibcPath,
			CreateClientOpts: ibc.CreateClientOptions{
				TrustingPeriod: "5m",
			},
		})

	// Reporter/logs
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	dbDir := ibctest.TempDir(t)
	dbPath := filepath.Join(dbDir, "blocks.db")

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,

		BlockDatabaseFile: dbPath,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// The database should exist on disk
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Copy the busy timeout from the migration
	// The journal_mode pragma should be persisted on disk, so we should not need to set that here
	_, err = db.Exec(`PRAGMA busy_timeout = 3000`)
	require.NoError(t, err)

	// Wait 2 blocks after building interchain
	test.WaitForBlocks(ctx, 2, g0, g1)

	// get height of chains
	g0_height, _ := g0.Height(ctx)
	g1_height, _ := g1.Height(ctx)

	// Verify there are no MsgUpdateClient messages after noted height for each chain.
	var count int
	row := db.QueryRow(qUpdateClientBothChains, g0ChainId, g0_height, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Start relayer
	r.StartRelayer(ctx, eRep, ibcPath)

	// Give relayer time to sync both chains
	test.WaitForBlocks(ctx, 4, g0, g1)

	// Verify there are 0 MsgUpdateClient messages for g0
	row = db.QueryRow(qUpdateClientSingleChain, g0ChainId, g0_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Verify there are 0 MsgUpdateClient messages for g1
	row = db.QueryRow(qUpdateClientSingleChain, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

}
