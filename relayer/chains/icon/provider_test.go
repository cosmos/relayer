package icon

import (
	"fmt"
	"testing"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"github.com/stretchr/testify/assert"
)

func TestConnectionDecode(t *testing.T) {

	input := ("0x0a0f30372d74656e6465726d696e742d3012230a0131120d4f524445525f4f524445524544120f4f524445525f554e4f524445524544180322200a0f30372d74656e6465726d696e742d30120d636f6e6e656374696f6e2d3533")

	var conn conntypes.ConnectionEnd
	_, err := HexStringToProtoUnmarshal(input, &conn)
	if err != nil {
		fmt.Println("error occured", err)
		return
	}

	assert.Equal(t, conn.ClientId, "07-tendermint-0")
}

// func GetProvider() *IconProvider {
// 	pcfg := IconProviderConfig{
// 		Keystore:          "/Users/viveksharmapoudel/my_work_bench/ibriz/ibc-related/ibc-relay/env/godWallet.json",
// 		Password:          "gochain",
// 		ICONNetworkID:     3,
// 		BTPNetworkID:      2,
// 		IbcHandlerAddress: "cx00ba205e3366369b0ca7f8f2ca39293cffadd33b",
// 		RPCAddr:           "http://localhost:9082/api/v3",
// 	}

// 	c := NewClient(pcfg.RPCAddr, &zap.Logger{})

// 	ksByte, err := os.ReadFile(pcfg.Keystore)
// 	if err != nil {
// 		return nil
// 	}

// 	wallet, err := wallet.NewFromKeyStore(ksByte, []byte(pcfg.Password))
// 	if err != nil {
// 		return nil
// 	}

// 	codec := MakeCodec(ModuleBasics, []string{})

// 	return &IconProvider{
// 		PCfg:   &pcfg,
// 		client: c,
// 		codec:  codec,
// 		wallet: wallet,
// 	}

// }
