package icon

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDecode(t *testing.T) {
	filtered := "0xed01857863616c6c896368616e6e656c2d30857863616c6c896368616e6e656c2d3180c58398967f028463f41cdc"
	// unfiltered := "0xee01857863616c6c896368616e6e656c2d30857863616c6c896368616e6e656c2d3180c6840098967f028463f41cdc"
	packet, err := parseSendPacket(filtered)
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
			RevisionNumber: 9999999,
			RevisionHeight: 2,
		},
		Data:      make([]byte, 0),
		Timestamp: 1676942556,
	}
	assert.Equal(t, expected, packet)
}

// func TestMonitor(t *testing.T) {
// 	provider := &IconProviderConfig{
// 		Key:               "icon",
// 		ChainName:         "icon",
// 		ChainID:           "0x1",
// 		RPCAddr:           "https://ctz.solidwallet.io/api/v3",
// 		Timeout:           "50",
// 		IbcHostAddress:    "cx997849d3920d338ed81800833fbb270c785e743d",
// 		IbcHandlerAddress: "cx997849d3920d338ed81800833fbb270c785e743d",
// 	}
// 	l := zap.Logger{}
// 	iconProvider, _ := provider.NewProvider(&l, "icon", true, "icon")

// 	// hash := "0x5306e343d648250f0567e9b549d3c03430aa0ab5a80dffc944cb0db3dbe4ed74"
// 	// param := jsonrpc.HexBytes(hash)
// 	s, _ := iconProvider.Address()
// 	fmt.Println(s)

// 	// FetchEventFromTransaction(param)

// }
