package ibctest_test

import (
	"context"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRelayMany_WIP(t *testing.T) {
	t.Parallel()

	pool, network := ibctest.DockerSetup(t)
	home := t.TempDir()

	cf := ibctest.NewBuiltinChainFactory([]ibctest.BuiltinChainFactoryEntry{
		gaiaFactoryEntry,
		osmosisFactoryEntry,
		// TODO: include junoFactoryEntry
	}, zaptest.NewLogger(t))

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia, osmosis := chains[0], chains[1]
	r := relayerFactory{}.Build(t, pool, network, home).(*relayer)

	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "r").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,

			Path: "gaia-osmo",
		})

	ctx := context.Background()
	eRep := testreporter.NewReporter(newNopWriteCloser()).RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   home,
		Pool:      pool,
		NetworkID: network,
	}))

	gaiaChannels, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannelID := gaiaChannels[0].ChannelID

	r.StartRelayerMany(ctx, "gaia-osmo")
	defer r.StopRelayer(ctx, eRep)

	// Gaia and juno faucets will just send IBC to the osmosis faucet,
	// so get its bech32 address first.
	osmosisFaucetAddrBytes, err := osmosis.GetAddress(ctx, ibctest.FaucetAccountKeyName)
	require.NoError(t, err)
	osmosisFaucetAddr, err := types.Bech32ifyAddressBytes(osmosis.Config().Bech32Prefix, osmosisFaucetAddrBytes)
	require.NoError(t, err)

	beforeGaiaTxHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	gaiaOsmoIBCDenom := transfertypes.ParseDenomTrace(
		transfertypes.GetPrefixedDenom(gaiaChannels[0].Counterparty.PortID, gaiaChannels[0].Counterparty.ChannelID, gaia.Config().Denom),
	).IBCDenom()

	const gaiaOsmoTxAmount = 13579 // Arbitrary amount that is easy to find in logs.
	gaiaTx, err := gaia.SendIBCTransfer(ctx, gaiaChannelID, ibctest.FaucetAccountKeyName, ibc.WalletAmount{
		Address: osmosisFaucetAddr,
		Denom:   gaia.Config().Denom,
		Amount:  gaiaOsmoTxAmount,
	}, nil)
	require.NoError(t, err)
	require.NoError(t, gaiaTx.Validate())

	afterGaiaTxHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	test.WaitForBlocks(ctx, 5, gaia, osmosis)
	afterOsmoBalance, err := osmosis.GetBalance(ctx, osmosisFaucetAddr, gaiaOsmoIBCDenom)
	require.NoError(t, err)
	require.Equal(t, afterOsmoBalance, int64(gaiaOsmoTxAmount))

	gaiaAck, err := test.PollForAck(ctx, gaia, beforeGaiaTxHeight, afterGaiaTxHeight+150, gaiaTx.Packet)
	require.NoError(t, err, "failed to get acknowledgement on gaia")
	require.NoError(t, gaiaAck.Validate(), "invalid acknowledgement on gaia")
}
