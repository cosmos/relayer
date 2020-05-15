package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/iqlusioninc/relayer/relayer"
)

// testClientPair tests that the client for src on dst and dst on src are the only clients on those chains
func testClientPair(t *testing.T, src, dst *Chain) {
	testClient(t, src, dst)
	testClient(t, dst, src)
}

// testClient queries clients and client for dst on src and returns a variety of errors
// testClient expects just one client on src, that for dst
// TODO: we should be able to find the chain id of dst on src, add a case for this in each switch
func testClient(t *testing.T, src, dst *Chain) {
	clients, err := src.QueryClients(1, 1000)
	require.NoError(t, err)
	require.Equal(t, len(clients), 2)
	require.Equal(t, clients[0].GetID(), src.PathEnd.ClientID)
	client, err := src.QueryClientState()
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, client.ClientState.GetID(), src.PathEnd.ClientID)
	require.Equal(t, client.ClientState.ClientType().String(), "tendermint")
}

// testConnectionPair tests that the only connection on src and dst is between the two chains
func testConnectionPair(t *testing.T, src, dst *Chain) {
	testConnection(t, src, dst)
	testConnection(t, dst, src)
}

// testConnection tests that the only connection on src has a counterparty that is the connection on dst
func testConnection(t *testing.T, src, dst *Chain) {
	conns, err := src.QueryConnections(1, 1000)
	require.NoError(t, err)
	require.Equal(t, len(conns), 1)
	require.Equal(t, conns[0].GetClientID(), src.PathEnd.ClientID)
	require.Equal(t, conns[0].GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conns[0].GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conns[0].GetState().String(), "STATE_OPEN")

	h, err := src.Client.Status()
	require.NoError(t, err)

	conn, err := src.QueryConnection(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, conn.Connection.GetClientID(), src.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conn.Connection.GetState().String(), "STATE_OPEN")
}

// testChannelPair tests that the only channel on src and dst is between the two chains
func testChannelPair(t *testing.T, src, dst *Chain) {
	testChannel(t, src, dst)
	testChannel(t, dst, src)
}

// testChannel tests that the only channel on src is a counterparty of dst
func testChannel(t *testing.T, src, dst *Chain) {
	chans, err := src.QueryChannels(1, 1000)
	require.NoError(t, err)
	require.Equal(t, 1, len(chans))
	require.Equal(t, chans[0].Ordering.String(), "ORDER_ORDERED")
	require.Equal(t, chans[0].State.String(), "STATE_OPEN")
	require.Equal(t, chans[0].Counterparty.ChannelID, dst.PathEnd.ChannelID)
	require.Equal(t, chans[0].Counterparty.GetPortID(), dst.PathEnd.PortID)

	h, err := src.Client.Status()
	require.NoError(t, err)

	ch, err := src.QueryChannel(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, ch.Channel.Ordering.String(), "ORDER_ORDERED")
	require.Equal(t, ch.Channel.State.String(), "STATE_OPEN")
	require.Equal(t, ch.Channel.Counterparty.ChannelID, dst.PathEnd.ChannelID)
	require.Equal(t, ch.Channel.Counterparty.GetPortID(), dst.PathEnd.PortID)
}
