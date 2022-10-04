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
)

// TestClientThreshold tests that the relyer will update light clients within a
// user specified time threshold.
func TestClientThresholdUpdate(t *testing.T) {

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

	// Relayer is set with "--time-threshold 5m"
	// The client beeing created below also has a trusting period of 5m.
	// The relayer should update the client immediatly.
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest", "100:1000"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--time-threshold", "5m"),
	).Build(t, client, network)

	// Prep Interchain with client trusting period of 5 min)
	const ibcPath = "demo-path"
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

	// Count how many updateclient messages were sent for each chain after specified height
	const qUpdateClientBothChains = `SELECT
	 COUNT(*)
	 FROM v_cosmos_messages
	 WHERE (type = "/ibc.core.client.v1.MsgUpdateClient" AND chain_id = ? AND block_height >= ?)
	 OR (type = "/ibc.core.client.v1.MsgUpdateClient" AND chain_id = ? AND block_height >= ?)
	 `

	// Verify there are no updateclient messages after noted height for each chain.
	var count int
	row := db.QueryRow(qUpdateClientBothChains, g0ChainId, g0_height, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Start relayer
	r.StartRelayer(ctx, eRep, ibcPath)

	// Give relayer time to sync both chains
	test.WaitForBlocks(ctx, 4, g0, g1)

	// Count how many updateclient messages were sent for each chain after specified height
	const qUpdateClientSingleChain = `SELECT
	 COUNT(*)
	 FROM v_cosmos_messages
	 WHERE type = "/ibc.core.client.v1.MsgUpdateClient" 
	 AND chain_id = ? AND block_height >= ?
	 `

	// Verify there are updateclient messages for g0
	row = db.QueryRow(qUpdateClientSingleChain, g0ChainId, g0_height)
	require.NoError(t, row.Scan(&count))
	require.Greater(t, count, 0)

	// Verify there are updateclient messages for g1
	row = db.QueryRow(qUpdateClientSingleChain, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Greater(t, count, 0)

}

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
	// The Relayer should NOT conitinuously update clients
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayeribctest.RelayerImageName, "latest", "100:1000"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--time-threshold", "0"),
	).Build(t, client, network)

	// Prep Interchain with client trusting period of 5 min)
	const ibcPath = "demo-path"
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

	// Count how many updateclient messages were sent for each chain after specified height
	const qUpdateClientBothChains = `SELECT
	 COUNT(*)
	 FROM v_cosmos_messages
	 WHERE (type = "/ibc.core.client.v1.MsgUpdateClient" AND chain_id = ? AND block_height >= ?)
	 OR (type = "/ibc.core.client.v1.MsgUpdateClient" AND chain_id = ? AND block_height >= ?)
	 `

	// Verify there are no updateclient messages after noted height for each chain.
	var count int
	row := db.QueryRow(qUpdateClientBothChains, g0ChainId, g0_height, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Start relayer
	r.StartRelayer(ctx, eRep, ibcPath)

	// Give relayer time to sync both chains
	test.WaitForBlocks(ctx, 4, g0, g1)

	// Count how many updateclient messages were sent for each chain after specified height
	const qUpdateClientSingleChain = `SELECT
	 COUNT(*)
	 FROM v_cosmos_messages
	 WHERE type = "/ibc.core.client.v1.MsgUpdateClient" 
	 AND chain_id = ? AND block_height >= ?
	 `

	// Verify there are 0 updateclient messages for g0
	row = db.QueryRow(qUpdateClientSingleChain, g0ChainId, g0_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Verify there are 0 updateclient messages for g1
	row = db.QueryRow(qUpdateClientSingleChain, g1ChainId, g1_height)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

}
