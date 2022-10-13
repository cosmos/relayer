package ibctest_test

import (
	"context"
	"path/filepath"
	"testing"

	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
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
)

func findClientStatus(ctx context.Context, chain ibc.Chain, clientID string) (string, error) {
	queryClient := clienttypes.NewQueryClient(CliContext(chain))
	req := &clienttypes.QueryClientStatusRequest{
		ClientId: clientID,
	}
	res, err := queryClient.ClientStatus(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Status, err
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

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Wait 2 blocks after building interchain
	require.NoError(t, test.WaitForBlocks(ctx, 2, g0, g1))

	require.NoError(t, r.StartRelayer(ctx, eRep, ibcPath))
	t.Cleanup(func() {
		_ = r.StopRelayer(ctx, eRep)
	})

	// Give relayer time to sync both chains
	require.NoError(t, test.WaitForBlocks(ctx, 5, g0, g1))

	const clientID = "07-tendermint-0"

	// Find client status for first chain
	status, err := findClientStatus(ctx, g0, clientID)

	require.NoError(t, err)
	require.Equal(t, "Active", status)

	// Find client status for second chain
	status, err = findClientStatus(ctx, g1, clientID)

	require.NoError(t, err)
	require.Equal(t, "Active", status)
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

	// Wait 2 blocks after building interchain
	require.NoError(t, test.WaitForBlocks(ctx, 2, g0, g1))

	require.NoError(t, r.StartRelayer(ctx, eRep, ibcPath))
	t.Cleanup(func() {
		_ = r.StopRelayer(ctx, eRep)
	})

	// Give relayer time to sync both chains
	require.NoError(t, test.WaitForBlocks(ctx, 5, g0, g1))

	const clientID = "07-tendermint-0"

	// Find client status for first chain
	status, err := findClientStatus(ctx, g0, clientID)

	require.NoError(t, err)
	require.Equal(t, "Active", status)

	// Find client status for second chain
	status, err = findClientStatus(ctx, g1, clientID)

	require.NoError(t, err)
	require.Equal(t, "Active", status)
}
