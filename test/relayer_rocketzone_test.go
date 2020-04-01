package test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

var (
	rocketChains = []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", rocketTestConfig},
	}
)

func TestRocket_GaiaToRocketBasicTransfer(t *testing.T) {
	chains := spinUpTestChains(t, rocketChains...)

	_, err := genTestPathAndSet(chains.MustGet("ibc0"), chains.MustGet("ibc1"), "transfer", "transfer")
	require.NoError(t, err)

	var (
		src          = chains.MustGet("ibc0")
		dst          = chains.MustGet("ibc1")
		testDenom    = "samoleans"
		dstDenom     = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		expectedCoin = sdk.NewCoin(dstDenom, sdk.NewInt(1000))
	)

	// Check if clients have been created, if not create them
	// then test querying clients from src and dst sides
	require.NoError(t, src.CreateClients(dst))
	testClientPair(t, src, dst)

	// Check if connection has been created, if not create it
	// the test querying connections from src and dst sides
	require.NoError(t, src.CreateConnection(dst, src.GetTimeout()))
	testConnectionPair(t, src, dst)

	// Check if channel has been created, if not create it
	// then test querying channels from src and dst sides
	require.NoError(t, src.CreateChannel(dst, true, src.GetTimeout()))
	testChannelPair(t, src, dst)

	// Then send the transfer
	require.NoError(t, src.SendTransferBothSides(dst, testCoin, dst.MustGetAddress(), true))

	// ...and check the balance
	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, expectedCoin.Amount.Int64(), dstBal.AmountOf(dstDenom).Int64())
}

func TestRocket_RocketToGaiaBasicTransfer(t *testing.T) {
	chains := spinUpTestChains(t, rocketChains...)

	_, err := genTestPathAndSet(chains.MustGet("ibc0"), chains.MustGet("ibc1"), "transfer", "transfer")
	require.NoError(t, err)

	var (
		src          = chains.MustGet("ibc1")
		dst          = chains.MustGet("ibc0")
		testDenom    = "ufuel"
		dstDenom     = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		expectedCoin = sdk.NewCoin(dstDenom, sdk.NewInt(1000))
	)

	// Check if clients have been created, if not create them
	// then test querying clients from src and dst sides
	require.NoError(t, src.CreateClients(dst))
	testClientPair(t, src, dst)

	// Check if connection has been created, if not create it
	// the test querying connections from src and dst sides
	require.NoError(t, src.CreateConnection(dst, src.GetTimeout()))
	testConnectionPair(t, src, dst)

	// Check if channel has been created, if not create it
	// then test querying channels from src and dst sides
	require.NoError(t, src.CreateChannel(dst, true, src.GetTimeout()))
	testChannelPair(t, src, dst)

	// Then send the transfer
	require.NoError(t, src.SendTransferBothSides(dst, testCoin, dst.MustGetAddress(), true))

	// ...and check the balance
	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, expectedCoin.Amount.Int64(), dstBal.AmountOf(dstDenom).Int64())
}
