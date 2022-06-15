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

	channelMessages, hasChannelKey := ibcMessagesCache.ChannelHandshake[channelKey]
	require.True(t, hasChannelKey, "no messages cached for channel key")

	_, hasChannelOpenInit := channelMessages[processor.MsgChannelOpenInit]
	require.True(t, hasChannelOpenInit, "no messages cached for MsgChannelOpenInit")

	ccp.handleMsgChannelOpenAck(msgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelOpen, ok = ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.True(t, channelOpen, "channel should be marked open now")

	channelMessages, hasChannelKey = ibcMessagesCache.ChannelHandshake[channelKey]
	require.True(t, hasChannelKey, "no messages cached for channel key")

	require.Len(t, channelMessages, 2)

	_, hasChannelOpenInit = channelMessages[processor.MsgChannelOpenInit]
	require.True(t, hasChannelOpenInit, "no messages cached for MsgChannelOpenInit")

	_, hasChannelOpenAck := channelMessages[processor.MsgChannelOpenAck]
	require.True(t, hasChannelOpenAck, "no messages cached for MsgChannelOpenAck")
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

	channelMessages, hasChannelKey := ibcMessagesCache.ChannelHandshake[channelKey]
	require.True(t, hasChannelKey, "no messages cached for channel key")

	_, hasChannelOpenTry := channelMessages[processor.MsgChannelOpenTry]
	require.True(t, hasChannelOpenTry, "no messages cached for MsgChannelOpenTry")

	ccp.handleMsgChannelOpenConfirm(msgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelOpen, ok = ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.True(t, channelOpen, "channel should be marked open now")

	channelMessages, hasChannelKey = ibcMessagesCache.ChannelHandshake[channelKey]
	require.True(t, hasChannelKey, "no messages cached for channel key")

	require.Len(t, channelMessages, 2)

	_, hasChannelOpenTry = channelMessages[processor.MsgChannelOpenTry]
	require.True(t, hasChannelOpenTry, "no messages cached for MsgChannelOpenTry")

	_, hasChannelOpenConfirm := channelMessages[processor.MsgChannelOpenConfirm]
	require.True(t, hasChannelOpenConfirm, "no messages cached for MsgChannelOpenConfirm")
}
