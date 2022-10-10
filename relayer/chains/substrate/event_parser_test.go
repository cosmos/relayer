package substrate

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestParsePacket(t *testing.T) {
	const (
		testPacketTimeoutHeight    = "1-1245"
		testPacketTimeoutTimestamp = "1654033235600000000"
		testPacketSequence         = "1"
		testPacketDataHex          = "68656C6C6F"
		testPacketSrcChannel       = "channel-0"
		testPacketSrcPort          = "port-0"
		testPacketDstChannel       = "channel-1"
		testPacketDstPort          = "port-1"
	)

	packetStr := `{
		"packet": {
			"sequence": 0,
			"source_port": "port-0",
			"source_channel": "channel-0",
			"destination_port": "port-1",
			"destination_channel": "channel-1",
			"data": "68656C6C6F",
			"timeout_height": {
				"revision_number": 0,
				"revision_height": 10
			},
			"timeout_timestamp": {
				"time": "2022-10-06T11:00:02.664464Z"
			}
		}
	}`

	var packetEventAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(packetStr), &packetEventAttributes)
	require.NoError(t, err)

	parsed := new(packetInfo)
	parsed.parseAttrs(zap.NewNop(), packetEventAttributes["packet"])

	packetData, err := hex.DecodeString(testPacketDataHex)
	require.NoError(t, err, "error decoding test packet data")

	require.Empty(t, cmp.Diff(provider.PacketInfo(*parsed), provider.PacketInfo{
		Sequence: uint64(0),
		Data:     packetData,
		TimeoutHeight: clienttypes.Height{
			RevisionNumber: uint64(0),
			RevisionHeight: uint64(10),
		},
		TimeoutTimestamp: uint64(1665054002664464000),
		SourceChannel:    testPacketSrcChannel,
		SourcePort:       testPacketSrcPort,
		DestChannel:      testPacketDstChannel,
		DestPort:         testPacketDstPort,
	}), "parsed does not match expected")
}

func TestParseChannel(t *testing.T) {
	const (
		testConnectionID = "connection-0"
		testChannelID1   = "channel-0"
		testPortID1      = "port-0"
		testChannelID2   = "channel-1"
		testPortID2      = "port-1"
	)

	clientStr := `{
		"height": {
			"revision_number": 0,
			"revision_height": 10
		},
		"port_id": "` + testPortID1 + `",
		"channel_id": "` + testChannelID1 + `",
		"connection_id": "` + testConnectionID + `",
		"counterparty_port_id": "` + testPortID2 + `",
		"counterparty_channel_id": "` + testChannelID2 + `"
	}`

	var clientAttributes ibcEventQueryItem
	err := json.Unmarshal([]byte(clientStr), &clientAttributes)
	require.NoError(t, err)

	parsed := new(channelInfo)
	parsed.parseAttrs(zap.NewNop(), clientAttributes)

	require.Empty(t, cmp.Diff(provider.ChannelInfo(*parsed), provider.ChannelInfo{
		ConnID:                testConnectionID,
		ChannelID:             testChannelID1,
		PortID:                testPortID1,
		CounterpartyChannelID: testChannelID2,
		CounterpartyPortID:    testPortID2,
	}), "parsed channel info does not match expected")
}
