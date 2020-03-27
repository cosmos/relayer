package relayer

import (
	"fmt"
	"testing"

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
		dstDenom     = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		expectedCoin = sdk.NewCoin(dstDenom, sdk.NewInt(2000))
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

	err = RelayUnRelayedPacketsOrderedChan(src, dst)
	require.NoError(t, err)

	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, expectedCoin, dstBal)
}
