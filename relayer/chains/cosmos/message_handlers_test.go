package cosmos

import (
	"os"
	"testing"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func mockCosmosChainProcessor(t *testing.T) *CosmosChainProcessor {
	const (
		chainID1 = "test-chain-1"
		chainID2 = "test-chain-2"
	)
	var (
		pathEnd1      = processor.PathEnd{ChainID: chainID1}
		pathEnd2      = processor.PathEnd{ChainID: chainID2}
		log           = zap.NewNop()
		pathProcessor = processor.NewPathProcessor(log, pathEnd1, pathEnd2)
		provider      = cosmos.CosmosProvider{PCfg: cosmos.CosmosProviderConfig{ChainID: chainID1}}
	)

	ccp, err := NewCosmosChainProcessor(log, &provider, "", os.Stdin, os.Stdout, []*processor.PathProcessor{pathProcessor})
	require.NoError(t, err, "error constructing cosmos chain processor")

	applicable := pathProcessor.SetChainProviderIfApplicable(&provider)
	require.True(t, applicable, "error setting path processor reference to chain processor")

	return ccp
}

func TestHandleMsgTransfer(t *testing.T) {
	const (
		sequence         = uint64(1)
		srcChannel       = "channel-0"
		dstChannel       = "channel-1"
		srcPort          = "transfer"
		dstPort          = "transfer"
		timeoutHeight    = 100
		timeoutRevision  = 1
		timeoutTimestamp = uint64(2054566111724000000)
	)
	var (
		packetData = []byte{0x1, 0x2, 0x3, 0x4}
		ccp        = mockCosmosChainProcessor(t)
	)

	ibcMessagesCache := processor.NewIBCMessagesCache()

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
			TimeoutHeight: clienttypes.Height{
				RevisionHeight: timeoutHeight,
				RevisionNumber: timeoutRevision,
			},
			TimeoutTimestamp: timeoutTimestamp,
		},
	}

	ccp.handleMsgTransfer(MsgHandlerParams{messageInfo: packetInfo, ibcMessagesCache: ibcMessagesCache})

	require.Len(t, ibcMessagesCache.PacketFlow, 1)

	channelKey := packetInfo.channelKey()

	channelMessages, ok := ibcMessagesCache.PacketFlow[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgTransfer]
	require.True(t, ok, "unable to find messages for MsgTransfer type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	cosmosMsg := cosmos.CosmosMsg(sequenceMessage)
	require.NotNil(t, cosmosMsg, "error parsing message as CosmosMsg")

	msgRecvPacket, ok := cosmosMsg.(*chantypes.MsgRecvPacket)
	require.True(t, ok, "unable to read message as MsgRecvPacket")

	require.Empty(t, cmp.Diff(packetInfo.packet, msgRecvPacket.Packet), "MsgRecvPacket data does not match MsgTransfer data")
}

func TestHandleMsgRecvPacket(t *testing.T) {
	const (
		sequence   = uint64(1)
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData = []byte{0x1, 0x2, 0x3, 0x4}
		packetAck  = []byte{0x2, 0x3, 0x4, 0x5}
		ccp        = mockCosmosChainProcessor(t)
	)

	ibcMessagesCache := processor.NewIBCMessagesCache()

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
		ack: packetAck,
	}

	ccp.handleMsgRecvPacket(MsgHandlerParams{messageInfo: packetInfo, ibcMessagesCache: ibcMessagesCache})

	require.Len(t, ibcMessagesCache.PacketFlow, 1)

	// flipped on purpose since MsgRecvPacket is committed on counterparty chain
	channelKey := packetInfo.channelKey().Counterparty()

	channelMessages, ok := ibcMessagesCache.PacketFlow[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgRecvPacket]
	require.True(t, ok, "unable to find messages for MsgRecvPacket type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	cosmosMsg := cosmos.CosmosMsg(sequenceMessage)
	require.NotNil(t, cosmosMsg, "error parsing message as CosmosMsg")

	msgRecvPacket, ok := cosmosMsg.(*chantypes.MsgAcknowledgement)
	require.True(t, ok, "unable to read message as MsgAcknowledgement")

	require.Empty(t, cmp.Diff(packetInfo.packet, msgRecvPacket.Packet), "MsgAcknowledgement data does not match MsgRecvPacket data")
}

func TestHandleMsgAcknowledgement(t *testing.T) {
	const (
		sequence   = uint64(1)
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData = []byte{0x1, 0x2, 0x3, 0x4}
		ccp        = mockCosmosChainProcessor(t)
	)

	ibcMessagesCache := processor.NewIBCMessagesCache()

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
	}

	ccp.handleMsgAcknowledgement(MsgHandlerParams{messageInfo: packetInfo, ibcMessagesCache: ibcMessagesCache})

	require.Len(t, ibcMessagesCache.PacketFlow, 1)

	channelKey := packetInfo.channelKey()

	channelMessages, ok := ibcMessagesCache.PacketFlow[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgAcknowledgement]
	require.True(t, ok, "unable to find messages for MsgAcknowledgement type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	require.Nil(t, sequenceMessage, "message is not nil, expected nil since no messages need to be constructed for counterparty")
}

func TestHandleMsgTimeout(t *testing.T) {
	const (
		sequence   = uint64(1)
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData = []byte{0x1, 0x2, 0x3, 0x4}
		ccp        = mockCosmosChainProcessor(t)
	)

	ibcMessagesCache := processor.NewIBCMessagesCache()

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
	}

	ccp.handleMsgTimeout(MsgHandlerParams{messageInfo: packetInfo, ibcMessagesCache: ibcMessagesCache})

	require.Len(t, ibcMessagesCache.PacketFlow, 1)

	channelKey := packetInfo.channelKey()

	channelMessages, ok := ibcMessagesCache.PacketFlow[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgTimeout]
	require.True(t, ok, "unable to find messages for MsgTimeout type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	require.Nil(t, sequenceMessage, "message is not nil, expected nil since no messages need to be constructed for counterparty")
}

func TestHandleMsgTimeoutOnClose(t *testing.T) {
	const (
		sequence   = uint64(1)
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData = []byte{0x1, 0x2, 0x3, 0x4}
		ccp        = mockCosmosChainProcessor(t)
	)

	ibcMessagesCache := processor.NewIBCMessagesCache()

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
	}

	ccp.handleMsgTimeoutOnClose(MsgHandlerParams{messageInfo: packetInfo, ibcMessagesCache: ibcMessagesCache})

	require.Len(t, ibcMessagesCache.PacketFlow, 1)

	channelKey := packetInfo.channelKey()

	channelMessages, ok := ibcMessagesCache.PacketFlow[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgTimeoutOnClose]
	require.True(t, ok, "unable to find messages for MsgTimeoutOnClose type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	require.Nil(t, sequenceMessage, "message is not nil, expected nil since no messages need to be constructed for counterparty")
}

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

	ccp.handleMsgConnectionOpenInit(MsgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionKey := connectionInfo.connectionKey()

	connectionOpen, ok := ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.False(t, connectionOpen, "connection should not be marked open yet")

	require.Len(t, ibcMessagesCache.ConnectionHandshake, 1)

	connectionMessages, hasConnectionKey := ibcMessagesCache.ConnectionHandshake[connectionKey]
	require.True(t, hasConnectionKey, "no messages cached for connection key")

	_, hasConnectionOpenInit := connectionMessages[processor.MsgConnectionOpenInit]
	require.True(t, hasConnectionOpenInit, "no messages cached for MsgConnectionOpenInit")

	ccp.handleMsgConnectionOpenAck(MsgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

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

	ccp.handleMsgConnectionOpenTry(MsgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

	connectionKey := connectionInfo.connectionKey().Counterparty()

	connectionOpen, ok := ccp.connectionStateCache[connectionKey]
	require.True(t, ok, "unable to find connection state for connection key")

	require.False(t, connectionOpen, "connection should not be marked open yet")

	require.Len(t, ibcMessagesCache.ConnectionHandshake, 1)

	connectionMessages, hasConnectionKey := ibcMessagesCache.ConnectionHandshake[connectionKey]
	require.True(t, hasConnectionKey, "no messages cached for connection key")

	_, hasConnectionOpenTry := connectionMessages[processor.MsgConnectionOpenTry]
	require.True(t, hasConnectionOpenTry, "no messages cached for MsgConnectionOpenTry")

	ccp.handleMsgConnectionOpenConfirm(MsgHandlerParams{messageInfo: connectionInfo, ibcMessagesCache: ibcMessagesCache})

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

	ccp.handleMsgChannelOpenInit(MsgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelKey := channelInfo.channelKey()

	channelOpen, ok := ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.False(t, channelOpen, "channel should not be marked open yet")

	require.Len(t, ibcMessagesCache.ChannelHandshake, 1)

	channelMessages, hasChannelKey := ibcMessagesCache.ChannelHandshake[channelKey]
	require.True(t, hasChannelKey, "no messages cached for channel key")

	_, hasChannelOpenInit := channelMessages[processor.MsgChannelOpenInit]
	require.True(t, hasChannelOpenInit, "no messages cached for MsgChannelOpenInit")

	ccp.handleMsgChannelOpenAck(MsgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

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

	ccp.handleMsgChannelOpenTry(MsgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

	channelKey := channelInfo.channelKey().Counterparty()

	channelOpen, ok := ccp.channelStateCache[channelKey]
	require.True(t, ok, "unable to find channel state for channel key")

	require.False(t, channelOpen, "channel should not be marked open yet")

	require.Len(t, ibcMessagesCache.ChannelHandshake, 1)

	channelMessages, hasChannelKey := ibcMessagesCache.ChannelHandshake[channelKey]
	require.True(t, hasChannelKey, "no messages cached for channel key")

	_, hasChannelOpenTry := channelMessages[processor.MsgChannelOpenTry]
	require.True(t, hasChannelOpenTry, "no messages cached for MsgChannelOpenTry")

	ccp.handleMsgChannelOpenConfirm(MsgHandlerParams{messageInfo: channelInfo, ibcMessagesCache: ibcMessagesCache})

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
