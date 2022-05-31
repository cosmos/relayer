package cosmos

import (
	"encoding/hex"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestParsePacket(t *testing.T) {
	testPacketTimeoutHeight := "1-1245"
	testPacketTimeoutTimestamp := "1654033235600000000"
	testPacketSequence := "1"
	testPacketDataHex := "0123456789ABCDEF"
	testPacketSrcChannel := "channel-0"
	testPacketSrcPort := "port-0"
	testPacketDstChannel := "channel-1"
	testPacketDstPort := "port-1"

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
	parsed = parsed.parsePacketInfo(zap.NewNop(), "", packetEventAttributes)

	require.Equal(t, uint64(1), parsed.packet.Sequence, "packet sequence does not match")
	require.Equal(t, strings.ToUpper(testPacketDataHex), strings.ToUpper(hex.EncodeToString(parsed.packet.Data)), "packet data does not match")

	require.Equal(t, uint64(1), parsed.packet.TimeoutHeight.RevisionNumber, "packet timeout height revision number does not match")
	require.Equal(t, uint64(1245), parsed.packet.TimeoutHeight.RevisionHeight, "packet timeout height revision height does not match")

	require.Equal(t, uint64(1654033235600000000), parsed.packet.TimeoutTimestamp, "packet timeout timestamp does not match")

	require.Equal(t, testPacketSrcChannel, parsed.packet.SourceChannel, "packet source channel does not match")
	require.Equal(t, testPacketSrcPort, parsed.packet.SourcePort, "packet source port does not match")
	require.Equal(t, testPacketDstChannel, parsed.packet.DestinationChannel, "packet destination channel does not match")
	require.Equal(t, testPacketDstPort, parsed.packet.DestinationPort, "packet destination port does not match")
}

func TestParseClient(t *testing.T) {
	testClientID1 := "test-client-id-1"
	testClientConsensusHeight := "1-1023"
	testClientHeader := "0123456789ABCDEF"

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

	parsed := parseClientInfo(zap.NewNop(), "", clientEventAttributes)

	require.Equal(t, testClientID1, parsed.clientID, "client ID does not match")
	require.Equal(t, uint64(1), parsed.consensusHeight.RevisionNumber, "client consensus height revision number does not match")
	require.Equal(t, uint64(1023), parsed.consensusHeight.RevisionHeight, "client consensus height revision height does not match")
	require.Equal(t, strings.ToUpper(testClientHeader), strings.ToUpper(hex.EncodeToString(parsed.header)), "client header does not match")
}

func TestParseChannel(t *testing.T) {
	testConnectionID1 := "test-connection-id-1"
	testChannelID1 := "test-channel-id-1"
	testPortID1 := "test-port-id-1"
	testChannelID2 := "test-channel-id-2"
	testPortID2 := "test-port-id-2"

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

	parsed := parseChannelInfo(channelEventAttributes)

	require.Equal(t, testConnectionID1, parsed.connectionID, "connection ID does not match")
	require.Equal(t, testChannelID1, parsed.channelID, "channel ID does not match")
	require.Equal(t, testPortID1, parsed.portID, "port ID does not match")
	require.Equal(t, testChannelID2, parsed.counterpartyChannelID, "counterparty channel ID does not match")
	require.Equal(t, testPortID2, parsed.counterpartyPortID, "counterparty port ID does not match")
}

func TestParseConnection(t *testing.T) {
	testConnectionID1 := "test-connection-id-1"
	testClientID1 := "test-client-id-1"
	testConnectionID2 := "test-connection-id-2"
	testClientID2 := "test-client-id-2"

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

	parsed := parseConnectionInfo(connectionEventAttributes)

	require.Equal(t, testClientID1, parsed.clientID, "client ID does not match")
	require.Equal(t, testConnectionID1, parsed.connectionID, "connection ID does not match")
	require.Equal(t, testClientID2, parsed.counterpartyClientID, "counterparty client ID does not match")
	require.Equal(t, testConnectionID2, parsed.counterpartyConnectionID, "counterparty connection ID does not match")
}

func TestParseEventLogs(t *testing.T) {
	testChainID1 := "test-chain-id-1"
	testClientID1 := "test-client-id-1"
	testClientConsensusHeight := "1-1023"
	testPacketTimeoutHeight := "1-1245"
	testPacketTimeoutTimestamp := "1654033235600000000"
	testPacketSequence := "1"
	testPacketDataHex := "0123456789ABCDEF"
	testPacketAckHex := "FBDA532947"
	testPacketSrcChannel := "channel-0"
	testPacketSrcPort := "port-0"
	testPacketDstChannel := "channel-1"
	testPacketDstPort := "port-1"
	abciLogs := sdk.ABCIMessageLogs{
		{
			MsgIndex: 0,
			Events: sdk.StringEvents{
				{
					Type: "message",
					Attributes: []sdk.Attribute{
						{
							Key:   "action",
							Value: processor.MsgUpdateClient,
						},
					},
				},
				{
					Type: clienttypes.EventTypeUpdateClient,
					Attributes: []sdk.Attribute{
						{
							Key:   clienttypes.AttributeKeyClientID,
							Value: testClientID1,
						},
						{
							Key:   clienttypes.AttributeKeyConsensusHeight,
							Value: testClientConsensusHeight,
						},
					},
				},
			},
		},
		{
			MsgIndex: 1,
			Events: sdk.StringEvents{
				{
					Type: "message",
					Attributes: []sdk.Attribute{
						{
							Key:   "action",
							Value: processor.MsgRecvPacket,
						},
					},
				},
				{
					Type: chantypes.EventTypeRecvPacket,
					Attributes: []sdk.Attribute{
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
					},
				},
				{
					Type: chantypes.EventTypeWriteAck,
					Attributes: []sdk.Attribute{
						{
							Key:   chantypes.AttributeKeyAckHex,
							Value: testPacketAckHex,
						},
					},
				},
			},
		},
	}

	ibcMessages := parseABCILogs(zap.NewNop(), testChainID1, abciLogs)

	require.Equal(t, len(ibcMessages), 2)

	msgUpdateClient := ibcMessages[0]
	require.Equal(t, processor.MsgUpdateClient, msgUpdateClient.messageType)

	clientInfoParsed, isClientInfo := msgUpdateClient.messageInfo.(clientInfo)
	require.True(t, isClientInfo, "messageInfo is not clientInfo")
	require.Equal(t, testClientID1, clientInfoParsed.clientID, "client ID does not match")
	require.Equal(t, uint64(1), clientInfoParsed.consensusHeight.RevisionNumber, "client consensus height revision number does not match")
	require.Equal(t, uint64(1023), clientInfoParsed.consensusHeight.RevisionHeight, "client consensus height revision height does not match")

	msgRecvPacket := ibcMessages[1]
	require.Equal(t, processor.MsgRecvPacket, msgRecvPacket.messageType, "message is not MsgRecvPacket")

	packetInfoParsed, isPacketInfo := msgRecvPacket.messageInfo.(packetInfo)
	require.True(t, isPacketInfo, "messageInfo is not packetInfo")

	require.Equal(t, uint64(1), packetInfoParsed.packet.Sequence, "packet sequence does not match")
	require.Equal(t, strings.ToUpper(testPacketDataHex), strings.ToUpper(hex.EncodeToString(packetInfoParsed.packet.Data)), "packet data does not match")

	require.Equal(t, uint64(1), packetInfoParsed.packet.TimeoutHeight.RevisionNumber, "packet timeout height revision number does not match")
	require.Equal(t, uint64(1245), packetInfoParsed.packet.TimeoutHeight.RevisionHeight, "packet timeout height revision height does not match")

	require.Equal(t, uint64(1654033235600000000), packetInfoParsed.packet.TimeoutTimestamp, "packet timeout timestamp does not match")

	require.Equal(t, testPacketSrcChannel, packetInfoParsed.packet.SourceChannel, "packet source channel does not match")
	require.Equal(t, testPacketSrcPort, packetInfoParsed.packet.SourcePort, "packet source port does not match")
	require.Equal(t, testPacketDstChannel, packetInfoParsed.packet.DestinationChannel, "packet destination channel does not match")
	require.Equal(t, testPacketDstPort, packetInfoParsed.packet.DestinationPort, "packet destination port does not match")

	require.Equal(t, strings.ToUpper(testPacketAckHex), strings.ToUpper(hex.EncodeToString(packetInfoParsed.ack)), "packet ack does not match")
}
