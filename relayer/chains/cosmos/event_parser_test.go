package cosmos

import (
	"encoding/hex"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
)

func TestParsePacket(t *testing.T) {
	const (
		testPacketTimeoutHeight    = "1-1245"
		testPacketTimeoutTimestamp = "1654033235600000000"
		testPacketSequence         = "1"
		testPacketDataHex          = "0123456789ABCDEF"
		testPacketSrcChannel       = "channel-0"
		testPacketSrcPort          = "port-0"
		testPacketDstChannel       = "channel-1"
		testPacketDstPort          = "port-1"
	)

	packetEventAttributes := []sdk.Attribute{
		{
			Key:   chantypes.AttributeKeySequence,
			Value: testPacketSequence,
		},
		{
			Key:   chantypes.AttributeKeyDataHex,
			Value: testPacketDataHex,
		},
		{
			Key:   chantypes.AttributeKeyTimeoutHeight,
			Value: testPacketTimeoutHeight,
		},
		{
			Key:   chantypes.AttributeKeyTimeoutTimestamp,
			Value: testPacketTimeoutTimestamp,
		},
		{
			Key:   chantypes.AttributeKeySrcChannel,
			Value: testPacketSrcChannel,
		},
		{
			Key:   chantypes.AttributeKeySrcPort,
			Value: testPacketSrcPort,
		},
		{
			Key:   chantypes.AttributeKeyDstChannel,
			Value: testPacketDstChannel,
		},
		{
			Key:   chantypes.AttributeKeyDstPort,
			Value: testPacketDstPort,
		},
	}

	parsed := new(packetInfo)
	parsed.parseAttrs(zap.NewNop(), packetEventAttributes)

	packetData, err := hex.DecodeString(testPacketDataHex)
	require.NoError(t, err, "error decoding test packet data")

	require.Empty(t, cmp.Diff(provider.PacketInfo(*parsed), provider.PacketInfo{
		Sequence: uint64(1),
		Data:     packetData,
		TimeoutHeight: clienttypes.Height{
			RevisionNumber: uint64(1),
			RevisionHeight: uint64(1245),
		},
		TimeoutTimestamp: uint64(1654033235600000000),
		SourceChannel:    testPacketSrcChannel,
		SourcePort:       testPacketSrcPort,
		DestChannel:      testPacketDstChannel,
		DestPort:         testPacketDstPort,
	}), "parsed does not match expected")
}

func TestParseClient(t *testing.T) {
	const (
		testClientID1             = "test-client-id-1"
		testClientConsensusHeight = "1-1023"
		testClientHeader          = "0123456789ABCDEF"
	)

	clientEventAttributes := []sdk.Attribute{
		{
			Key:   clienttypes.AttributeKeyClientID,
			Value: testClientID1,
		},
		{
			Key:   clienttypes.AttributeKeyConsensusHeight,
			Value: testClientConsensusHeight,
		},
		{
			Key:   clienttypes.AttributeKeyHeader,
			Value: testClientHeader,
		},
	}

	parsed := new(clientInfo)
	parsed.parseAttrs(zap.NewNop(), clientEventAttributes)

	clientHeader, err := hex.DecodeString(testClientHeader)
	require.NoError(t, err, "error parsing test client header")

	require.Empty(t, cmp.Diff(*parsed, clientInfo{
		clientID: testClientID1,
		consensusHeight: clienttypes.Height{
			RevisionNumber: uint64(1),
			RevisionHeight: uint64(1023),
		},
		header: clientHeader,
	}, cmp.AllowUnexported(clientInfo{}, clienttypes.Height{})), "parsed client info does not match expected")
}

func TestParseChannel(t *testing.T) {
	const (
		testConnectionID1 = "test-connection-id-1"
		testChannelID1    = "test-channel-id-1"
		testPortID1       = "test-port-id-1"
		testChannelID2    = "test-channel-id-2"
		testPortID2       = "test-port-id-2"
	)

	channelEventAttributes := []sdk.Attribute{
		{
			Key:   chantypes.AttributeKeyConnectionID,
			Value: testConnectionID1,
		},
		{
			Key:   chantypes.AttributeKeyChannelID,
			Value: testChannelID1,
		},
		{
			Key:   chantypes.AttributeKeyPortID,
			Value: testPortID1,
		},
		{
			Key:   chantypes.AttributeCounterpartyChannelID,
			Value: testChannelID2,
		},
		{
			Key:   chantypes.AttributeCounterpartyPortID,
			Value: testPortID2,
		},
	}

	parsed := new(channelInfo)
	parsed.parseAttrs(zap.NewNop(), channelEventAttributes)

	require.Empty(t, cmp.Diff(provider.ChannelInfo(*parsed), provider.ChannelInfo{
		ConnID:                testConnectionID1,
		ChannelID:             testChannelID1,
		PortID:                testPortID1,
		CounterpartyChannelID: testChannelID2,
		CounterpartyPortID:    testPortID2,
	}), "parsed channel info does not match expected")
}

func TestParseConnection(t *testing.T) {
	const (
		testConnectionID1 = "test-connection-id-1"
		testClientID1     = "test-client-id-1"
		testConnectionID2 = "test-connection-id-2"
		testClientID2     = "test-client-id-2"
	)

	connectionEventAttributes := []sdk.Attribute{
		{
			Key:   conntypes.AttributeKeyConnectionID,
			Value: testConnectionID1,
		},
		{
			Key:   conntypes.AttributeKeyClientID,
			Value: testClientID1,
		},
		{
			Key:   conntypes.AttributeKeyCounterpartyConnectionID,
			Value: testConnectionID2,
		},
		{
			Key:   conntypes.AttributeKeyCounterpartyClientID,
			Value: testClientID2,
		},
	}

	parsed := new(connectionInfo)
	parsed.parseAttrs(zap.NewNop(), connectionEventAttributes)

	require.Empty(t, cmp.Diff(provider.ConnectionInfo(*parsed), provider.ConnectionInfo{
		ClientID:             testClientID1,
		ConnID:               testConnectionID1,
		CounterpartyClientID: testClientID2,
		CounterpartyConnID:   testConnectionID2,
	}), "parsed connection info does not match expected")
}

func TestParseEventLogs(t *testing.T) {
	const (
		testClientID1              = "test-client-id-1"
		testClientConsensusHeight  = "1-1023"
		testPacketTimeoutHeight    = "1-1245"
		testPacketTimeoutTimestamp = "1654033235600000000"
		testPacketSequence         = "1"
		testPacketDataHex          = "0123456789ABCDEF"
		testPacketAckHex           = "FBDA532947"
		testPacketSrcChannel       = "channel-0"
		testPacketSrcPort          = "port-0"
		testPacketDstChannel       = "channel-1"
		testPacketDstPort          = "port-1"
	)
	events := []abci.Event{

		{
			Type: clienttypes.EventTypeUpdateClient,
			Attributes: []abci.EventAttribute{
				{
					Key:   []byte(clienttypes.AttributeKeyClientID),
					Value: []byte(testClientID1),
				},
				{
					Key:   []byte(clienttypes.AttributeKeyConsensusHeight),
					Value: []byte(testClientConsensusHeight),
				},
			},
		},
		{
			Type: chantypes.EventTypeRecvPacket,
			Attributes: []abci.EventAttribute{
				{
					Key:   []byte(chantypes.AttributeKeySequence),
					Value: []byte(testPacketSequence),
				},
				{
					Key:   []byte(chantypes.AttributeKeyDataHex),
					Value: []byte(testPacketDataHex),
				},
				{
					Key:   []byte(chantypes.AttributeKeyTimeoutHeight),
					Value: []byte(testPacketTimeoutHeight),
				},
				{
					Key:   []byte(chantypes.AttributeKeyTimeoutTimestamp),
					Value: []byte(testPacketTimeoutTimestamp),
				},
				{
					Key:   []byte(chantypes.AttributeKeySrcChannel),
					Value: []byte(testPacketSrcChannel),
				},
				{
					Key:   []byte(chantypes.AttributeKeySrcPort),
					Value: []byte(testPacketSrcPort),
				},
				{
					Key:   []byte(chantypes.AttributeKeyDstChannel),
					Value: []byte(testPacketDstChannel),
				},
				{
					Key:   []byte(chantypes.AttributeKeyDstPort),
					Value: []byte(testPacketDstPort),
				},
			},
		},
		{
			Type: chantypes.EventTypeWriteAck,
			Attributes: []abci.EventAttribute{
				{
					Key:   []byte(chantypes.AttributeKeySequence),
					Value: []byte(testPacketSequence),
				},
				{
					Key:   []byte(chantypes.AttributeKeyAckHex),
					Value: []byte(testPacketAckHex),
				},
				{
					Key:   []byte(chantypes.AttributeKeySrcChannel),
					Value: []byte(testPacketSrcChannel),
				},
				{
					Key:   []byte(chantypes.AttributeKeySrcPort),
					Value: []byte(testPacketSrcPort),
				},
				{
					Key:   []byte(chantypes.AttributeKeyDstChannel),
					Value: []byte(testPacketDstChannel),
				},
				{
					Key:   []byte(chantypes.AttributeKeyDstPort),
					Value: []byte(testPacketDstPort),
				},
			},
		},
	}

	ibcMessages := ibcMessagesFromEvents(zap.NewNop(), events, "", 0)

	require.Len(t, ibcMessages, 3)

	msgUpdateClient := ibcMessages[0]
	require.Equal(t, clienttypes.EventTypeUpdateClient, msgUpdateClient.eventType)

	clientInfoParsed, isClientInfo := msgUpdateClient.info.(*clientInfo)
	require.True(t, isClientInfo, "messageInfo is not clientInfo")

	require.Empty(t, cmp.Diff(*clientInfoParsed, clientInfo{
		clientID: testClientID1,
		consensusHeight: clienttypes.Height{
			RevisionNumber: uint64(1),
			RevisionHeight: uint64(1023),
		},
	}, cmp.AllowUnexported(clientInfo{}, clienttypes.Height{})), "parsed client info does not match expected")

	msgRecvPacket := ibcMessages[1]
	require.Equal(t, chantypes.EventTypeRecvPacket, msgRecvPacket.eventType, "message event is not recv_packet")

	packetInfoParsed, isPacketInfo := msgRecvPacket.info.(*packetInfo)
	require.True(t, isPacketInfo, "recv_packet messageInfo is not packetInfo")

	msgWriteAcknowledgement := ibcMessages[2]
	require.Equal(t, chantypes.EventTypeWriteAck, msgWriteAcknowledgement.eventType, "message event is not write_acknowledgement")

	ackPacketInfoParsed, isPacketInfo := msgWriteAcknowledgement.info.(*packetInfo)
	require.True(t, isPacketInfo, "ack messageInfo is not packetInfo")

	packetAck, err := hex.DecodeString(testPacketAckHex)
	require.NoError(t, err, "error decoding test packet ack")

	packetData, err := hex.DecodeString(testPacketDataHex)
	require.NoError(t, err, "error decoding test packet data")

	require.Empty(t, cmp.Diff(provider.PacketInfo(*packetInfoParsed), provider.PacketInfo{
		Sequence: uint64(1),
		Data:     packetData,
		TimeoutHeight: clienttypes.Height{
			RevisionNumber: uint64(1),
			RevisionHeight: uint64(1245),
		},
		TimeoutTimestamp: uint64(1654033235600000000),
		SourceChannel:    testPacketSrcChannel,
		SourcePort:       testPacketSrcPort,
		DestChannel:      testPacketDstChannel,
		DestPort:         testPacketDstPort,
	}), "parsed packet info does not match expected")

	require.Empty(t, cmp.Diff(provider.PacketInfo(*ackPacketInfoParsed), provider.PacketInfo{
		Sequence:      uint64(1),
		SourceChannel: testPacketSrcChannel,
		SourcePort:    testPacketSrcPort,
		DestChannel:   testPacketDstChannel,
		DestPort:      testPacketDstPort,
		Ack:           packetAck,
	}), "parsed packet info does not match expected")
}
