package test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/stretchr/testify/require"
)

var (
	gaiaChains = []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", gaiaTestConfig},
	}
)

func TestGaiaToGaiaStreamingRelayer(t *testing.T) {
	chains := spinUpTestChains(t, gaiaChains...)

	var (
		src         = chains.MustGet("ibc0")
		dst         = chains.MustGet("ibc1")
		testDenom   = "samoleans"
		testCoin    = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		twoTestCoin = sdk.NewCoin(testDenom, sdk.NewInt(2000))
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
	testClientPair(t, src, dst)
	require.NoError(t, src.CreateConnection(dst, src.GetTimeout()))
	testConnectionPair(t, src, dst)
	require.NoError(t, src.CreateChannel(dst, true, src.GetTimeout()))
	testChannelPair(t, src, dst)

	// send a couple of transfers to the queue on src
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true))
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true))
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.WaitForNBlocks(1))

	// start the relayer process in it's own goroutine
	rlyDone, err := relayer.RunStrategy(src, dst, path.MustGetStrategy(), path.Ordered())
	require.NoError(t, err)

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(dst, twoTestCoin, dst.MustGetAddress(), false))
	require.NoError(t, dst.SendTransferMsg(src, twoTestCoin, src.MustGetAddress(), false))

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(6))

	// kill relayer routine
	rlyDone()

	// check balance on src against expected
	srcGot, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64(), srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64(), dstGot.AmountOf(testDenom).Int64())

	// Test the full transfer command as well
	require.NoError(t, src.SendTransferBothSides(dst, testCoin, dst.MustGetAddress(), true))
	require.NoError(t, dst.SendTransferBothSides(src, testCoin, src.MustGetAddress(), false))

	// check balance on src against expected
	srcGot, err = src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64(), srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err = dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64(), dstGot.AmountOf(testDenom).Int64())
}
