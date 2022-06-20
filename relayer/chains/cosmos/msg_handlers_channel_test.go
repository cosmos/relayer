package cosmos

import (
	"testing"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
)

func TestHandleChannelHandshake(t *testing.T) {
	const (
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)

	ccp := mockCosmosChainProcessor(t)

	channelInfo := &channelInfo{
		channelID:             srcChannel,
		portID:                srcPort,
		counterpartyChannelID: dstChannel,
		counterpartyPortID:    dstPort,
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ccp.handleMsgChannelOpenInit(msgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelKey := channelInfo.channelKey()

	channelOpen, ok := ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.False(t, channelOpen, "channel should not be marked open yet")

	require.Len(t, ibcMessagesCache.ChannelHandshake, 1)

	openInitMessages, hasChannelOpenInit := ibcMessagesCache.ChannelHandshake[processor.MsgChannelOpenInit]
	require.True(t, hasChannelOpenInit, "no messages cached for MsgChannelOpenInit")

	openInitMessage, hasOpenInitMessage := openInitMessages[channelKey]
	require.True(t, hasOpenInitMessage, "no open init messages cached for channel key")

	require.NotNil(t, openInitMessage)

	ccp.handleMsgChannelOpenAck(msgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelOpen, ok = ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.True(t, channelOpen, "channel should be marked open now")

	openInitMessages, hasChannelOpenInit = ibcMessagesCache.ChannelHandshake[processor.MsgChannelOpenInit]
	require.True(t, hasChannelOpenInit, "no messages cached for MsgChannelOpenInit")

	openAckMessages, hasChannelOpenAck := ibcMessagesCache.ChannelHandshake[processor.MsgChannelOpenAck]
	require.True(t, hasChannelOpenAck, "no messages cached for MsgChannelOpenAck")

	openInitMessage, hasOpenInitMessage = openInitMessages[channelKey]
	require.True(t, hasOpenInitMessage, "no open init messages cached for channel key")

	require.NotNil(t, openInitMessage)

	openAckMessage, hasOpenAckMessage := openAckMessages[channelKey]
	require.True(t, hasOpenAckMessage, "no open ack messages cached for channel key")

	require.NotNil(t, openAckMessage)
}

func TestHandleChannelHandshakeCounterparty(t *testing.T) {
	const (
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)

	ccp := mockCosmosChainProcessor(t)

	channelInfo := &channelInfo{
		channelID:             srcChannel,
		portID:                srcPort,
		counterpartyChannelID: dstChannel,
		counterpartyPortID:    dstPort,
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ccp.handleMsgChannelOpenTry(msgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelKey := channelInfo.channelKey().Counterparty()

	channelOpen, ok := ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.False(t, channelOpen, "channel should not be marked open yet")

	require.Len(t, ibcMessagesCache.ChannelHandshake, 1)

	openTryMessages, hasChannelOpenTry := ibcMessagesCache.ChannelHandshake[processor.MsgChannelOpenTry]
	require.True(t, hasChannelOpenTry, "no messages cached for MsgChannelOpenTry")

	openTryMessage, hasOpenTryMessage := openTryMessages[channelKey]
	require.True(t, hasOpenTryMessage, "no open try messages cached for channel key")

	require.NotNil(t, openTryMessage)

	ccp.handleMsgChannelOpenConfirm(msgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelOpen, ok = ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.True(t, channelOpen, "channel should be marked open now")

	openTryMessages, hasChannelOpenTry = ibcMessagesCache.ChannelHandshake[processor.MsgChannelOpenTry]
	require.True(t, hasChannelOpenTry, "no messages cached for MsgChannelOpenTry")

	openConfirmMessages, hasChannelOpenConfirm := ibcMessagesCache.ChannelHandshake[processor.MsgChannelOpenConfirm]
	require.True(t, hasChannelOpenConfirm, "no messages cached for MsgChannelOpenConfirm")

	openTryMessage, hasOpenTryMessage = openTryMessages[channelKey]
	require.True(t, hasOpenTryMessage, "no open try messages cached for channel key")

	require.NotNil(t, openTryMessage)

	openConfirmMessage, hasOpenConfirmMessage := openConfirmMessages[channelKey]
	require.True(t, hasOpenConfirmMessage, "no open confirm messages cached for channel key")

	require.Nil(t, openConfirmMessage)

}
