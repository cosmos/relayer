package icon

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/gorilla/websocket"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestParseEvent(t *testing.T) {
	eventData := "0a0f30372d74656e6465726d696e742d34120261611a050a03696263"
	filtered, _ := hex.DecodeString(eventData)

	p := &icon.Counterparty{}
	err := proto.Unmarshal(filtered, p)
	if err != nil {
		fmt.Println(err)
	}
	assert.NoError(t, err)
	assert.Equal(t, "07-tendermint-4", p.ClientId)
}

func TestParseCounterParty(t *testing.T) {
	cp := &icon.Counterparty{
		ClientId:     "07-tendermint-2",
		ConnectionId: "connection-0",
		Prefix: &icon.MerklePrefix{
			KeyPrefix: []byte("ibc"),
		},
	}
	byt, err := proto.Marshal(cp)
	assert.NoError(t, err)
	fmt.Printf("%x\n", byt)
}

func TestEventMap(t *testing.T) {
	eventName := "BTPMessage(int,int)"
	assert.Equal(t, IconCosmosEventMap[eventName], "")

	eventName = EventTypeCreateClient
	assert.Equal(t, IconCosmosEventMap[eventName], "create_client")

}

func TestCreateClientEvent(t *testing.T) {

	event := types.EventLogStr{
		Addr: types.Address("cxb1b0f589c980ee1738cf964ef6b26d4bbcb54ce7"),
		Indexed: []string{
			"ConnectionOpenAck(str,bytes)",
			"connection-1",
		},
		Data: []string{"0x0a0f30372d74656e6465726d696e742d3012230a0131120d4f524445525f4f524445524544120f4f524445525f554e4f5244455245441803221f0a0f30372d74656e6465726d696e742d30120c636f6e6e656374696f6e2d31"},
	}

	evt := ToEventLogBytes(event)
	ibcMsg := parseIBCMessageFromEvent(&zap.Logger{}, evt, 0)

	fmt.Printf("Ibc message is %s \n ", ibcMsg)
	// clientMsg := ibcMsg.info.(*clientInfo)
	// assert.Equal(t, "07-tendermint-1", clientMsg.clientID)
}

func TestConnectionOpenInitByte(t *testing.T) {
	// format of event received from block notification
	event := types.EventLog{
		Addr: types.Address("cxc598844f5a0b8997a9f9d280c3f228a20c93e1d5"),
		Indexed: [][]byte{
			{67, 111, 110, 110, 101, 99, 116, 105, 111, 110, 79, 112, 101, 110, 73, 110, 105, 116, 40, 115, 116, 114, 44, 115, 116, 114, 44, 98, 121, 116, 101, 115, 41},
			{48, 55, 45, 116, 101, 110, 100, 101, 114, 109, 105, 110, 116, 45, 48},
		},
		Data: [][]byte{
			{99, 111, 110, 110, 101, 99, 116, 105, 111, 110, 45, 49},
			{10, 15, 48, 55, 45, 116, 101, 110, 100, 101, 114, 109, 105, 110, 116, 45, 50, 18, 12, 99, 111, 110, 110, 101, 99, 116, 105, 111, 110, 45, 48, 26, 5, 10, 3, 105, 98, 99},
		},
	}

	ibcMsg := parseIBCMessageFromEvent(&zap.Logger{}, event, 0)
	connAttrs := ibcMsg.info.(*connectionInfo)
	fmt.Printf("%+v", connAttrs)
}

func TestConnectionOpenInit(t *testing.T) {
	event := types.EventLog{
		Addr: types.Address("cxc598844f5a0b8997a9f9d280c3f228a20c93e1d5"),
		Indexed: [][]byte{
			{67, 111, 110, 110, 101, 99, 116, 105, 111, 110, 79, 112, 101, 110, 73, 110, 105, 116, 40, 115, 116, 114, 44, 115, 116, 114, 44, 98, 121, 116, 101, 115, 41},
			{48, 55, 45, 116, 101, 110, 100, 101, 114, 109, 105, 110, 116, 45, 48},
		},
		Data: [][]byte{
			{99, 111, 110, 110, 101, 99, 116, 105, 111, 110, 45, 49},
			{10, 15, 48, 55, 45, 116, 101, 110, 100, 101, 114, 109, 105, 110, 116, 45, 50, 18, 12, 99, 111, 110, 110, 101, 99, 116, 105, 111, 110, 45, 48, 26, 5, 10, 3, 105, 98, 99},
		},
	}
	evt := types.EventLogStr{
		Addr:    types.Address("cxc598844f5a0b8997a9f9d280c3f228a20c93e1d5"),
		Indexed: []string{EventTypeConnectionOpenInit, "07-tendermint-0"},
		Data:    []string{"connection-1", "0x0a0f30372d74656e6465726d696e742d32120c636f6e6e656374696f6e2d301a050a03696263"},
	}

	encodedEvent := ToEventLogBytes(evt)

	assert.Equal(t, event.Addr, encodedEvent.Addr)
	assert.Equal(t, event.Indexed[0], encodedEvent.Indexed[0])
	assert.Equal(t, event.Indexed[1], encodedEvent.Indexed[1])
	assert.Equal(t, event.Data[0], encodedEvent.Data[0])
	assert.Equal(t, event.Data[1], encodedEvent.Data[1])

	cp := &icon.Counterparty{
		ClientId:     "07-tendermint-0",
		ConnectionId: "connection-1",
		Prefix:       &icon.MerklePrefix{},
	}

	ibcMsg := parseIBCMessageFromEvent(&zap.Logger{}, encodedEvent, 0)
	connAttrs := ibcMsg.info.(*connectionInfo)
	assert.Equal(t, cp.ClientId, connAttrs.ClientID)
	assert.Equal(t, cp.ConnectionId, connAttrs.ConnID)
}

func TestMonitorEvents(t *testing.T) {
	provider := IconProviderConfig{
		Key:               "icon",
		ChainName:         "icon",
		ChainID:           "0x1",
		RPCAddr:           "https://ctz.solidwallet.io/api/v3",
		Timeout:           "0",
		IbcHandlerAddress: "cx997849d3920d338ed81800833fbb270c785e743d",
	}
	l := zap.Logger{}
	ip, _ := provider.NewProvider(&l, "icon", true, "icon")
	i := ip.(*IconProvider)

	const height int64 = 59489570

	t.Log("test")
	blockReq := &types.BlockRequest{
		EventFilters: []*types.EventFilter{{
			// Addr: types.Address(CONTRACT_ADDRESS),
			Signature: EventTypeSendPacket,
			// Indexed:   []*string{&dstAddr},
		}},
		Height: types.NewHexInt(height),
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	h, s := int(height), 0
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		t.Log("height")

		err := i.client.MonitorBlock(ctx, blockReq, func(conn *websocket.Conn, v *types.BlockNotification) error {
			t.Log("height")

			_h, _ := v.Height.Int()

			if _h != h {
				err := fmt.Errorf("invalid block height: %d, expected: %d", _h, h+1)
				l.Warn(err.Error())
				return err
			}
			h++
			s++

			return nil
		},
			func(conn *websocket.Conn) {
				l.Info("Connected")
			},
			func(conn *websocket.Conn, err error) {
				l.Info("Disconnected")
				_ = conn.Close()
			})
		if err.Error() == "context deadline exceeded" {
			return
		}
	}()

	wg.Wait()

}

func TestChannelHandshakeDataParsing(t *testing.T) {

	data := []string{
		"080110021a060a046d6f636b220c636f6e6e656374696f6e2d322a0769637332302d31",
	}
	d, _ := hex.DecodeString(data[0])
	var channel icon.Channel
	proto.Unmarshal(d, &channel)
	assert.Equal(t, channel.ConnectionHops[0], "connection-2")
}
