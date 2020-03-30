package relayer

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

var (
	gaiaChains = []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", gaiaTestConfig},
	}
)

func TestBasicTransfer(t *testing.T) {
	// t.Parallel()
	chains := spinUpTestChains(t, gaiaChains...)

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
	require.NoError(t, src.CreateConnection(dst, src.timeout))
	testConnectionPair(t, src, dst)

	// Check if channel has been created, if not create it
	// then test querying channels from src and dst sides
	require.NoError(t, src.CreateChannel(dst, true, src.timeout))
	testChannelPair(t, src, dst)

	// Then send the transfer
	require.NoError(t, src.SendTransferBothSides(dst, testCoin, dst.MustGetAddress(), true))

	// ...and check the balance
	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, expectedCoin.Amount.Int64(), dstBal.AmountOf(dstDenom).Int64())

	// done()
}

func TestRelayUnRelayedPacketsOrderedChan(t *testing.T) {
	// t.Parallel()
	chains := spinUpTestChains(t, gaiaChains...)

	_, err := genTestPathAndSet(chains.MustGet("ibc0"), chains.MustGet("ibc1"), "transfer", "transfer")
	require.NoError(t, err)

	var (
		src = chains.MustGet("ibc0")
		dst = chains.MustGet("ibc1")

		testDenom    = "samoleans"
		ibc0to1Denom = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		ibc1to0Denom = fmt.Sprintf("%s/%s/%s", src.PathEnd.PortID, src.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		dstExpected  = sdk.NewCoin(ibc0to1Denom, sdk.NewInt(2000))
		srcExpected  = sdk.NewCoin(ibc1to0Denom, sdk.NewInt(2000))
	)

	// create the path
	require.NoError(t, src.CreateClients(dst))
	require.NoError(t, src.CreateConnection(dst, src.timeout))
	require.NoError(t, src.CreateChannel(dst, true, src.timeout))

	// send some src samoleans to dst
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true))
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true))

	// send some dst samoleans to src
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true))
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true))

	// wait for txs to be included
	require.NoError(t, dst.WaitForNBlocks(1))

	// relay the packets
	require.NoError(t, RelayUnRelayedPacketsOrderedChan(src, dst))

	// ...and check src bal
	srcBal, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected, sdk.NewCoin(ibc1to0Denom, srcBal.AmountOf(ibc1to0Denom)))

	// ...and check dst bal
	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected, sdk.NewCoin(ibc0to1Denom, dstBal.AmountOf(ibc0to1Denom)))
}

func TestStreamingRelayer(t *testing.T) {
	// t.Parallel()
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
	require.NoError(t, src.CreateConnection(dst, src.timeout))
	require.NoError(t, src.CreateChannel(dst, true, src.timeout))

	// send a couple of transfers to the queue on src
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true))
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true))
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.WaitForNBlocks(1))

	// start the relayer process in it's own goroutine
	rlyDone, err := path.MustGetStrategy().Run(src, dst)
	require.NoError(t, err)

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(dst, twoTestCoin, dst.MustGetAddress(), false))
	require.NoError(t, dst.SendTransferMsg(src, twoTestCoin, src.MustGetAddress(), false))

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(4))

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
}
