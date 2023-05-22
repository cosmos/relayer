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
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	icon_types "github.com/icon-project/IBC-Integration/libraries/go/common/icon"

	"github.com/cosmos/relayer/v2/relayer/chains/archway/types"
	"github.com/cosmos/relayer/v2/relayer/chains/icon"
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

const (
	archway_mock_address = "archway1maqs3qvslrjaq8xz9402shucnr4wzdujty8lr7ux5z5rnj989lwsmssrzk"
)

func GetProvider(ctx context.Context, handlerAddr string, local bool) (provider.ChainProvider, error) {

	absPath, _ := filepath.Abs("../../../env/archway/keys")
	var config ArchwayProviderConfig
	if local {
		config = ArchwayProviderConfig{
			KeyDirectory:      absPath,
			Key:               "testWallet",
			ChainName:         "archway",
			ChainID:           "localnet",
			RPCAddr:           "http://localhost:26657",
			AccountPrefix:     "archway",
			KeyringBackend:    "test",
			GasAdjustment:     1.5,
			GasPrices:         "0.02stake",
			Debug:             true,
			Timeout:           "20s",
			SignModeStr:       "direct",
			MinGasAmount:      1000_000,
			IbcHandlerAddress: handlerAddr,
		}
	} else {

		config = ArchwayProviderConfig{
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
			MinGasAmount:      1000_000,
			IbcHandlerAddress: handlerAddr,
		}
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
	p, err := GetProvider(ctx, "", false)
	assert.NoError(t, err)
	pArch := p.(*ArchwayProvider)
	// _, err = pArch.AddKey("testWallet", 118)
	// assert.NoError(t, err)
	a := "archway1qlfxs7h3r02njh5cykjak2nel54hq8s47h7khl"
	addr, err := pArch.GetKeyAddress()
	assert.NoError(t, err)
	assert.Equal(t, a, addr.String())

	op, err := pArch.QueryBalance(ctx, "default")
	assert.NoError(t, err)

	fmt.Println("balance", op)
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

type SendPacket struct {
	Pkt struct {
		Packet HexBytes `json:"packet"`
		Id     string   `json:"id"`
	} `json:"send_packet"`
}

func (m *SendPacket) Type() string {
	return "sendPacket"
}

func (m *SendPacket) MsgBytes() ([]byte, error) {
	return json.Marshal(m)
}

// func TestTransaction(t *testing.T) {
// 	ctx := context.Background()
// 	contract := "archway1j2zsnnv7qpd6hqhrkg96c57wv9yff4y6amarcvsp5lkta2e4k5vstvt9j3"
// 	p, _ := GetProvider(ctx, contract)
// 	pArch := p.(*ArchwayProvider)
// 	pArch.Init(ctx)

// 	key := "jptKey"

// 	msg := &SendPacket{
// 		Pkt: struct {
// 			Packet HexBytes "json:\"packet\""
// 			Id     string   "json:\"id\""
// 		}{
// 			Packet: NewHexBytes([]byte("Hello")),
// 			Id:     key,
// 		},
// 	}

// 	// msg, err := pArch.MsgSendPacketTemp(key)
// 	// assert.NoError(t, err)

// 	callback := func(rtr *provider.RelayerTxResponse, err error) {
// 		if err != nil {
// 			return
// 		}
// 	}

// 	err := pArch.SendMessagesToMempool(ctx, []provider.RelayerMessage{msg}, "memo", nil, callback)
// 	assert.NoError(t, err)

// 	storageKey := fmt.Sprintf("0007%x%s", []byte("packets"), key)
// 	_, err = pArch.QueryArchwayProof(ctx, []byte(storageKey), 1932589)
// 	assert.NoError(t, err)

// }

func TestTxCall(t *testing.T) {

	ctx := context.Background()

	p, _ := GetProvider(ctx, "", false)
	pArch := p.(*ArchwayProvider)

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

	// pktData := []byte("hello_world")

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
	// 		Id:     "345",
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

	// _param := GetPacket{
	// 	GetPacket: struct {
	// 		Id string "json:\"id\""
	// 	}{
	// 		Id: "100",
	// 	},
	// }

	// type GetAllPacket struct {
	// 	GetAllPacket interface{} `json:"get_packet"`
	// }

	cs := types.GetClientState{
		ClientState: struct {
			ClientId string "json:\"client_id\""
		}{
			ClientId: "iconclient-0",
		},
	}

	param, _ := json.Marshal(cs)

	queryCLient := wasmtypes.NewQueryClient(cliCtx)
	contractState, err := queryCLient.SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
		Address:   archway_mock_address,
		QueryData: param,
	})

	assert.NoError(t, err)
	e := contractState.Data
	var i icon_types.ClientState
	err = json.Unmarshal(e, &i)
	fmt.Printf("data is %s \n", e)
	assert.NoError(t, err)
	fmt.Printf("data is %+v \n", i)

}

func TestSerializeAny(t *testing.T) {

	d := clienttypes.Height{
		RevisionNumber: 0,
		RevisionHeight: 20000,
	}
	anyValue, err := codectypes.NewAnyWithValue(&d)
	assert.NoError(t, err)
	clt := clienttypes.MsgCreateClient{
		ClientState:    anyValue,
		ConsensusState: anyValue,
		Signer:         "acbdef",
	}
	cdc := MakeCodec(ModuleBasics, []string{})
	actual, err := cdc.Marshaler.MarshalJSON(&clt)
	assert.NoError(t, err)
	expected, _ := hex.DecodeString("7b22636c69656e745f7374617465223a7b224074797065223a222f6962632e636f72652e636c69656e742e76312e486569676874222c227265766973696f6e5f6e756d626572223a2230222c227265766973696f6e5f686569676874223a223230303030227d2c22636f6e73656e7375735f7374617465223a7b224074797065223a222f6962632e636f72652e636c69656e742e76312e486569676874222c227265766973696f6e5f6e756d626572223a2230222c227265766973696f6e5f686569676874223a223230303030227d2c227369676e6572223a22616362646566227d")
	assert.Equal(t, actual, expected)

}

func GetIconProvider(network_id int) *icon.IconProvider {

	absPath, _ := filepath.Abs("../../../env/godWallet.json")

	pcfg := icon.IconProviderConfig{
		Keystore:          absPath,
		Password:          "gochain",
		ICONNetworkID:     3,
		BTPNetworkID:      int64(network_id),
		BTPNetworkTypeID:  1,
		IbcHandlerAddress: "cxff5fce97254f26dee5a5d35496743f61169b6db6",
		RPCAddr:           "http://localhost:9082/api/v3",
		Timeout:           "20s",
	}
	log, _ := zap.NewProduction()
	p, _ := pcfg.NewProvider(log, "", false, "icon")

	iconProvider, _ := p.(*icon.IconProvider)
	return iconProvider
}

// func TestCreateClient(t *testing.T) {

// 	ctx := context.Background()
// 	ap, err := GetProvider(ctx, "archway1maqs3qvslrjaq8xz9402shucnr4wzdujty8lr7ux5z5rnj989lwsmssrzk", true)
// 	assert.NoError(t, err)

// 	archwayP, ok := ap.(*ArchwayProvider)
// 	if !ok {
// 		assert.Fail(t, "failed to convert to archwayP")
// 	}

// 	networkId := 2
// 	height := 307
// 	ip := GetIconProvider(networkId)

// 	btpHeader, err := ip.GetBtpHeader(int64(height))
// 	assert.NoError(t, err)

// 	header := icon.NewIconIBCHeader(btpHeader, nil, int64(height))
// 	fmt.Println(header.Height())

// 	clS, err := ip.NewClientState("07-tendermint", header, 100, 100, true, true)
// 	assert.NoError(t, err)

// 	msg, err := archwayP.MsgCreateClient(clS, header.ConsensusState())
// 	if err != nil {
// 		assert.Fail(t, err.Error())
// 		fmt.Println("error in unexpected place ")
// 		return
// 	}

// 	fmt.Printf("the value is %s \n", msg)

// 	callback := func(rtr *provider.RelayerTxResponse, err error) {
// 		if err != nil {
// 			return
// 		}
// 	}

// 	err = archwayP.SendMessagesToMempool(ctx, []provider.RelayerMessage{msg}, "memo", nil, callback)
// 	time.Sleep(2 * 1000)
// 	assert.NoError(t, err)

// }

func TestGetClientState(t *testing.T) {
	ctx := context.Background()
	ap, err := GetProvider(ctx, "", false)
	assert.NoError(t, err)

	archwayP, ok := ap.(*ArchwayProvider)
	if !ok {
		assert.Fail(t, "failed to convert to archwayP")
	}

	state, err := archwayP.QueryClientStateContract(ctx, "iconclient-0")
	assert.NoError(t, err)
	fmt.Printf("ClentState %+v \n", state)

}

func TestDataDecode(t *testing.T) {

	d := []byte{10, 32, 47, 105, 99, 111, 110, 46, 108, 105, 103, 104, 116, 99, 108, 105, 101, 110, 116, 46, 118, 49, 46, 67, 108, 105, 101, 110, 116, 83, 116, 97, 116, 101, 18, 32, 127, 98, 36, 134, 45, 9, 198, 30, 199, 185, 205, 28, 128, 214, 203, 138, 15, 65, 45, 70, 134, 139, 202, 40, 61, 44, 97, 169, 50, 7, 225, 18}
	// d := "103247105991111104610810510310411699108105101110116461184946671081051011101168311697116101183212798361344591983019918520528128214203138156545701341392024061449716950722518"
	// b, err := hex.DecodeString(d)
	// assert.NoError(t, err)

	ctx := context.Background()
	ap, err := GetProvider(ctx, "", false)
	assert.NoError(t, err)
	archwayP, _ := ap.(*ArchwayProvider)

	var iconee exported.ClientState
	err = archwayP.Cdc.Marshaler.UnmarshalInterface(d, &iconee)
	assert.NoError(t, err)
	fmt.Println(iconee.GetLatestHeight())

}

func TestXxx(t *testing.T) {
	signer := "hello"
	assert.Equal(t, types.HexBytes(signer), types.NewHexBytes([]byte(signer)))
}
