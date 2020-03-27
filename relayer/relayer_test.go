package relayer

import (
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestBasicTransfer(t *testing.T) {
	tcs := []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", gaiaTestConfig},
	}

	chains, doneFunc := spinUpTestChains(t, tcs...)
	t.Cleanup(doneFunc)

	src, dst := chains.MustGet(tcs[0].chainID), chains.MustGet(tcs[1].chainID)

	_, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	var (
		testDenom    = "samoleans"
		dstDenom     = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		expectedCoin = sdk.NewCoin(dstDenom, sdk.NewInt(1000))
	)

	// Check if clients have been created, if not create them
	err = src.CreateClients(dst)
	require.NoError(t, err)

	// Test querying clients from src and dst sides
	testClientPair(t, src, dst)

	// Check if connection has been created, if not create it
	err = src.CreateConnection(dst, src.timeout)
	require.NoError(t, err)

	// Test querying connections from src and dst sides
	testConnectionPair(t, src, dst)

	// Check if channel has been created, if not create it
	err = src.CreateChannel(dst, true, src.timeout)
	require.NoError(t, err)

	// Test querying channels from src and dst sides
	testChannelPair(t, src, dst)

	err = src.SendTransferBothSides(dst, testCoin, dst.MustGetAddress(), true)
	require.NoError(t, err)

	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, expectedCoin, sdk.NewCoin(dstDenom, dstBal.AmountOf(dstDenom)))
}

func TestRelayUnRelayedPacketsOnOrderedChannel(t *testing.T) {
	tcs := []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", gaiaTestConfig},
	}

	chains, doneFunc := spinUpTestChains(t, tcs...)
	t.Cleanup(doneFunc)

	src, dst := chains.MustGet(tcs[0].chainID), chains.MustGet(tcs[1].chainID)

	_, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	var (
		testDenom    = "samoleans"
		ibc0to1Denom = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		ibc1to0Denom = fmt.Sprintf("%s/%s/%s", src.PathEnd.PortID, src.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		dstExpected  = sdk.NewCoin(ibc0to1Denom, sdk.NewInt(2000))
		srcExpected  = sdk.NewCoin(ibc1to0Denom, sdk.NewInt(2000))
	)

	err = src.CreateClients(dst)
	require.NoError(t, err)

	err = src.CreateConnection(dst, src.timeout)
	require.NoError(t, err)

	err = src.CreateChannel(dst, true, src.timeout)
	require.NoError(t, err)

	err = src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true)
	require.NoError(t, err)

	err = src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true)
	require.NoError(t, err)

	err = dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true)
	require.NoError(t, err)

	err = dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	err = RelayUnRelayedPacketsOrderedChan(src, dst)
	require.NoError(t, err)

	srcBal, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected, sdk.NewCoin(ibc1to0Denom, srcBal.AmountOf(ibc1to0Denom)))

	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected, sdk.NewCoin(ibc0to1Denom, dstBal.AmountOf(ibc0to1Denom)))
}

func TestStreamingRelayer(t *testing.T) {
	t.Skip("TestStreamingRelayer currently failing")
	tcs := []testChain{
		{"ibc0", gaiaTestConfig},
		{"ibc1", gaiaTestConfig},
	}

	chains, doneFunc := spinUpTestChains(t, tcs...)
	t.Cleanup(doneFunc)

	src, dst := chains.MustGet(tcs[0].chainID), chains.MustGet(tcs[1].chainID)

	path, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	var (
		testDenom   = "samoleans"
		testCoin    = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		twoTestCoin = sdk.NewCoin(testDenom, sdk.NewInt(2000))
	)

	srcExpected, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	dstExpected, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)

	err = src.CreateClients(dst)
	require.NoError(t, err)

	err = src.CreateConnection(dst, src.timeout)
	require.NoError(t, err)

	err = src.CreateChannel(dst, true, src.timeout)
	require.NoError(t, err)

	err = src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true)
	require.NoError(t, err)

	err = src.SendTransferMsg(dst, testCoin, dst.MustGetAddress(), true)
	require.NoError(t, err)

	err = dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true)
	require.NoError(t, err)

	err = dst.SendTransferMsg(src, testCoin, src.MustGetAddress(), true)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	strat, err := path.GetStrategy()
	require.NoError(t, err)

	go func(t *testing.T, src, dst *Chain, strat Strategy) {
		err = strat.Run(src, dst)
		require.NoError(t, err)
	}(t, src, dst, strat)

	err = src.SendTransferMsg(dst, twoTestCoin, dst.MustGetAddress(), false)
	require.NoError(t, err)

	err = dst.SendTransferMsg(src, twoTestCoin, src.MustGetAddress(), false)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	srcGot, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected, srcGot)

	dstGot, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected, dstGot)
}
