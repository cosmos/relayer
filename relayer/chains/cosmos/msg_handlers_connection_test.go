package cosmos

import (
	"testing"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
)

func TestHandleConnectionHandshake(t *testing.T) {
	const (
		srcConnection = "connection-0"
		dstConnection = "connection-1"
		srcClient     = "client-0"
		dstClient     = "client-1"
	)

	ccp := mockCosmosChainProcessor(t)

	connectionInfo := &connectionInfo{
		connectionID:             srcConnection,
		clientID:                 srcClient,
		counterpartyClientID:     dstClient,
		counterpartyConnectionID: dstConnection,
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ccp.handleMsgConnectionOpenInit(msgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionKey := connectionInfo.connectionKey()

	connectionOpen, ok := ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.False(t, connectionOpen, "connection should not be marked open yet")

	require.Len(t, ibcMessagesCache.ConnectionHandshake, 1)

	connectionMessages, hasConnectionKey := ibcMessagesCache.ConnectionHandshake[connectionKey]
	require.True(t, hasConnectionKey, "no messages cached for connection key")

	_, hasConnectionOpenInit := connectionMessages[processor.MsgConnectionOpenInit]
	require.True(t, hasConnectionOpenInit, "no messages cached for MsgConnectionOpenInit")

	ccp.handleMsgConnectionOpenAck(msgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionOpen, ok = ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.True(t, connectionOpen, "connection should be marked open now")

	connectionMessages, hasConnectionKey = ibcMessagesCache.ConnectionHandshake[connectionKey]
	require.True(t, hasConnectionKey, "no messages cached for connection key")

	require.Len(t, connectionMessages, 2)

	_, hasConnectionOpenInit = connectionMessages[processor.MsgConnectionOpenInit]
	require.True(t, hasConnectionOpenInit, "no messages cached for MsgConnectionOpenInit")

	_, hasConnectionOpenAck := connectionMessages[processor.MsgConnectionOpenAck]
	require.True(t, hasConnectionOpenAck, "no messages cached for MsgConnectionOpenAck")
}

func TestHandleConnectionHandshakeCounterparty(t *testing.T) {
	const (
		srcConnection = "connection-0"
		dstConnection = "connection-1"
		srcClient     = "client-0"
		dstClient     = "client-1"
	)

	ccp := mockCosmosChainProcessor(t)

	connectionInfo := &connectionInfo{
		connectionID:             srcConnection,
		clientID:                 srcClient,
		counterpartyClientID:     dstClient,
		counterpartyConnectionID: dstConnection,
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ccp.handleMsgConnectionOpenTry(msgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionKey := connectionInfo.connectionKey().Counterparty()

	connectionOpen, ok := ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.False(t, connectionOpen, "connection should not be marked open yet")

	require.Len(t, ibcMessagesCache.ConnectionHandshake, 1)

	connectionMessages, hasConnectionKey := ibcMessagesCache.ConnectionHandshake[connectionKey]
	require.True(t, hasConnectionKey, "no messages cached for connection key")

	_, hasConnectionOpenTry := connectionMessages[processor.MsgConnectionOpenTry]
	require.True(t, hasConnectionOpenTry, "no messages cached for MsgConnectionOpenTry")

	ccp.handleMsgConnectionOpenConfirm(msgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionOpen, ok = ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.True(t, connectionOpen, "connection should be marked open now")

	connectionMessages, hasConnectionKey = ibcMessagesCache.ConnectionHandshake[connectionKey]
	require.True(t, hasConnectionKey, "no messages cached for connection key")

	require.Len(t, connectionMessages, 2)

	_, hasConnectionOpenTry = connectionMessages[processor.MsgConnectionOpenTry]
	require.True(t, hasConnectionOpenTry, "no messages cached for MsgConnectionOpenTry")

	_, hasConnectionOpenConfirm := connectionMessages[processor.MsgConnectionOpenConfirm]
	require.True(t, hasConnectionOpenConfirm, "no messages cached for MsgConnectionOpenConfirm")
}
