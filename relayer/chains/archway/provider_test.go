package archway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/CosmWasm/wasmd/app"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockAccountSequenceMismatchError struct {
	Expected uint64
	Actual   uint64
}

func (err mockAccountSequenceMismatchError) Error() string {
	return fmt.Sprintf("account sequence mismatch, expected %d, got %d: incorrect account sequence", err.Expected, err.Actual)
}

type Msg struct {
	Count int
}

func (m *Msg) Type() string {
	return "int"
}

func (m *Msg) MsgBytes() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Msg) ValidateBasic() error {
	return nil
}

func (m *Msg) GetSigners() []sdk.AccAddress {
	return nil
}

func (m *Msg) Reset() {

}

func (m *Msg) String() string {
	return "str"
}
func (m *Msg) ProtoMessage() {
}

func GetProvider(ctx context.Context) (provider.ChainProvider, error) {

	absPath, _ := filepath.Abs("../../../env/archway/keys")
	config := ArchwayProviderConfig{
		KeyDirectory:      absPath,
		Key:               "testWallet",
		ChainName:         "archway",
		ChainID:           "constantine-2",
		RPCAddr:           "https://rpc.constantine-2.archway.tech:443",
		AccountPrefix:     "archway",
		KeyringBackend:    "test",
		GasAdjustment:     1.5,
		GasPrices:         "0.02uconst",
		Debug:             true,
		Timeout:           "20s",
		SignModeStr:       "direct",
		MinGasAmount:      300_000,
		IbcHandlerAddress: "heheh",
	}

	p, err := config.NewProvider(&zap.Logger{}, "../../../env/archway", true, "archway")
	if err != nil {
		return nil, err
	}
	err = p.Init(ctx)
	if err != nil {
		return nil, err
	}
	return p, err

}

func TestGetAddress(t *testing.T) {
	ctx := context.Background()
	p, err := GetProvider(ctx)
	assert.NoError(t, err)
	pArch := p.(*ArchwayProvider)
	// _, err = pArch.AddKey("testWallet", 118)
	// assert.NoError(t, err)
	a := "archway1qlfxs7h3r02njh5cykjak2nel54hq8s47h7khl"
	addr, err := pArch.GetKeyAddress()
	assert.NoError(t, err)
	assert.Equal(t, a, addr.String())
	// opx, err := pArch.ShowAddress("testWallet")
	// assert.NoError(t, err)
	// assert.Equal(t, addr, opx)
}

type HexBytes string

func (hs HexBytes) Value() ([]byte, error) {
	if hs == "" {
		return nil, nil
	}
	return hex.DecodeString(string(hs[2:]))
}
func NewHexBytes(b []byte) HexBytes {
	return HexBytes(hex.EncodeToString(b))
}

func TestTxCall(t *testing.T) {

	ctx := context.Background()
	p, _ := GetProvider(ctx)
	pArch := p.(*ArchwayProvider)

	contract := "archway192v3xzzftjylqlty0tw6p8k7adrlf2l3ch9j76augya4yp8tf36ss7d3wa"

	// cl, _ := client.NewClientFromNode("http://localhost:26657")
	cl, _ := client.NewClientFromNode("https://rpc.constantine-2.archway.tech:443")
	addr, err := pArch.GetKeyAddress()
	assert.NoError(t, err)

	encodingConfig := app.MakeEncodingConfig()
	cliCtx := client.Context{}.
		WithClient(cl).
		WithFromName(pArch.PCfg.Key).
		WithFromAddress(addr).
		WithTxConfig(encodingConfig.TxConfig).
		WithSkipConfirmation(true).
		WithBroadcastMode("sync")

	/////////////////////////////////////////////////
	///////////////////// EXECUTION /////////////////
	/////////////////////////////////////////////////

	pktData := []byte("data")

	// type SendPacketParams struct {
	// 	Packet HexBytes `json:"packet"`
	// 	Id     string   `json:"id"`
	// }
	// type SendPacket struct {
	// 	Pkt SendPacketParams `json:"send_packet"`
	// }

	// sendPkt := SendPacket{
	// 	Pkt: SendPacketParams{
	// 		Packet: NewHexBytes(pktData),
	// 		Id:     "100",
	// 	},
	// }

	// dB, err := json.Marshal(sendPkt)
	// assert.NoError(t, err)

	// msg := &wasmtypes.MsgExecuteContract{
	// 	Sender:   addr.String(),
	// 	Contract: contract,
	// 	Msg:      dB,
	// }

	// a := pArch.TxFactory()
	// factory, err := pArch.PrepareFactory(a)
	// assert.NoError(t, err)

	// tx.GenerateOrBroadcastTxWithFactory(cliCtx, factory, msg)

	/////////////////////////////////////////////////
	/////////////////////// QUERY ///////////////////
	/////////////////////////////////////////////////

	type GetPacket struct {
		GetPacket struct {
			Id string `json:"id"`
		} `json:"get_packet"`
	}

	type PacketOutput struct {
		Packet []byte `json:"packet"`
	}

	_param := GetPacket{
		GetPacket: struct {
			Id string "json:\"id\""
		}{
			Id: "100",
		},
	}

	// type GetAllPacket struct {
	// 	GetAllPacket interface{} `json:"get_packet"`
	// }

	// _param := GetAllPacket{GetAllPacket: struct{}{}}

	param, _ := json.Marshal(_param)

	queryCLient := wasmtypes.NewQueryClient(cliCtx)
	contractState, _ := queryCLient.SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
		Address:   contract,
		QueryData: param,
	})
	e := contractState.Data.Bytes()
	var i PacketOutput
	err = json.Unmarshal(e, &i)
	assert.NoError(t, err)
	assert.Equal(t, pktData, i.Packet)

}
