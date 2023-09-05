package interchaintest_test

import (
	"context"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
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

func TestMultipleChannelsOneConnection(t *testing.T) {
	image := relayerinterchaintest.BuildRelayerImage(t)

	client, network := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		interchaintestrelayer.CustomDockerImage(image, "latest", "100:1000"),
		interchaintestrelayer.ImagePull(false),
	).Build(t, client, network)

	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:        "gaia",
			ChainName:   "gaia",
			Version:     "v7.0.3",
			ChainConfig: ibc.ChainConfig{GasPrices: "0.00atom"},
		},
		{
			Name:        "osmosis",
			ChainName:   "osmosis",
			Version:     "v11.0.1",
			ChainConfig: ibc.ChainConfig{GasPrices: "0.00osmo"},
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

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create user accounts on both chains
	const initFunds = int64(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user-key", initFunds, gaia, osmosis)

	gaiaUser, osmosisUser := users[0], users[1]

	// Create the second and third channels on the same connection as the first channel
	rly := r.(*interchaintestrly.CosmosRelayer)

	creatChanCmd := []string{
		"rly", "tx", "channel", pathGaiaOsmosis,
		"--override",
		"--home", rly.HomeDir(),
	}

	for i := 0; i < 2; i++ {
		res := r.Exec(ctx, eRep, creatChanCmd, nil)
		require.NotEmpty(t, res)
		require.NoError(t, res.Err)
	}

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

	// Wait a few blocks for the relayer to start
	err = testutil.WaitForBlocks(ctx, 5, gaia, osmosis)
	require.NoError(t, err)

	// Assert that all three channels were successfully initialized
	channels, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 3, len(channels))

	// Send an IBC transfer across all three channels
	const transferAmount = int64(1000)
	transfer := ibc.WalletAmount{
		Address: osmosisUser.FormattedAddress(),
		Denom:   gaia.Config().Denom,
		Amount:  transferAmount,
	}

	for _, channel := range channels {
		_, err = gaia.SendIBCTransfer(ctx, channel.ChannelID, gaiaUser.KeyName(), transfer, ibc.TransferOptions{})
		require.NoError(t, err)
	}

	// Wait a few blocks for the transfers to be successfully relayed
	err = testutil.WaitForBlocks(ctx, 5, gaia, osmosis)
	require.NoError(t, err)

	// Compose IBC denoms for each channel
	ibcDenoms := make([]transfertypes.DenomTrace, 3)

	ibcDenoms[0] = transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, gaia.Config().Denom))
	ibcDenoms[1] = transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[1].Counterparty.PortID, channels[1].Counterparty.ChannelID, gaia.Config().Denom))
	ibcDenoms[2] = transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[2].Counterparty.PortID, channels[2].Counterparty.ChannelID, gaia.Config().Denom))

	// Assert that the transfers are all successful out of the src chain account
	nativeGaiaBal, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, initFunds-transferAmount*3, nativeGaiaBal)

	// Assert that the transfers are all successful on the dst chain account
	for _, denom := range ibcDenoms {
		balance, err := osmosis.GetBalance(ctx, osmosisUser.FormattedAddress(), denom.IBCDenom())
		require.NoError(t, err)
		require.Equal(t, transferAmount, balance)
	}

	// Send the funds back to the original source chain
	for i, channel := range channels {
		transfer := ibc.WalletAmount{
			Address: gaiaUser.FormattedAddress(),
			Denom:   ibcDenoms[i].IBCDenom(),
			Amount:  transferAmount,
		}

		_, err = osmosis.SendIBCTransfer(ctx, channel.Counterparty.ChannelID, osmosisUser.KeyName(), transfer, ibc.TransferOptions{})
		require.NoError(t, err)
	}

	// Wait a few blocks for the transfers to be successfully relayed
	err = testutil.WaitForBlocks(ctx, 5, gaia, osmosis)
	require.NoError(t, err)

	// Assert that the transfers are all successful back on the original src chain account
	nativeGaiaBal, err = gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, initFunds, nativeGaiaBal)

	// Assert that the transfers are all successfully sent back to the original src chain account
	for _, denom := range ibcDenoms {
		balance, err := osmosis.GetBalance(ctx, osmosisUser.FormattedAddress(), denom.IBCDenom())
		require.NoError(t, err)
		require.Equal(t, int64(0), balance)
	}

}
