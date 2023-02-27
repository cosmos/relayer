package icon

import (
	"fmt"
	"testing"

	"github.com/icon-project/ibc-relayer/relayer/chains/icon/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// func TestPrint(t *testing.T) {
// 	hash := "0x5306e343d648250f0567e9b549d3c03430aa0ab5a80dffc944cb0db3dbe4ed74"
// 	param := jsonrpc.HexBytes(hash)
// 	res, _ := EventFromTransaction(types.HexBytes(param))
// 	fmt.Printf("%+v", res)
// }

// func TestEventFormat(t *testing.T) {
// 	hash := "0xee01857863616c6c896368616e6e656c2d30857863616c6c896368616e6e656c2d3180c6840098967f028463f40509"
// 	param := jsonrpc.HexBytes(hash)
// 	fmt.Printf("%+v", param)
// }

func TestParseIBCMessageFromEvent(t *testing.T) {
	event := &types.EventLog{
		Addr: types.Address(""),
		Indexed: []string{
			EventTypeSendPacket,
			"0xee01857863616c6c896368616e6e656c2d30857863616c6c896368616e6e656c2d3180c602840098967f8463f4406d",
		},
	}
	msg := parseIBCMessageFromEvent(&zap.Logger{}, *event, "icon", 9_999_999)
	ibcMessage := *msg
	assert.Equal(t, EventTypeSendPacket, ibcMessage.eventType)
	assert.NotNil(t, ibcMessage.info)
}

func TestDecode(t *testing.T) {
	unfiltered := "0xee01857863616c6c896368616e6e656c2d30857863616c6c896368616e6e656c2d3180c602840098967f8463f4406d"
	packet, err := _parsePacket(unfiltered)
	if err != nil {
		fmt.Printf("%+v", err)
	}
	require.NoError(t, err)
	expected := &Packet{
		Sequence:           1,
		SourcePort:         "xcall",
		SourceChannel:      "channel-0",
		DestinationPort:    "xcall",
		DestinationChannel: "channel-1",
		Height: Height{
			RevisionNumber: 9_999_999,
			RevisionHeight: 2,
		},
		Data:      make([]byte, 0),
		Timestamp: 1676951661,
	}
	assert.Equal(t, expected, packet)
}

func TestClientSetup(t *testing.T) {
	provider := IconProviderConfig{
		Key:               "icon",
		ChainName:         "icon",
		ChainID:           "0x1",
		RPCAddr:           "https://ctz.solidwallet.io/api/v3",
		Timeout:           "0",
		IbcHostAddress:    "cx997849d3920d338ed81800833fbb270c785e743d",
		IbcHandlerAddress: "cx997849d3920d338ed81800833fbb270c785e743d",
	}
	l := zap.Logger{}
	ip, e := provider.NewProvider(&l, "icon", true, "icon")
	i := ip.(*IconProvider)

	require.NoError(t, e)
	hash := "0x5306e343d648250f0567e9b549d3c03430aa0ab5a80dffc944cb0db3dbe4ed74"
	param := &types.TransactionHashParam{Hash: types.HexBytes(hash)}
	res, err := i.client.GetTransactionResult(param)
	fmt.Println(res.EventLogs)
	require.NoError(t, err)
	assert.Equal(t, types.HexInt("0x1"), res.Status)
}

// func TestMonitorEvents(t *testing.T) {
// 	provider := IconProviderConfig{
// 		Key:               "icon",
// 		ChainName:         "icon",
// 		ChainID:           "0x1",
// 		RPCAddr:           "https://ctz.solidwallet.io/api/v3",
// 		Timeout:           "0",
// 		IbcHostAddress:    "cx997849d3920d338ed81800833fbb270c785e743d",
// 		IbcHandlerAddress: "cx997849d3920d338ed81800833fbb270c785e743d",
// 	}
// 	l := zap.Logger{}
// 	ip, _ := provider.NewProvider(&l, "icon", true, "icon")
// 	i := ip.(*IconProvider)

// 	const height int64 = 59489570

// 	blockReq := &types.BlockRequest{
// 		EventFilters: []*types.EventFilter{{
// 			Addr:      types.Address(CONTRACT_ADDRESS),
// 			Signature: SEND_PACKET_SIGNATURE,
// 			// Indexed:   []*string{&dstAddr},
// 		}},
// 		Height: types.NewHexInt(height),
// 	}
// 	ctx := context.Background()
// 	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
// 	defer cancel()

// 	h, s := int(height), 0

// 	go func() {
// 		err := i.client.MonitorBlock(ctx, blockReq, func(conn *websocket.Conn, v *types.BlockNotification) error {
// 			_h, _ := v.Height.Int()
// 			if _h != h {
// 				err := fmt.Errorf("invalid block height: %d, expected: %d", _h, h+1)
// 				l.Warn(err.Error())
// 				return err
// 			}
// 			h++
// 			s++

// 			return nil
// 		},
// 			func(conn *websocket.Conn) {
// 				l.Info("Connected")
// 			},
// 			func(conn *websocket.Conn, err error) {
// 				l.Info("Disconnected")
// 				_ = conn.Close()
// 			})
// 		if err.Error() == "context deadline exceeded" {
// 			return
// 		}
// 	}()

// }
