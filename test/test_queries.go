package test

import (
	"strings"
	"testing"

	. "github.com/iqlusioninc/relayer/relayer"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
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
	require.Equal(t, len(clients), 1)
	require.Equal(t, clients[0].GetID(), src.PathEnd.ClientID)

	client, err := src.QueryClientState()
	require.NoError(t, err)
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
	require.Equal(t, conns[0].GetState().String(), "OPEN")

	h, err := src.Client.Status()
	require.NoError(t, err)

	conn, err := src.QueryConnection(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, conn.Connection.GetClientID(), src.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conn.Connection.GetState().String(), "OPEN")
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
	require.Equal(t, chans[0].GetOrdering().String(), "ORDERED")
	require.Equal(t, chans[0].GetState().String(), "OPEN")
	require.Equal(t, chans[0].GetCounterparty().GetChannelID(), dst.PathEnd.ChannelID)
	require.Equal(t, chans[0].GetCounterparty().GetPortID(), dst.PathEnd.PortID)

	h, err := src.Client.Status()
	require.NoError(t, err)

	ch, err := src.QueryChannel(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, ch.Channel.GetOrdering().String(), "ORDERED")
	require.Equal(t, ch.Channel.GetState().String(), "OPEN")
	require.Equal(t, ch.Channel.GetCounterparty().GetChannelID(), dst.PathEnd.ChannelID)
	require.Equal(t, ch.Channel.GetCounterparty().GetPortID(), dst.PathEnd.PortID)
}

// for the love of logs https://www.youtube.com/watch?v=DtsKcHmceqY
func getLoggingChain(chns []*Chain, rsr *dockertest.Resource) *Chain {
	for _, c := range chns {
		if strings.Contains(rsr.Container.Name, c.ChainID) {
			return c
		}
	}
	return nil
}

func genTestPathAndSet(src, dst *Chain, srcPort, dstPort string) (*Path, error) {
	path := GenPath(src.ChainID, dst.ChainID, srcPort, dstPort)
	if err := src.SetPath(path.Src); err != nil {
		return nil, err
	}
	if err := dst.SetPath(path.Dst); err != nil {
		return nil, err
	}
	return path, nil
}
