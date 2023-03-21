package interchaintest_test

import (
	"context"
	"fmt"
	"testing"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestFeeMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "juno", Version: "v13.0.0", ChainConfig: ibc.ChainConfig{ChainID: "chain-a", GasPrices: "0.0ujuno"}},
		{Name: "juno", Version: "v13.0.0", ChainConfig: ibc.ChainConfig{ChainID: "chain-b", GasPrices: "0.0ujuno"}}},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA, chainB := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathChainAChainB = "chainA-chainB"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    pathChainAChainB,
			CreateChannelOpts: ibc.CreateChannelOptions{
				SourcePortName: "transfer",
				DestPortName:   "transfer",
				Order:          ibc.Unordered,
				Version:        "{\"fee_version\":\"ics29-1\",\"app_version\":\"ics20-1\"}",
			}})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	height, err := chainB.Height(ctx)
	require.NoError(t, err)
	_, err = cosmos.PollForMessage[chantypes.MsgChannelOpenConfirm](ctx, chainB, chainB.Config().EncodingConfig.InterfaceRegistry, 1, height+10, nil)
	require.NoError(t, err)

	// // Create the Path
	// pathName := "chainA-chainB"
	// err = r.GeneratePath(ctx, eRep, chainA.Config().ChainID, chainB.Config().ChainID, pathName)
	// require.NoError(t, err)

	// // Create the lightclients
	// err = r.CreateClients(ctx, eRep, pathName, ibc.CreateClientOptions{})
	// require.NoError(t, err)

	// // Create a new connection
	// err = r.CreateConnections(ctx, eRep, pathName)
	// require.NoError(t, err)

	// // Create the channel
	// err = r.CreateChannel(ctx, eRep, pathName, ibc.CreateChannelOptions{Version: "{\"fee_version\":\"fee29-1\",\"app_version\":\"ics20-1\"}"})
	// require.NoError(t, err)
	// time.Sleep(2*time.Minute)

	// Start the relayer
	require.NoError(t, r.StartRelayer(ctx, eRep, pathChainAChainB))
	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// Query for Channel of both the chains
	channelsA, err := r.GetChannels(ctx, eRep, chainA.Config().ChainID)
	require.NoError(t, err)
	channelA := channelsA[0]

	channelsB, err := r.GetChannels(ctx, eRep, chainB.Config().ChainID)
	require.NoError(t, err)
	channelB := channelsB[0]

	// Query for the Addresses of both the chains
	WalletA, ok := r.GetWallet(chainA.Config().ChainID)

	WalletB, ok := r.GetWallet(chainB.Config().ChainID)

}
