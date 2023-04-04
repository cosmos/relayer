package interchaintest_test

import (
	"context"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

// TestRelayerMultiplePathsSingleProcess tests relaying multiple paths
// from the same process using the go relayer. A single
// CosmosChainProcessor (gaia) will feed data to two PathProcessors (gaia-osmosis and gaia-juno).
func TestRelayerMultiplePathsSingleProcess(t *testing.T) {
	var (
		r    = relayerinterchaintest.NewRelayer(t, relayerinterchaintest.RelayerConfig{})
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)

	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "gaia",
			ChainName:     "gaia",
			Version:       "v7.0.3",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			Name:          "osmosis",
			ChainName:     "osmosis",
			Version:       "v11.0.1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			Name:          "juno",
			ChainName:     "juno",
			Version:       "v9.0.0",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia, osmosis, juno := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain), chains[2].(*cosmos.CosmosChain)
	osmosisCfg, junoCfg := osmosis.Config(), juno.Config()

	// Build the network; spin up the chains and configure the relayer
	const pathGaiaOsmosis = "gaia-osmosis"
	const pathGaiaJuno = "gaia-juno"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddChain(juno).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    pathGaiaOsmosis,
		}).
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia,
			Chain2:  juno,
			Relayer: r,
			Path:    pathGaiaJuno,
		})

	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
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

	// Start the relayers
	err = r.StartRelayer(ctx, eRep, pathGaiaOsmosis, pathGaiaJuno)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, gaia, osmosis, juno)
	gaiaUser, osmosisUser, junoUser := users[0].(*cosmos.CosmosWallet), users[1].(*cosmos.CosmosWallet), users[2].(*cosmos.CosmosWallet)

	// Wait a few blocks for user accounts to be created on chain.
	err = testutil.WaitForBlocks(ctx, 5, gaia, osmosis, juno)
	require.NoError(t, err)

	gaiaAddress := gaiaUser.FormattedAddress()
	require.NotEmpty(t, gaiaAddress)

	osmosisAddress := osmosisUser.FormattedAddress()
	require.NotEmpty(t, osmosisAddress)

	junoAddress := junoUser.FormattedAddress()
	require.NotEmpty(t, junoAddress)

	// get ibc chans
	osmosisChans, err := r.GetChannels(ctx, eRep, osmosisCfg.ChainID)
	require.NoError(t, err)

	junoChans, err := r.GetChannels(ctx, eRep, junoCfg.ChainID)
	require.NoError(t, err)

	osmosisIBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(osmosisChans[0].Counterparty.PortID, osmosisChans[0].Counterparty.ChannelID, osmosisCfg.Denom)).IBCDenom()
	junoIBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(junoChans[0].Counterparty.PortID, junoChans[0].Counterparty.ChannelID, junoCfg.Denom)).IBCDenom()

	var eg errgroup.Group

	const transferAmount = int64(1_000_000)

	eg.Go(func() error {
		osmosisHeight, err := osmosis.Height(ctx)
		if err != nil {
			return err
		}
		// Fund gaia user with ibc denom osmo
		tx, err := osmosis.SendIBCTransfer(ctx, osmosisChans[0].ChannelID, osmosisUser.KeyName(), ibc.WalletAmount{
			Amount:  transferAmount,
			Denom:   osmosisCfg.Denom,
			Address: gaiaAddress,
		}, ibc.TransferOptions{})
		if err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
		return err
	})

	eg.Go(func() error {
		junoHeight, err := juno.Height(ctx)
		if err != nil {
			return err
		}
		// Fund gaia user with ibc denom juno
		tx, err := juno.SendIBCTransfer(ctx, junoChans[0].ChannelID, junoUser.KeyName(), ibc.WalletAmount{
			Amount:  transferAmount,
			Denom:   junoCfg.Denom,
			Address: gaiaAddress,
		}, ibc.TransferOptions{})
		if err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, juno, junoHeight, junoHeight+10, tx.Packet)
		return err
	})

	require.NoError(t, eg.Wait())

	osmosisOnGaiaBalance, err := gaia.GetBalance(ctx, gaiaAddress, osmosisIBCDenom)
	require.NoError(t, err)

	require.Equal(t, transferAmount, osmosisOnGaiaBalance)

	junoOnGaiaBalance, err := gaia.GetBalance(ctx, gaiaAddress, junoIBCDenom)
	require.NoError(t, err)

	require.Equal(t, transferAmount, junoOnGaiaBalance)
}
