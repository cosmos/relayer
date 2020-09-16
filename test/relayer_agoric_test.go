package test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ovrclk/relayer/relayer"
	"github.com/stretchr/testify/require"
)

var (
	agoricChains = []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", agoricTestConfig},
	}
)

func TestAgoricToGaiaStreaming(t *testing.T) {
	chains := spinUpTestChains(t, agoricChains...)

	var (
		src            = chains.MustGet("ibc0")
		dst            = chains.MustGet("ibc1")
		srcDenom       = "samoleans"
		dstDenom       = "uagstake"
		srcTestCoin    = sdk.NewCoin(srcDenom, sdk.NewInt(1000))
		dstTestCoin    = sdk.NewCoin(dstDenom, sdk.NewInt(1000))
		twoSrcTestCoin = sdk.NewCoin(srcDenom, sdk.NewInt(2000))
		twoDstTestCoin = sdk.NewCoin(dstDenom, sdk.NewInt(2000))
	)

	path, err := genTestPathAndSet(src, dst, "transfer", "transfer") // FIGME: "relayertesttransfer")
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
	require.NoError(t, src.SendTransferMsg(dst, srcTestCoin, dst.MustGetAddress()))
	require.NoError(t, src.SendTransferMsg(dst, srcTestCoin, dst.MustGetAddress()))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(src, dstTestCoin, src.MustGetAddress()))
	require.NoError(t, dst.SendTransferMsg(src, dstTestCoin, src.MustGetAddress()))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.WaitForNBlocks(1))

	// start the relayer process in it's own goroutine
	rlyDone, err := relayer.RunStrategy(src, dst, path.MustGetStrategy(), path.Ordered())
	require.NoError(t, err)

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(dst, twoDstTestCoin, dst.MustGetAddress()))
	require.NoError(t, dst.SendTransferMsg(src, twoSrcTestCoin, src.MustGetAddress()))

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(4))

	// kill relayer routine
	rlyDone()

	// check balance on src against expected
	srcGot, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(srcDenom).Int64(), srcGot.AmountOf(srcDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(dstDenom).Int64(), dstGot.AmountOf(dstDenom).Int64())

	// check balance on src against expected
	srcGot, err = src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(srcDenom).Int64(), srcGot.AmountOf(srcDenom).Int64())

	// check balance on dst against expected
	dstGot, err = dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(dstDenom).Int64(), dstGot.AmountOf(dstDenom).Int64())
}
