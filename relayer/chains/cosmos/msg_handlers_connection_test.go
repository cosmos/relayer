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

	openInitMessages, hasConnectionOpenInit := ibcMessagesCache.ConnectionHandshake[processor.MsgConnectionOpenInit]
	require.True(t, hasConnectionOpenInit, "no messages cached for MsgConnectionOpenInit")

	openInitMessage, hasOpenInitMessage := openInitMessages[connectionKey]
	require.True(t, hasOpenInitMessage, "no init messages cached for connection key")

	require.NotNil(t, openInitMessage)

	ccp.handleMsgConnectionOpenAck(msgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionOpen, ok = ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.True(t, connectionOpen, "connection should be marked open now")

	openInitMessages, hasConnectionOpenInit = ibcMessagesCache.ConnectionHandshake[processor.MsgConnectionOpenInit]
	require.True(t, hasConnectionOpenInit, "no messages cached for MsgConnectionOpenInit")

	openAckMessages, hasConnectionOpenAck := ibcMessagesCache.ConnectionHandshake[processor.MsgConnectionOpenAck]
	require.True(t, hasConnectionOpenAck, "no messages cached for MsgConnectionOpenAck")

	openInitMessage, hasOpenInitMessage = openInitMessages[connectionKey]
	require.True(t, hasOpenInitMessage, "no init messages cached for connection key")

	require.NotNil(t, openInitMessage)

	openAckMessage, hasOpenAckMessage := openAckMessages[connectionKey]
	require.True(t, hasOpenAckMessage, "no ack messages cached for connection key")

	require.NotNil(t, openAckMessage)

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

	openTryMessages, hasConnectionOpenTry := ibcMessagesCache.ConnectionHandshake[processor.MsgConnectionOpenTry]
	require.True(t, hasConnectionOpenTry, "no messages cached for MsgConnectionOpenTry")

	openTryMessage, hasOpenTryMessage := openTryMessages[connectionKey]
	require.True(t, hasOpenTryMessage, "no messages cached for connection key")

	require.NotNil(t, openTryMessage)

	ccp.handleMsgConnectionOpenConfirm(msgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionOpen, ok = ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.True(t, connectionOpen, "connection should be marked open now")

	openTryMessages, hasConnectionOpenTry = ibcMessagesCache.ConnectionHandshake[processor.MsgConnectionOpenTry]
	require.True(t, hasConnectionOpenTry, "no messages cached for MsgConnectionOpenTry")

	openConfirmMessages, hasConnectionOpenConfirm := ibcMessagesCache.ConnectionHandshake[processor.MsgConnectionOpenConfirm]
	require.True(t, hasConnectionOpenConfirm, "no messages cached for MsgConnectionOpenConfirm")

	openTryMessage, hasOpenTryMessage = openTryMessages[connectionKey]
	require.True(t, hasOpenTryMessage, "no open try messages cached for connection key")

	require.NotNil(t, openTryMessage)

	openConfirmMessage, hasOpenConfirmMessage := openConfirmMessages[connectionKey]
	require.True(t, hasOpenConfirmMessage, "no open confirm messages cached for connection key")

	require.Nil(t, openConfirmMessage)
}
