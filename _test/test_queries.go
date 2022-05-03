package test

import (
	"context"
	"testing"
	"time"

	"github.com/avast/retry-go/v4"
	clientypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/stretchr/testify/require"
)

// testClientPair tests that the client for src on dst and dst on src are the only clients on those chains
func testClientPair(ctx context.Context, t *testing.T, src, dst *relayer.Chain) {
	t.Helper()
	testClient(ctx, t, src)
	testClient(ctx, t, dst)
}

// testClient queries client for existence of dst on src
func testClient(ctx context.Context, t *testing.T, src *relayer.Chain) {
	t.Helper()

	srch, err := src.ChainProvider.QueryLatestHeight(ctx)
	require.NoError(t, err)

	var client *clientypes.QueryClientStateResponse
	require.NoError(t, retry.Do(func() error {
		var err error
		client, err = src.ChainProvider.QueryClientStateResponse(ctx, srch, src.ClientID())
		if err != nil {
			srch, _ = src.ChainProvider.QueryLatestHeight(ctx)
			return err
		}

		return nil
	}))

	require.NotNil(t, client)

	cs, err := clientypes.UnpackClientState(client.ClientState)
	require.NoError(t, err)
	require.Equal(t, cs.ClientType(), "07-tendermint") // TODO remove this check or create separate test for substrate
}

// testConnectionPair tests that the only connection on src and dst is between the two chains
func testConnectionPair(ctx context.Context, t *testing.T, src, dst *relayer.Chain) {
	t.Helper()
	testConnection(ctx, t, src, dst)
	testConnection(ctx, t, dst, src)
}

// testConnection tests that the only connection on src has a counterparty that is the connection on dst
func testConnection(ctx context.Context, t *testing.T, src, dst *relayer.Chain) {
	t.Helper()

	conns, err := src.ChainProvider.QueryConnections(ctx)
	require.NoError(t, err)
	require.Equal(t, len(conns), 1)
	require.Equal(t, conns[0].ClientId, src.PathEnd.ClientID)
	require.Equal(t, conns[0].Counterparty.GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conns[0].Counterparty.GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conns[0].State.String(), "STATE_OPEN")

	h, err := src.ChainProvider.QueryLatestHeight(ctx)
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	conn, err := src.ChainProvider.QueryConnection(ctx, h, src.ConnectionID())
	require.NoError(t, err)
	require.Equal(t, conn.Connection.ClientId, src.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conn.Connection.State.String(), "STATE_OPEN")
}

// testChannelPair tests that the only channel on src and dst is between the two chains
func testChannelPair(ctx context.Context, t *testing.T, src, dst *relayer.Chain, channelID, portID string) {
	t.Helper()
	testChannel(ctx, t, src, dst, channelID, portID)
	testChannel(ctx, t, dst, src, channelID, portID)
}

// testChannel tests that the only channel on src is a counterparty of dst
func testChannel(ctx context.Context, t *testing.T, src, dst *relayer.Chain, channelID, portID string) {
	t.Helper()

	chans, err := src.ChainProvider.QueryChannels(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(chans))
	require.Equal(t, chans[0].Ordering.String(), "ORDER_UNORDERED")
	require.Equal(t, chans[0].State.String(), "STATE_OPEN")
	require.Equal(t, chans[0].Counterparty.ChannelId, channelID)
	require.Equal(t, chans[0].Counterparty.GetPortID(), portID)

	h, err := src.ChainProvider.QueryLatestHeight(ctx)
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	ch, err := src.ChainProvider.QueryChannel(ctx, h, channelID, portID)
	require.NoError(t, err)
	require.Equal(t, ch.Channel.Ordering.String(), "ORDER_UNORDERED")
	require.Equal(t, ch.Channel.State.String(), "STATE_OPEN")
	require.Equal(t, ch.Channel.Counterparty.ChannelId, channelID)
	require.Equal(t, ch.Channel.Counterparty.GetPortID(), portID)
}
