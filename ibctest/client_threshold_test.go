package ibctest_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	relayeribctest "github.com/cosmos/relayer/v2/ibctest"
	"github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/v5/relayer"
	"github.com/strangelove-ventures/ibctest/v5/test"
	"github.com/strangelove-ventures/ibctest/v5/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

const (
	g0ChainId = "gaia-0"
	g1ChainId = "gaia-1"

	ibcPath = "demo-path"
)

// Tests that the Relayer will update light clients within a
// user specified time threshold.
// If the client is set to expire withing the threshold, the relayer should update the client.
func TestClientThresholdUpdate(t *testing.T) {
	t.Parallel()

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
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Wait 2 blocks after building interchain
	require.NoError(t, test.WaitForBlocks(ctx, 2, g0, g1))

	g0Height, err := g0.Height(ctx)
	require.NoError(t, err)
	g1Height, err := g1.Height(ctx)
	require.NoError(t, err)

	require.NoError(t, r.StartRelayer(ctx, eRep, ibcPath))
	t.Cleanup(func() {
		_ = r.StopRelayer(ctx, eRep)
	})

	const (
		clientID     = "07-tendermint-0"
		heightOffset = 10
	)
	var eg errgroup.Group

	eg.Go(func() error {
		msg, err := pollForUpdateClient(ctx, g0, g0Height, g0Height+heightOffset)
		if err != nil {
			return fmt.Errorf("first chain: %w", err)
		}
		if msg.ClientId != clientID {
			return fmt.Errorf("first chain: unexpected client id, want %s, got %s", clientID, msg.ClientId)
		}
		return nil
	})
	eg.Go(func() error {
		msg, err := pollForUpdateClient(ctx, g1, g1Height, g1Height+heightOffset)
		if err != nil {
			return fmt.Errorf("second chain: %w", err)
		}
		if msg.ClientId != clientID {
			return fmt.Errorf("second chain: unexpected client id, want %s, got %s", clientID, msg.ClientId)
		}
		return nil
	})

	require.NoError(t, eg.Wait())
}

// Tests that passing in a "--time-threshold" of "0" to the relayer
// will not update the client if it nears expiration.
func TestClientThresholdNoUpdate(t *testing.T) {
	t.Parallel()

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

	g0Height, err := g0.Height(ctx)
	require.NoError(t, err)
	g1Height, err := g1.Height(ctx)
	require.NoError(t, err)

	require.NoError(t, r.StartRelayer(ctx, eRep, ibcPath))
	t.Cleanup(func() {
		_ = r.StopRelayer(ctx, eRep)
	})

	const heightOffset = 10

	var errCount int64

	var eg errgroup.Group
	eg.Go(func() error {
		_, err := pollForUpdateClient(ctx, g0, g0Height, g0Height+heightOffset)
		if err != nil {
			atomic.AddInt64(&errCount, 1)
		}
		return err
	})
	eg.Go(func() error {
		_, err := pollForUpdateClient(ctx, g1, g1Height, g1Height+heightOffset)
		if err != nil {
			atomic.AddInt64(&errCount, 1)
		}
		return err
	})

	require.Error(t, eg.Wait())
	require.EqualValues(t, 2, errCount)
}
