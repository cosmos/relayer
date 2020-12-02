package test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/stretchr/testify/require"
)

var (
	akashChains = []testChain{
		{"ibc-0", gaiaTestConfig},
		{"ibc-1", akashTestConfig},
	}
)

func TestAkashToGaiaStreamingRelayer(t *testing.T) {
	chains := spinUpTestChains(t, akashChains...)

	var (
		src         = chains.MustGet("ibc-0")
		dst         = chains.MustGet("ibc-1")
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
	require.NoError(t, src.CreateOpenConnections(dst, 3, src.GetTimeout()))
	testConnectionPair(t, src, dst)
	require.NoError(t, src.CreateOpenChannels(dst, 3, src.GetTimeout()))
	testChannelPair(t, src, dst)

	// send a couple of transfers to the queue on src
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), 0, 0))
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), 0, 0))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), 0, 0))
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), 0, 0))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.WaitForNBlocks(1))

	// start the relayer process in it's own goroutine
	rlyDone, err := relayer.RunStrategy(src, dst, path.MustGetStrategy())
	require.NoError(t, err)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.WaitForNBlocks(1))
	require.NoError(t, dst.WaitForNBlocks(1))

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(dst, twoTestCoin, dst.MustGetAddress(), 0, 0))
	require.NoError(t, dst.SendTransferMsg(src, twoTestCoin, src.MustGetAddress(), 0, 0))

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(6))

	// kill relayer routine
	rlyDone()

	// check balance on src against expected
	srcGot, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-4000, srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-4000, dstGot.AmountOf(testDenom).Int64())

	// check balance on src against expected
	srcGot, err = src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-4000, srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err = dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-4000, dstGot.AmountOf(testDenom).Int64())
}
