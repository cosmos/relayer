package substrate

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	testSequence       = uint64(1)
	testConnectionID1  = "connection-0"
	testConnectionID2  = "connection-1"
	testClientID1      = "client-0"
	testClientID2      = "client-1"
	testChannelID1     = "channel-0"
	testChannelID2     = "channel-1"
	testPortID1        = "port-0"
	testPortID2        = "port-1"
	testRevisionNumber = 1
	testRevisionHeight = 10
	testTimestamp      = "2022-10-06T11:00:02.664464Z"
	testClientType     = uint32(1)
	testPacketDataHex  = "68656C6C6F"
	testPacketAckHex   = "68656C6C6F"
)

func TestParsePacket(t *testing.T) {

	packetStr := `{
		"height": {
			"revision_number": ` + cast.ToString(testRevisionNumber) + `,
			"revision_height": ` + cast.ToString(testRevisionHeight) + `
		},
		"sequence": ` + cast.ToString(testSequence) + `,
		"source_port": "` + testPortID1 + `",
		"source_channel": "` + testChannelID1 + `",
		"destination_port": "` + testPortID2 + `",
		"destination_channel": "` + testChannelID2 + `",
		"data": "` + testPacketDataHex + `",
		"timeout_height": {
			"revision_number": ` + cast.ToString(testRevisionNumber) + `,
			"revision_height": ` + cast.ToString(testRevisionHeight) + `
		},
		"timeout_timestamp": {
			"time": "` + testTimestamp + `"
		},
		"ack": "` + testPacketAckHex + `"
	}`

	var packetEventAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(packetStr), &packetEventAttributes)
	require.NoError(t, err)

	parsed := new(packetInfo)
	parsed.parseAttrs(zap.NewNop(), packetEventAttributes)

	packetData, err := hex.DecodeString(testPacketDataHex)
	require.NoError(t, err, "error decoding test packet data")

	ackData, err := hex.DecodeString(testPacketAckHex)
	require.NoError(t, err, "error decoding test ack data")

	require.Empty(t, cmp.Diff(provider.PacketInfo(*parsed), provider.PacketInfo{
		Height:   testRevisionHeight,
		Sequence: testSequence,
		Data:     packetData,
		TimeoutHeight: clienttypes.Height{
			RevisionNumber: testRevisionNumber,
			RevisionHeight: testRevisionHeight,
		},
		TimeoutTimestamp: uint64(1665054002664464000),
		SourceChannel:    testChannelID1,
		SourcePort:       testPortID1,
		DestChannel:      testChannelID2,
		DestPort:         testPortID2,
		Ack:              ackData,
	}), "parsed does not match expected")
}

func TestParseClient(t *testing.T) {

	clientStr := `{
		"height": {
			"revision_number": ` + cast.ToString(testRevisionNumber) + `,
			"revision_height": ` + cast.ToString(testRevisionHeight) + `
		},
		"client_id": "` + testClientID1 + `",
		"consensus_height": {
			"revision_number": ` + cast.ToString(testRevisionNumber) + `,
			"revision_height": ` + cast.ToString(testRevisionHeight) + `
		},
		"client_type": "` + cast.ToString(testClientType) + `"
	}`

	var clientEventAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(clientStr), &clientEventAttributes)
	require.NoError(t, err)

	parsed := new(clientInfo)
	parsed.parseAttrs(zap.NewNop(), clientEventAttributes)

	require.Empty(t, cmp.Diff(*parsed, clientInfo{
		height: clienttypes.Height{
			RevisionNumber: testRevisionNumber,
			RevisionHeight: testRevisionHeight,
		},
		clientID: testClientID1,
		consensusHeight: clienttypes.Height{
			RevisionNumber: testRevisionNumber,
			RevisionHeight: testRevisionHeight,
		},
		clientType: testClientType,
	}, cmp.AllowUnexported(clientInfo{}, clienttypes.Height{})), "parsed client info does not match expected")
}

func TestParseClientUpdate(t *testing.T) {

	// header is always ignored, so there is no need to populate it
	clientStr := `{
		"common": {
			"height": {
				"revision_number": ` + cast.ToString(testRevisionNumber) + `,
				"revision_height": ` + cast.ToString(testRevisionHeight) + `
			},
			"client_id": "` + testClientID1 + `",
			"consensus_height": {
				"revision_number": ` + cast.ToString(testRevisionNumber) + `,
				"revision_height": ` + cast.ToString(testRevisionHeight) + `
			},
			"client_type": "` + cast.ToString(testClientType) + `"
		}
	}`

	var clientEventAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(clientStr), &clientEventAttributes)
	require.NoError(t, err)

	parsed := new(clientUpdateInfo)
	parsed.parseAttrs(zap.NewNop(), clientEventAttributes)

	require.Empty(t, cmp.Diff(*parsed, clientUpdateInfo{
		common: clientInfo{
			height: clienttypes.Height{
				RevisionNumber: testRevisionNumber,
				RevisionHeight: testRevisionHeight,
			},
			clientID: testClientID1,
			consensusHeight: clienttypes.Height{
				RevisionNumber: testRevisionNumber,
				RevisionHeight: testRevisionHeight,
			},
			clientType: testClientType,
		},
	}, cmp.AllowUnexported(clientInfo{}, clienttypes.Height{})), "parsed client info does not match expected")
}

func TestParseChannel(t *testing.T) {

	clientStr := `{
		"height": {
			"revision_number": ` + cast.ToString(testRevisionNumber) + `,
			"revision_height": ` + cast.ToString(testRevisionHeight) + `
		},
		"port_id": "` + testPortID1 + `",
		"channel_id": "` + testChannelID1 + `",
		"connection_id": "` + testConnectionID1 + `",
		"counterparty_port_id": "` + testPortID2 + `",
		"counterparty_channel_id": "` + testChannelID2 + `"
	}`

	var clientAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(clientStr), &clientAttributes)
	require.NoError(t, err)

	parsed := new(channelInfo)
	parsed.parseAttrs(zap.NewNop(), clientAttributes)

	require.Empty(t, cmp.Diff(provider.ChannelInfo(*parsed), provider.ChannelInfo{
		Height:                testRevisionHeight,
		ConnID:                testConnectionID1,
		ChannelID:             testChannelID1,
		PortID:                testPortID1,
		CounterpartyChannelID: testChannelID2,
		CounterpartyPortID:    testPortID2,
	}), "parsed channel info does not match expected")
}

func TestParseConnection(t *testing.T) {

	connectionStr := `{
		"height": {
			"revision_number": ` + cast.ToString(testRevisionNumber) + `,
			"revision_height": ` + cast.ToString(testRevisionHeight) + `
		},
		"connection_id": "` + testConnectionID1 + `",
		"client_id": "` + testClientID1 + `",
		"counterparty_connection_id": "` + testConnectionID2 + `",
		"counterparty_client_id": "` + testClientID2 + `"
	}`

	var connectionAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(connectionStr), &connectionAttributes)
	require.NoError(t, err)

	parsed := new(connectionInfo)
	parsed.parseAttrs(zap.NewNop(), connectionAttributes)

	require.Empty(t, cmp.Diff(provider.ConnectionInfo(*parsed), provider.ConnectionInfo{
		Height:               testRevisionHeight,
		ClientID:             testClientID1,
		ConnID:               testConnectionID1,
		CounterpartyClientID: testClientID2,
		CounterpartyConnID:   testConnectionID2,
	}), "parsed connection info does not match expected")
}

func TestParseEvents(t *testing.T) {
	events := `[
		{
			"` + UpdateClient + `": {
				"common": {
					"height": {
						"revision_number": ` + cast.ToString(testRevisionNumber) + `,
						"revision_height": ` + cast.ToString(testRevisionHeight) + `
					},
					"client_id": "` + testClientID1 + `",
					"consensus_height": {
						"revision_number": ` + cast.ToString(testRevisionNumber) + `,
						"revision_height": ` + cast.ToString(testRevisionHeight) + `
					},
					"client_type": "` + cast.ToString(testClientType) + `"
				}
			}
		},
		{
			"` + ReceivePacket + `": {
				"height": {
					"revision_number": ` + cast.ToString(testRevisionNumber) + `,
					"revision_height": ` + cast.ToString(testRevisionHeight) + `
				},
				"sequence": ` + cast.ToString(testSequence) + `,
				"source_port": "` + testPortID1 + `",
				"source_channel": "` + testChannelID1 + `",
				"destination_port": "` + testPortID2 + `",
				"destination_channel": "` + testChannelID2 + `",
				"data": "` + testPacketDataHex + `",
				"timeout_height": {
					"revision_number": ` + cast.ToString(testRevisionNumber) + `,
					"revision_height": ` + cast.ToString(testRevisionHeight) + `
				},
				"timeout_timestamp": {
					"time": "` + testTimestamp + `"
				},
				"ack": "` + testPacketAckHex + `"
			}
		}
	]`

	var eventsResult rpcclienttypes.IBCEventsQueryResult
	err := json.Unmarshal([]byte(events), &eventsResult)
	require.NoError(t, err)

	proc := SubstrateChainProcessor{}
	ibcMessages := proc.ibcMessagesFromEvents(eventsResult)

	require.Len(t, ibcMessages, 2)

	msgUpdateClient := ibcMessages[0]
	require.Equal(t, clienttypes.EventTypeUpdateClient, msgUpdateClient.eventType)

	clientInfoParsed, isClientInfo := msgUpdateClient.info.(*clientUpdateInfo)
	require.True(t, isClientInfo, "messageInfo is not clientInfo")

	require.Empty(t, cmp.Diff(*clientInfoParsed, clientUpdateInfo{
		common: clientInfo{
			height: clienttypes.Height{
				RevisionNumber: testRevisionNumber,
				RevisionHeight: testRevisionHeight,
			},
			clientID: testClientID1,
			consensusHeight: clienttypes.Height{
				RevisionNumber: testRevisionNumber,
				RevisionHeight: testRevisionHeight,
			},
			clientType: testClientType,
		},
	}, cmp.AllowUnexported(clientInfo{}, clienttypes.Height{})), "parsed client info does not match expected")

	msgRecvPacket := ibcMessages[1]
	require.Equal(t, chantypes.EventTypeRecvPacket, msgRecvPacket.eventType, "message event is not recv_packet")

	packetInfoParsed, isPacketInfo := msgRecvPacket.info.(*packetInfo)
	require.True(t, isPacketInfo, "messageInfo is not packetInfo")

	packetAck, err := hex.DecodeString(testPacketAckHex)
	require.NoError(t, err, "error decoding test packet ack")

	packetData, err := hex.DecodeString(testPacketDataHex)
	require.NoError(t, err, "error decoding test packet data")

	require.Empty(t, cmp.Diff(provider.PacketInfo(*packetInfoParsed), provider.PacketInfo{
		Height:   testRevisionHeight,
		Sequence: testSequence,
		Data:     packetData,
		TimeoutHeight: clienttypes.Height{
			RevisionNumber: testRevisionNumber,
			RevisionHeight: testRevisionHeight,
		},
		TimeoutTimestamp: uint64(cast.ToTime(testTimestamp).UnixNano()),
		SourceChannel:    testChannelID1,
		SourcePort:       testPortID1,
		DestChannel:      testChannelID2,
		DestPort:         testPortID2,
		Ack:              packetAck,
	}), "parsed packet info does not match expected")
}
