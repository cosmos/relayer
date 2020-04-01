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

func TestRocket_GaiaToRocketStreamingRelayer(t *testing.T) {
	chains := spinUpTestChains(t, rocketChains...)

	var (
		src            = chains.MustGet("ibc0")
		dst            = chains.MustGet("ibc1")
		testDenomSrc   = "samoleans"
		testDenomDst   = "nitro"
		testCoinSrc    = sdk.NewCoin(testDenomSrc, sdk.NewInt(1000))
		twoTestCoinSrc = sdk.NewCoin(testDenomSrc, sdk.NewInt(2000))
		testCoinDst    = sdk.NewCoin(testDenomDst, sdk.NewInt(1000))
		twoTestCoinDst = sdk.NewCoin(testDenomDst, sdk.NewInt(2000))
	)

	path, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	// query initial balances to compare against at the end
	srcExpected, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	dstExpected, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)

	// create path
	require.NoError(t, src.CreateClients(dst))
	require.NoError(t, src.CreateConnection(dst, src.GetTimeout()))
	require.NoError(t, src.CreateChannel(dst, true, src.GetTimeout()))

	// send a couple of transfers to the queue on src
	require.NoError(t, src.SendTransferMsg(dst, testCoinSrc, dst.MustGetAddress(), true))
	require.NoError(t, src.SendTransferMsg(dst, testCoinSrc, dst.MustGetAddress(), true))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(src, testCoinDst, src.MustGetAddress(), true))
	require.NoError(t, dst.SendTransferMsg(src, testCoinDst, src.MustGetAddress(), true))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.WaitForNBlocks(1))

	// start the relayer process in it's own goroutine
	rlyDone, err := path.MustGetStrategy().Run(src, dst)
	require.NoError(t, err)

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(dst, twoTestCoinDst, dst.MustGetAddress(), false))
	require.NoError(t, dst.SendTransferMsg(src, twoTestCoinSrc, src.MustGetAddress(), false))

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(4))

	// kill relayer routine
	rlyDone()

	// check balance on src against expected
	srcGot, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenomSrc).Int64(), srcGot.AmountOf(testDenomSrc).Int64())

	// check balance on dst against expected
	dstGot, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenomDst).Int64(), dstGot.AmountOf(testDenomDst).Int64())
}
