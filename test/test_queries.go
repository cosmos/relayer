package test

import (
	"testing"
	"time"

	"github.com/cosmos/relayer/relayer"

	"github.com/avast/retry-go"
	clientypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/stretchr/testify/require"
)

// testClientPair tests that the client for src on dst and dst on src are the only clients on those chains
func testClientPair(t *testing.T, src, dst *relayer.Chain) {
	testClient(t, src)
	testClient(t, dst)
}

// testClient queries client for existence of dst on src
func testClient(t *testing.T, src *relayer.Chain) {
	srch, err := src.ChainProvider.QueryLatestHeight()
	require.NoError(t, err)
	var (
		client *clientypes.QueryClientStateResponse
	)
	if err = retry.Do(func() error {
		client, err = src.ChainProvider.QueryClientStateResponse(srch, src.ClientID())
		if err != nil {
			srch, _ = src.ChainProvider.QueryLatestHeight()
		}
		return err
	}); err != nil {
		return
	}
	require.NoError(t, err)
	require.NotNil(t, client)
	cs, err := clientypes.UnpackClientState(client.ClientState)
	require.NoError(t, err)
	require.Equal(t, cs.ClientType(), "07-tendermint")
}

// testConnectionPair tests that the only connection on src and dst is between the two chains
func testConnectionPair(t *testing.T, src, dst *relayer.Chain) {
	testConnection(t, src, dst)
	testConnection(t, dst, src)
}

// testConnection tests that the only connection on src has a counterparty that is the connection on dst
func testConnection(t *testing.T, src, dst *relayer.Chain) {
	conns, err := src.ChainProvider.QueryConnections()
	require.NoError(t, err)
	require.Equal(t, len(conns), 1)
	require.Equal(t, conns[0].ClientId, src.PathEnd.ClientID)
	require.Equal(t, conns[0].Counterparty.GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conns[0].Counterparty.GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conns[0].State.String(), "STATE_OPEN")

	h, err := src.ChainProvider.QueryLatestHeight()
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	conn, err := src.ChainProvider.QueryConnection(h, src.ConnectionID())
	require.NoError(t, err)
	require.Equal(t, conn.Connection.ClientId, src.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conn.Connection.State.String(), "STATE_OPEN")
}

// testChannelPair tests that the only channel on src and dst is between the two chains
func testChannelPair(t *testing.T, src, dst *relayer.Chain, channelID, portID string) {
	testChannel(t, src, dst, channelID, portID)
	testChannel(t, dst, src, channelID, portID)
}

// testChannel tests that the only channel on src is a counterparty of dst
func testChannel(t *testing.T, src, dst *relayer.Chain, channelID, portID string) {
	chans, err := src.ChainProvider.QueryChannels()
	require.NoError(t, err)
	require.Equal(t, 1, len(chans))
	require.Equal(t, chans[0].Ordering.String(), "ORDER_UNORDERED")
	require.Equal(t, chans[0].State.String(), "STATE_OPEN")
	require.Equal(t, chans[0].Counterparty.ChannelId, channelID)
	require.Equal(t, chans[0].Counterparty.GetPortID(), portID)

	h, err := src.ChainProvider.QueryLatestHeight()
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	ch, err := src.ChainProvider.QueryChannel(h, channelID, portID)
	require.NoError(t, err)
	require.Equal(t, ch.Channel.Ordering.String(), "ORDER_UNORDERED")
	require.Equal(t, ch.Channel.State.String(), "STATE_OPEN")
	require.Equal(t, ch.Channel.Counterparty.ChannelId, channelID)
	require.Equal(t, ch.Channel.Counterparty.GetPortID(), portID)
}
