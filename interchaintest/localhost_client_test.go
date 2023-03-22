package interchaintest_test

import (
	"context"
	"fmt"
	"testing"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	ictestcosmos "github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestLocalhostIBC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 0
	image := ibc.DockerImage{
		Repository: "ghcr.io/cosmos/ibc-go-simd",
		Version:    "main",
		UidGid:     "",
	}
	cdc := ictestcosmos.DefaultEncoding()
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "ibc-go-simd",
			ChainName:     "simd",
			Version:       "main",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "simd",
				ChainID:        "chain-a",
				Images:         []ibc.DockerImage{image},
				Bin:            "simd",
				Bech32Prefix:   "cosmos",
				Denom:          "stake",
				CoinType:       "118",
				GasPrices:      "0.0stake",
				GasAdjustment:  1.1,
				EncodingConfig: &cdc,
			}}},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathLocalhost = "chainA-localhost"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddRelayer(r, "relayer")

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	const relayerKey = "relayer-key"
	err = r.AddChainConfiguration(ctx, eRep, chainA.Config(), relayerKey, chainA.GetHostRPCAddress(), chainA.GetHostGRPCAddress())
	require.NoError(t, err)

	wallet, err := r.AddKey(ctx, eRep, chainA.Config().ChainID, relayerKey, chainA.Config().CoinType)
	require.NoError(t, err)
	_ = wallet

	err = r.GeneratePath(ctx, eRep, chainA.Config().ChainID, chainA.Config().ChainID, pathLocalhost)
	require.NoError(t, err)

	updateCmd := []string{
		"paths", "update", pathLocalhost,
		"--src-client-id", ibcexported.LocalhostClientID,
		"--src-connection-id", ibcexported.LocalhostConnectionID,
		"--dst-client-id", ibcexported.LocalhostClientID,
		"--dst-connection-id", ibcexported.LocalhostConnectionID,
	}
	res := r.Exec(ctx, eRep, updateCmd, nil)
	require.NoError(t, res.Err)

	err = r.CreateClients(ctx, eRep, pathLocalhost, ibc.CreateClientOptions{TrustingPeriod: "1h"})
	require.NoError(t, err)

	err = r.CreateConnections(ctx, eRep, pathLocalhost)
	require.NoError(t, err)

	height, err := chainA.Height(ctx)
	require.NoError(t, err)

	err = r.CreateChannel(ctx, eRep, pathLocalhost, ibc.CreateChannelOptions{})
	require.NoError(t, err)

	_, err = ictestcosmos.PollForMessage[chantypes.MsgChannelOpenConfirm](
		ctx,
		chainA,
		chainA.Config().EncodingConfig.InterfaceRegistry,
		height,
		height+10,
		nil,
	)
	require.NoError(t, err)

	// create a new user account and wait a few blocks for it to be created on chain
	user := interchaintest.GetAndFundTestUsers(t, ctx, "user-1", 10_000_000, chainA)[0]
	err = testutil.WaitForBlocks(ctx, 5, chainA)

	_ = user

	// Start the relayer
	require.NoError(t, r.StartRelayer(ctx, eRep, pathLocalhost))

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)
}
