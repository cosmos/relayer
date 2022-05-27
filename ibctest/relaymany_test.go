package ibctest_test

import (
	"context"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

func TestRelayMany(t *testing.T) {
	t.Parallel()

	pool, network := ibctest.DockerSetup(t)
	home := t.TempDir()

	cf := ibctest.NewBuiltinChainFactory([]ibctest.BuiltinChainFactoryEntry{
		gaiaFactoryEntry,
		osmosisFactoryEntry,
		junoFactoryEntry,
	}, zaptest.NewLogger(t))

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia, osmosis, juno := chains[0], chains[1], chains[2]

	r := relayerFactory{}.Build(t, pool, network, home).(*relayer)

	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddChain(juno).
		AddRelayer(r, "r").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    "gaia-osmo",
		}).
		AddLink(ibctest.InterchainLink{
			Chain1:  osmosis,
			Chain2:  juno,
			Relayer: r,
			Path:    "osmo-juno",
		})

	eRep := testreporter.NewReporter(newNopWriteCloser()).RelayerExecReporter(t)

	ctx := context.Background()

	err = ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName: t.Name(),
		HomeDir:  home,

		Pool:      pool,
		NetworkID: network,
	})
	require.NoError(t, err)

	// Note, StartRelayerMany is a method on *relayer,
	// not part of the ibc.Relayer interface.
	r.StartRelayerMany(ctx, "gaia-osmo", "osmo-juno")
	defer r.StopRelayer(ctx, eRep)

	// Gaia and juno faucets will just send IBC to the osmosis faucet,
	// so get its bech32 address first.
	osmosisFaucetAddrBytes, err := osmosis.GetAddress(ctx, ibctest.FaucetAccountKeyName)
	require.NoError(t, err)
	osmosisFaucetAddr, err := types.Bech32ifyAddressBytes(osmosis.Config().Bech32Prefix, osmosisFaucetAddrBytes)
	require.NoError(t, err)

	osmosisHeightBeforeTransfers, err := osmosis.Height(ctx)
	require.NoError(t, err)

	var gaiaTx, junoTx ibc.Tx

	// Concurrently send IBC transfers.
	// Each one independently takes about 4 seconds,
	// so it's worth it for a little time saved.
	eg, egCtx := errgroup.WithContext(ctx)

	// Send the gaia transfer.
	eg.Go(func() error {
		gaiaChannels, err := r.GetChannels(egCtx, eRep, gaia.Config().ChainID)
		require.NoError(t, err)
		gaiaTx, err = gaia.SendIBCTransfer(egCtx, gaiaChannels[0].ChannelID, ibctest.FaucetAccountKeyName, ibc.WalletAmount{
			Address: osmosisFaucetAddr,
			Denom:   gaia.Config().Denom,
			Amount:  100,
		}, nil)

		return err
	})

	// Send the juno transfer.
	eg.Go(func() error {
		junoChannels, err := r.GetChannels(egCtx, eRep, juno.Config().ChainID)
		require.NoError(t, err)
		junoTx, err = juno.SendIBCTransfer(egCtx, junoChannels[0].ChannelID, ibctest.FaucetAccountKeyName, ibc.WalletAmount{
			Address: osmosisFaucetAddr,
			Denom:   juno.Config().Denom,
			Amount:  100,
		}, nil)
		return err
	})

	require.NoError(t, eg.Wait())

	osmosisHeightAfterTransfers, err := osmosis.Height(ctx)
	require.NoError(t, err)

	_, err = test.PollForAck(ctx, osmosis, osmosisHeightBeforeTransfers, osmosisHeightAfterTransfers+15, gaiaTx.Packet)
	require.NoError(t, err)

	_, err = test.PollForAck(ctx, osmosis, osmosisHeightBeforeTransfers, osmosisHeightAfterTransfers+15, junoTx.Packet)
	require.NoError(t, err)
}
