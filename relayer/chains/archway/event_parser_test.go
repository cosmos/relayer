package archway

import (
	"context"
	"testing"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestWasmPrefix(t *testing.T) {
	string := "wasm-create_client"
	assert.Equal(t, wasmPrefix, string[:5])
}

func TestEventParser(t *testing.T) {

	addr := "https://rpc.constantine-2.archway.tech:443"
	contract_address := "archway1w28yk5n5pjk8mjshxycu2lhhlcr8lzqnfc23mgtcsuzdwlv5cx2qemlcsd"
	client, err := NewRPCClient(addr, 5*time.Second)
	assert.NoError(t, err)
	ctx := context.Background()
	var h int64 = 1713974
	rs, err := client.BlockResults(ctx, &h)
	assert.NoError(t, err)

	var m []ibcMessage
	for _, tx := range rs.TxsResults {
		if tx.Code != 0 {
			// tx was not successful
			continue
		}
		m = append(m, ibcMessagesFromEvents(&zap.Logger{}, tx.Events, "archway", 1711515, contract_address, true)...)
	}

	assert.Equal(t, len(m), 1)
	ibcPacket := m[0]
	assert.Equal(t, ibcPacket.eventType, chantypes.EventTypeSendPacket)
	dummyInfo := &packetInfo{
		Height:           1711515,
		Sequence:         1811435,
		SourcePort:       "port-1",
		SourceChannel:    "channel-0",
		DestPort:         "port-1",
		DestChannel:      "channel-0",
		ChannelOrder:     "ORDER_UNORDERED",
		Data:             []byte{123, 34, 97, 109, 111, 117, 110, 116, 34, 58, 34, 49, 54, 49, 50, 55, 52, 34, 44, 34, 100, 101, 110, 111, 109, 34, 58, 34, 117, 97, 116, 111, 109, 34, 44, 34, 114, 101, 99, 101, 105, 118, 101, 114, 34, 58, 34, 111, 115, 109, 111, 49, 120, 52, 54, 102, 102, 52, 53, 99, 116, 107, 97, 107, 54, 114, 106, 113, 107, 97, 57, 117, 113, 118, 119, 113, 116, 52, 118, 104, 100, 114, 52, 53, 114, 112, 99, 55, 102, 51, 34, 44, 34, 115, 101, 110, 100, 101, 114, 34, 58, 34, 99, 111, 115, 109, 111, 115, 49, 120, 52, 54, 102, 102, 52, 53, 99, 116, 107, 97, 107, 54, 114, 106, 113, 107, 97, 57, 117, 113, 118, 119, 113, 116, 52, 118, 104, 100, 114, 52, 53, 116, 54, 116, 119, 108, 114, 34, 125},
		TimeoutHeight:    clienttypes.Height{RevisionHeight: 9454229, RevisionNumber: 1},
		TimeoutTimestamp: 0,
		Ack:              nil,
	}
	assert.Equal(t, dummyInfo, ibcPacket.info)

}
