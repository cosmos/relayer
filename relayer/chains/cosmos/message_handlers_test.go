package cosmos

import (
	"testing"

	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestConnectionStateCache(t *testing.T) {
	var (
		mockConn             = "connection-0"
		mockCounterpartyConn = "connection-1"
		msgOpenInit          = provider.ConnectionInfo{
			ConnID:             mockConn,
			CounterpartyConnID: "", // empty for MsgConnectionOpenInit, different from all other connection handshake messages.
		}
		msgOpenAck = provider.ConnectionInfo{
			ConnID:             mockConn,
			CounterpartyConnID: mockCounterpartyConn, // non-empty
		}

		// fully populated connectionKey
		k = processor.ConnectionInfoConnectionKey(msgOpenAck)
	)

	t.Run("fresh handshake", func(t *testing.T) {
		// Emulate scenario of a connection handshake occurring after the relayer is started.
		// The connectionStateCache will initially be empty for this connection. The MsgConnectionOpenInit
		// will populate the connectionStateCache with a key that has an empty counterparty connection ID.
		// The MsgConnectionOpenTry needs to replace this key with a key that has the counterparty connection ID.

		ccp := NewCosmosChainProcessor(zap.NewNop(), &CosmosProvider{}, nil)
		c := processor.NewIBCMessagesCache()

		// Observe MsgConnectionOpenInit, which does not have counterparty connection ID.
		ccp.handleConnectionMessage(conntypes.EventTypeConnectionOpenInit, msgOpenInit, c)

		require.Len(t, ccp.connectionStateCache, 1)

		// The connection state is not open, but the entry should exist in the connectionStateCache.
		// MsgInitKey returns the ConnectionKey with an empty counterparty connection ID.
		require.False(t, ccp.connectionStateCache[k.MsgInitKey()])

		// Observe MsgConnectionOpenAck, which does have counterparty connection ID.
		ccp.handleConnectionMessage(conntypes.EventTypeConnectionOpenAck, msgOpenAck, c)

		// The key with the empty counterparty connection ID should have been removed.
		// The key with the counterparty connection ID should have been added.
		require.Len(t, ccp.connectionStateCache, 1)

		// The fully populated ConnectionKey should now be the only entry for this connection.
		// The connection should now be open.
		require.True(t, ccp.connectionStateCache[k])
	})

	t.Run("handshake already occurred", func(t *testing.T) {
		// Emulate scenario of a connection handshake occurring before the relayer is started,
		// but where all connection handshake messages occurred within the initial-block-history.
		// This introduces an interesting edge condition where the connectionStateCache will be populated
		// with the open connection, including the counterparty connection ID, but then the
		// MsgConnectionOpenInit message will be observed afterwards without the counterparty connection ID.
		// We need to make sure that the connectionStateCache does not have two keys for the same connection,
		// i.e. one key with the counterparty connection ID, and one without.

		ccp := NewCosmosChainProcessor(zap.NewNop(), &CosmosProvider{}, nil)
		c := processor.NewIBCMessagesCache()

		// Initialize connectionStateCache with populated connection ID and counterparty connection ID.
		// This emulates initializeConnectionState after a recent connection handshake has completed
		ccp.connectionStateCache[k] = true

		// Observe MsgConnectionOpenInit, which does not have counterparty connection ID.
		ccp.handleConnectionMessage(conntypes.EventTypeConnectionOpenInit, msgOpenInit, c)

		// The key with the empty counterparty connection ID should not have been added.
		require.Len(t, ccp.connectionStateCache, 1)

		// The fully populated ConnectionKey should still be the only entry for this connection.
		// The connection is still marked open since it was open during initializeConnectionState.
		require.True(t, ccp.connectionStateCache[k])

		// Observe MsgConnectionOpenAck, which does have counterparty connection ID.
		ccp.handleConnectionMessage(conntypes.EventTypeConnectionOpenAck, msgOpenAck, c)

		// Number of keys should still be 1.
		require.Len(t, ccp.connectionStateCache, 1)

		// The fully populated ConnectionKey should still be the only entry for this connection.
		require.True(t, ccp.connectionStateCache[k])
	})
}

func TestChannelStateCache(t *testing.T) {
	var (
		mockChan             = "channel-0"
		mockCounterpartyChan = "channel-1"
		msgOpenInit          = provider.ChannelInfo{
			ChannelID:             mockChan,
			CounterpartyChannelID: "", // empty for MsgChannelOpenInit, different from all other channel handshake messages.
		}
		msgOpenAck = provider.ChannelInfo{
			ChannelID:             mockChan,
			CounterpartyChannelID: mockCounterpartyChan, // non-empty
		}

		// fully populated channelKey
		k = processor.ChannelInfoChannelKey(msgOpenAck)
	)

	t.Run("fresh handshake", func(t *testing.T) {
		// Emulate scenario of a channel handshake occurring after the relayer is started.
		// The channelStateCache will initially be empty for this channel. The MsgChannelOpenInit
		// will populate the channelStateCache with a key that has an empty counterparty channel ID.
		// The MsgChannelOpenTry needs to replace this key with a key that has the counterparty channel ID.

		ccp := NewCosmosChainProcessor(zap.NewNop(), &CosmosProvider{}, nil)
		c := processor.NewIBCMessagesCache()

		// Observe MsgChannelOpenInit, which does not have counterparty channel ID.
		ccp.handleChannelMessage(chantypes.EventTypeChannelOpenInit, msgOpenInit, c)

		require.Len(t, ccp.channelStateCache, 1)

		// The channel state is not open, but the entry should exist in the channelStateCache.
		// MsgInitKey returns the ChannelKey with an empty counterparty channel ID.
		require.False(t, ccp.channelStateCache[k.MsgInitKey()])

		// Observe MsgChannelOpenAck, which does have counterparty channel ID.
		ccp.handleChannelMessage(chantypes.EventTypeChannelOpenAck, msgOpenAck, c)

		// The key with the empty counterparty channel ID should have been removed.
		// The key with the counterparty channel ID should have been added.
		require.Len(t, ccp.channelStateCache, 1)

		// The fully populated ChannelKey should now be the only entry for this channel.
		// The channel now open.
		require.True(t, ccp.channelStateCache[k])
	})

	t.Run("handshake already occurred", func(t *testing.T) {
		// Emulate scenario of a channel handshake occurring before the relayer is started,
		// but where all channel handshake messages occurred within the initial-block-history.
		// This introduces an interesting edge condition where the channelStateCache will be populated
		// with the open channel, including the counterparty channel ID, but then the
		// MsgChannelOpenInit message will be observed afterwards without the counterparty channel ID.
		// We need to make sure that the channelStateCache does not have two keys for the same channel,
		// i.e. one key with the counterparty channel ID, and one without.

		ccp := NewCosmosChainProcessor(zap.NewNop(), &CosmosProvider{}, nil)
		c := processor.NewIBCMessagesCache()

		// Initialize channelStateCache with populated channel ID and counterparty channel ID.
		// This emulates initializeChannelState after a recent channel handshake has completed
		ccp.channelStateCache[k] = true

		// Observe MsgChannelOpenInit, which does not have counterparty channel ID.
		ccp.handleChannelMessage(chantypes.EventTypeChannelOpenInit, msgOpenInit, c)

		// The key with the empty counterparty channel ID should not have been added.
		require.Len(t, ccp.channelStateCache, 1)

		// The fully populated ChannelKey should still be the only entry for this channel.
		// The channel is still marked open since it was open during initializeChannelState.
		require.True(t, ccp.channelStateCache[k])

		// Observe MsgChannelOpenAck, which does have counterparty channel ID.
		ccp.handleChannelMessage(chantypes.EventTypeChannelOpenAck, msgOpenAck, c)

		// Number of keys should still be 1.
		require.Len(t, ccp.channelStateCache, 1)

		// The fully populated ChannelKey should still be the only entry for this channel.
		require.True(t, ccp.channelStateCache[k])
	})
}
