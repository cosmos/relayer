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

const (
	testConnectionID1  = "connection-0"
	testConnectionID2  = "connection-1"
	testClientID1      = "client-0"
	testClientID2      = "client-1"
	testChannelID1     = "channel-0"
	testChannelID2     = "channel-1"
	testPortID1        = "port-0"
	testPortID2        = "port-1"
	testRevisionNumber = "0"
	testRevisionHeight = "10"
)

func TestParsePacket(t *testing.T) {

	testPacketDataHex := "68656C6C6F"

	packetStr := `{
		"packet": {
			"sequence": 0,
			"source_port": "` + testPortID1 + `",
			"source_channel": "` + testChannelID1 + `",
			"destination_port": "` + testPortID2 + `",
			"destination_channel": "` + testChannelID2 + `",
			"data": "` + testPacketDataHex + `",
			"timeout_height": {
				"revision_number": ` + testRevisionNumber + `,
				"revision_height": ` + testRevisionHeight + `
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
		SourceChannel:    testChannelID1,
		SourcePort:       testPortID1,
		DestChannel:      testChannelID2,
		DestPort:         testPortID2,
	}), "parsed does not match expected")
}

func TestParseChannel(t *testing.T) {

	clientStr := `{
		"height": {
			"revision_number": 0,
			"revision_height": 10
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
		ConnID:                testConnectionID1,
		ChannelID:             testChannelID1,
		PortID:                testPortID1,
		CounterpartyChannelID: testChannelID2,
		CounterpartyPortID:    testPortID2,
	}), "parsed channel info does not match expected")
}

func TestParseConnection(t *testing.T) {

	connectionStr := `{
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
		ClientID:             testClientID1,
		ConnID:               testConnectionID1,
		CounterpartyClientID: testClientID2,
		CounterpartyConnID:   testConnectionID2,
	}), "parsed connection info does not match expected")
}
