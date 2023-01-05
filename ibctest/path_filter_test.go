package ibctest_test

import (
	"context"
	"fmt"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	relayeribctest "github.com/cosmos/relayer/v2/ibctest"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/test"
	"github.com/strangelove-ventures/ibctest/v5/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

// TestScenarioPathFilterAllow tests the channel allowlist
func TestScenarioPathFilterAllow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	nv := 1
	nf := 0

	// Chain Factory
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
		{Name: "osmosis", Version: "v11.0.1", NumValidators: &nv, NumFullNodes: &nf},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// Relayer Factory to construct relayer
	r := relayeribctest.NewRelayerFactory(relayeribctest.RelayerConfig{
		Processor:           relayer.ProcessorEvents,
		InitialBlockHistory: 100,
	}).Build(t, nil, "")

	// Prep Interchain
	const ibcPath = "gaia-osmosis"
	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    ibcPath,
		})

	// Reporter/logs
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	client, network := ibctest.DockerSetup(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))

	// Get Channel ID
	gaiaChans, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannel := gaiaChans[0]
	osmosisChannel := gaiaChans[0].Counterparty

	r.UpdatePath(ctx, eRep, ibcPath, ibc.ChannelFilter{
		Rule:        processor.RuleAllowList,
		ChannelList: []string{gaiaChannel.ChannelID},
	})

	// Create and Fund User Wallets
	fundAmount := int64(10_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), gaia, osmosis)

	gaiaUser, osmosisUser := users[0], users[1]

	r.StartRelayer(ctx, eRep, ibcPath)

	// Send Transaction
	amountToSend := int64(1_000_000)
	gaiaDstAddress := gaiaUser.Bech32Address(osmosis.Config().Bech32Prefix)
	osmosisDstAddress := osmosisUser.Bech32Address(gaia.Config().Bech32Prefix)

	gaiaHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	osmosisHeight, err := osmosis.Height(ctx)
	require.NoError(t, err)

	var eg errgroup.Group
	eg.Go(func() error {
		tx, err := gaia.SendIBCTransfer(ctx, gaiaChannel.ChannelID, gaiaUser.KeyName, ibc.WalletAmount{
			Address: gaiaDstAddress,
			Denom:   gaia.Config().Denom,
			Amount:  amountToSend,
		},
			nil,
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}
		_, err = test.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+10, tx.Packet)
		return err
	})

	eg.Go(func() error {
		tx, err := osmosis.SendIBCTransfer(ctx, osmosisChannel.ChannelID, osmosisUser.KeyName, ibc.WalletAmount{
			Address: osmosisDstAddress,
			Denom:   osmosis.Config().Denom,
			Amount:  amountToSend,
		},
			nil,
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}
		_, err = test.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
		return err
	})
	// Acks should exist
	require.NoError(t, eg.Wait())

	// Trace IBC Denom
	gaiaDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(osmosisChannel.PortID, osmosisChannel.ChannelID, gaia.Config().Denom))
	gaiaIbcDenom := gaiaDenomTrace.IBCDenom()

	osmosisDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(gaiaChannel.PortID, gaiaChannel.ChannelID, osmosis.Config().Denom))
	osmosisIbcDenom := osmosisDenomTrace.IBCDenom()

	// Test destination wallets have increased funds
	gaiaIBCBalance, err := osmosis.GetBalance(ctx, gaiaDstAddress, gaiaIbcDenom)
	require.NoError(t, err)
	require.Equal(t, amountToSend, gaiaIBCBalance)

	osmosisIBCBalance, err := gaia.GetBalance(ctx, osmosisDstAddress, osmosisIbcDenom)
	require.NoError(t, err)
	require.Equal(t, amountToSend, osmosisIBCBalance)
}

// TestScenarioPathFilterDeny tests the channel denylist
func TestScenarioPathFilterDeny(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	nv := 1
	nf := 0

	// Chain Factory
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
		{Name: "osmosis", Version: "v11.0.1", NumValidators: &nv, NumFullNodes: &nf},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// Relayer Factory to construct relayer
	r := relayeribctest.NewRelayerFactory(relayeribctest.RelayerConfig{
		Processor:           relayer.ProcessorEvents,
		InitialBlockHistory: 100,
	}).Build(t, nil, "")

	// Prep Interchain
	const ibcPath = "gaia-osmosis"
	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    ibcPath,
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	client, network := ibctest.DockerSetup(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))

	// Get Channel ID
	gaiaChans, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannel := gaiaChans[0]
	osmosisChannel := gaiaChans[0].Counterparty

	r.UpdatePath(ctx, eRep, ibcPath, ibc.ChannelFilter{
		Rule:        processor.RuleDenyList,
		ChannelList: []string{gaiaChannel.ChannelID},
	})

	// Create and Fund User Wallets
	fundAmount := int64(10_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), gaia, osmosis)

	gaiaUser, osmosisUser := users[0], users[1]

	r.StartRelayer(ctx, eRep, ibcPath)

	// Send Transaction
	amountToSend := int64(1_000_000)
	gaiaDstAddress := gaiaUser.Bech32Address(osmosis.Config().Bech32Prefix)
	osmosisDstAddress := osmosisUser.Bech32Address(gaia.Config().Bech32Prefix)

	gaiaHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	osmosisHeight, err := osmosis.Height(ctx)
	require.NoError(t, err)

	var eg errgroup.Group
	eg.Go(func() error {
		tx, err := gaia.SendIBCTransfer(ctx, gaiaChannel.ChannelID, gaiaUser.KeyName, ibc.WalletAmount{
			Address: gaiaDstAddress,
			Denom:   gaia.Config().Denom,
			Amount:  amountToSend,
		},
			nil,
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}

		// we want an error here
		ack, err := test.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+10, tx.Packet)
		if err == nil {
			return fmt.Errorf("no error when error was expected when polling for ack: %+v", ack)
		}

		return nil
	})

	eg.Go(func() error {
		tx, err := osmosis.SendIBCTransfer(ctx, osmosisChannel.ChannelID, osmosisUser.KeyName, ibc.WalletAmount{
			Address: osmosisDstAddress,
			Denom:   osmosis.Config().Denom,
			Amount:  amountToSend,
		},
			nil,
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}

		// we want an error here
		ack, err := test.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
		if err == nil {
			return fmt.Errorf("no error when error was expected when polling for ack: %+v", ack)
		}

		return nil
	})
	// Test that acks do not show up
	require.NoError(t, eg.Wait())

	// Trace IBC Denom
	gaiaDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(osmosisChannel.PortID, osmosisChannel.ChannelID, gaia.Config().Denom))
	gaiaIbcDenom := gaiaDenomTrace.IBCDenom()

	osmosisDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(gaiaChannel.PortID, gaiaChannel.ChannelID, osmosis.Config().Denom))
	osmosisIbcDenom := osmosisDenomTrace.IBCDenom()

	// Test destination wallets do not have increased funds
	gaiaIBCBalance, err := osmosis.GetBalance(ctx, gaiaDstAddress, gaiaIbcDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), gaiaIBCBalance)

	osmosisIBCBalance, err := gaia.GetBalance(ctx, osmosisDstAddress, osmosisIbcDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), osmosisIBCBalance)
}
