package archway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/gogoproto/proto"
	icn "github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"

	// tendermint "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"

	"github.com/cosmos/relayer/v2/relayer/chains/icon"
	"github.com/cosmos/relayer/v2/relayer/common"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type mockAccountSequenceMismatchError struct {
	Expected uint64
	Actual   uint64
}

func (err mockAccountSequenceMismatchError) Error() string {
	return fmt.Sprintf("account sequence mismatch, expected %d, got %d: incorrect account sequence", err.Expected, err.Actual)
}

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
			ChainID:           "constantine-3",
			RPCAddr:           "https://rpc.constantine.archway.tech:443",
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

	p, err := config.NewProvider(zaptest.NewLogger(&testing.T{}), "../../../env/archway", true, "archway")
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
	p, err := GetProvider(ctx, "archway14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sy85n2u", false)
	assert.NoError(t, err)
	pArch := p.(*ArchwayProvider)
	assert.NoError(t, err)
	a := "archway1jpdcgkwv7wmwaqc6lyvd82dwhkxxfvplp6u8gw"
	addr, err := pArch.GetKeyAddress()
	assert.NoError(t, err)
	assert.Equal(t, a, addr.String())

	// op, err := pArch.QueryBalance(ctx, "default")
	// assert.NoError(t, err)

	// fmt.Println("balance", op)
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

// func TestTxnResult(t *testing.T) {
// 	hash := "A7FAA098E4671ABDB9C3557B4E94F5C208939804B4CE64BF066669EC75313151"
// 	b, e := hex.DecodeString(hash)
// 	assert.NoError(t, e)

// 	ctx := context.Background()
// 	p, err := GetProvider(ctx, "archway21", true)
// 	assert.NoError(t, err)
// 	pArch, ok := p.(*ArchwayProvider)
// 	assert.True(t, ok)

// 	a := make(chan provider.RelayerTxResponse, 10)

// 	callback := func(rtr *provider.RelayerTxResponse, err error) {
// 		fmt.Printf("Tx Response:: %+v\n ", rtr)
// 		if err == nil {
// 			a <- *rtr
// 		}
// 		return
// 	}

// 	pArch.waitForTx(ctx, b, nil, time.Minute*10, callback)
// brakHere:
// 	for {
// 		select {
// 		case <-a:
// 			{
// 				fmt.Println("response received")
// 				break brakHere
// 			}
// 		}

// 	}

// }

// func TestClientState(t *testing.T) {

// 	ctx := context.Background()
// 	contractAddr := "archway1vguuxez2h5ekltfj9gjd62fs5k4rl2zy5hfrncasykzw08rezpfsa4aasz"
// 	p, err := GetProvider(ctx, contractAddr, true)
// 	assert.NoError(t, err)

// 	archP := p.(*ArchwayProvider)

// 	clientId := "iconclient-0"

// 	iconM, err := archP.QueryClientStateContract(ctx, clientId)
// 	assert.NoError(t, err)
// 	fmt.Printf("%+v", iconM)
// }

// func TestTxCall(t *testing.T) {

// 	ctx := context.Background()

// 	p, _ := GetProvider(ctx, "", false)
// 	pArch := p.(*ArchwayProvider)

// 	// cl, _ := client.NewClientFromNode("http://localhost:26657")
// 	cl, _ := client.NewClientFromNode("https://rpc.constantine-2.archway.tech:443")

// 	addr, err := pArch.GetKeyAddress()
// 	assert.NoError(t, err)

// 	encodingConfig := app.MakeEncodingConfig()
// 	cliCtx := client.Context{}.
// 		WithClient(cl).
// 		WithFromName(pArch.PCfg.Key).
// 		WithFromAddress(addr).
// 		WithTxConfig(encodingConfig.TxConfig).
// 		WithSkipConfirmation(true).
// 		WithBroadcastMode("sync")

// 	/////////////////////////////////////////////////
// 	///////////////////// EXECUTION /////////////////
// 	/////////////////////////////////////////////////

// 	// pktData := []byte("hello_world")

// 	// type SendPacketParams struct {
// 	// 	Packet HexBytes `json:"packet"`
// 	// 	Id     string   `json:"id"`
// 	// }
// 	// type SendPacket struct {
// 	// 	Pkt SendPacketParams `json:"send_packet"`
// 	// }

// 	// sendPkt := SendPacket{
// 	// 	Pkt: SendPacketParams{
// 	// 		Packet: NewHexBytes(pktData),
// 	// 		Id:     "345",
// 	// 	},
// 	// }

// 	// dB, err := json.Marshal(sendPkt)
// 	// assert.NoError(t, err)

// 	// msg := &wasmtypes.MsgExecuteContract{
// 	// 	Sender:   addr.String(),
// 	// 	Contract: contract,
// 	// 	Msg:      dB,
// 	// }

// 	// a := pArch.TxFactory()
// 	// factory, err := pArch.PrepareFactory(a)
// 	// assert.NoError(t, err)

// 	// tx.GenerateOrBroadcastTxWithFactory(cliCtx, factory, msg)

// 	/////////////////////////////////////////////////
// 	/////////////////////// QUERY ///////////////////
// 	/////////////////////////////////////////////////

// 	type GetPacket struct {
// 		GetPacket struct {
// 			Id string `json:"id"`
// 		} `json:"get_packet"`
// 	}

// 	type PacketOutput struct {
// 		Packet []byte `json:"packet"`
// 	}

// 	// _param := GetPacket{
// 	// 	GetPacket: struct {
// 	// 		Id string "json:\"id\""
// 	// 	}{
// 	// 		Id: "100",
// 	// 	},
// 	// }

// 	// type GetAllPacket struct {
// 	// 	GetAllPacket interface{} `json:"get_packet"`
// 	// }

// 	cs := types.GetClientState{
// 		ClientState: struct {
// 			ClientId string "json:\"client_id\""
// 		}{
// 			ClientId: "iconclient-0",
// 		},
// 	}

// 	param, _ := json.Marshal(cs)

// 	queryCLient := wasmtypes.NewQueryClient(cliCtx)
// 	contractState, err := queryCLient.SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
// 		Address:   archway_mock_address,
// 		QueryData: param,
// 	})

// 	assert.NoError(t, err)
// 	e := contractState.Data
// 	var i icon_types.ClientState
// 	err = json.Unmarshal(e, &i)
// 	fmt.Printf("data is %s \n", e)
// 	assert.NoError(t, err)
// 	fmt.Printf("data is %+v \n", i)

// }

// func TestCreateClient(t *testing.T) {

// 	ctx := context.Background()
// 	ap, err := GetProvider(ctx, "archway1hpufl3l8g44aaz3qsqw886sjanhhu73ul6tllxuw3pqlhxzq9e4sqcz9uv", true) //"archway1g4w5f2l25dav7h4mc0mzeute5859wa9hgmavancmprfldqun6ppqsn0zma")
// 	assert.NoError(t, err)

// 	networkId := 1
// 	height := 59
// 	ip := GetIconProvider(networkId)

// 	btpHeader, err := ip.GetBtpHeader(int64(height))
// 	assert.NoError(t, err)

// 	header := icon.NewIconIBCHeader(btpHeader, nil, int64(height))

// 	clS, err := ip.NewClientState("07-tendermint", header, 100, 100, true, true)
// 	assert.NoError(t, err)

// 	msg, err := ap.MsgCreateClient(clS, header.ConsensusState())
// 	assert.NoError(t, err)

// 	call := make(chan bool)

// 	callback := func(rtr *provider.RelayerTxResponse, err error) {
// 		assert.NoError(t, err)
// 		fmt.Printf("Tx Response:: %+v\n ", rtr)
// 		call <- true
// 	}

// 	err = ap.SendMessagesToMempool(ctx, []provider.RelayerMessage{msg}, "memo", nil, callback)
// 	assert.NoError(t, err)
// namedLoop:
// 	for {
// 		select {
// 		case <-call:
// 			break namedLoop
// 		}
// 	}
// }

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
		// RPCAddr: "http://localhost:9999",
		Timeout: "20s",
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

// func TestGetClientState(t *testing.T) {
// 	ctx := context.Background()
// 	ap, err := GetProvider(ctx, "", false)
// 	assert.NoError(t, err)

// 	archwayP, ok := ap.(*ArchwayProvider)
// 	if !ok {
// 		assert.Fail(t, "failed to convert to archwayP")
// 	}

// 	state, err := archwayP.QueryClientStateContract(ctx, "iconclient-0")
// 	assert.NoError(t, err)
// 	fmt.Printf("ClentState %+v \n", state)

// }

// func TestDataDecode(t *testing.T) {

// 	d := []byte{10, 32, 47, 105, 99, 111, 110, 46, 108, 105, 103, 104, 116, 99, 108, 105, 101, 110, 116, 46, 118, 49, 46, 67, 108, 105, 101, 110, 116, 83, 116, 97, 116, 101, 18, 32, 127, 98, 36, 134, 45, 9, 198, 30, 199, 185, 205, 28, 128, 214, 203, 138, 15, 65, 45, 70, 134, 139, 202, 40, 61, 44, 97, 169, 50, 7, 225, 18}
// 	// d := "103247105991111104610810510310411699108105101110116461184946671081051011101168311697116101183212798361344591983019918520528128214203138156545701341392024061449716950722518"
// 	// b, err := hex.DecodeString(d)
// 	// assert.NoError(t, err)

// 	ctx := context.Background()
// 	ap, err := GetProvider(ctx, "archway123", false)
// 	assert.NoError(t, err)
// 	archwayP, _ := ap.(*ArchwayProvider)

// 	var iconee exported.ClientState
// 	err = archwayP.Cdc.Marshaler.UnmarshalInterface(d, &iconee)
// 	assert.NoError(t, err)
// 	fmt.Println(iconee.GetLatestHeight())
// }

func TestStructCast(t *testing.T) {

	type Struct1 struct {
		Fieldx int
		Fieldy []byte
	}
	type StructA struct {
		Field1 int
		Field2 string
		Field3 Struct1
	}

	type Struct2 struct {
		Fieldx int
		Fieldy []byte
	}
	type StructB struct {
		Field1 uint
		Field2 string
		Field3 Struct2
	}

	a := &StructA{
		Field1: 1,
		Field2: "helo",
		Field3: Struct1{
			Fieldx: 0,
			Fieldy: []byte("Hellllllllo"),
		},
	}

	b, _ := json.Marshal(a)
	var c StructB
	err := json.Unmarshal(b, &c)
	assert.NoError(t, err)
	assert.Equal(t, c, StructB{
		Field1: uint(a.Field1),
		Field2: a.Field2,
		Field3: Struct2{
			Fieldx: a.Field3.Fieldx,
			Fieldy: a.Field3.Fieldy,
		},
	})
}

// func TestArchwayLightHeader(t *testing.T) {
// 	ctx := context.Background()
// 	apx, err := GetProvider(ctx, "abcd", true)
// 	assert.NoError(t, err)

// 	ap := apx.(*ArchwayProvider)

// 	tsHeight := 34055
// 	cl := "07-tendermint-0"

// 	trustedIbcHeader, err := ap.QueryIBCHeader(ctx, int64(tsHeight))

// 	latestBlockHeader, err := ap.QueryIBCHeader(ctx, 34060)

// 	trustedHeight := clienttypes.Height{
// 		RevisionHeight: uint64(tsHeight),
// 		RevisionNumber: 0,
// 	}

// 	msg, err := ap.MsgUpdateClientHeader(latestBlockHeader, trustedHeight, trustedIbcHeader)
// 	assert.NoError(t, err)

// 	iconP := GetIconProvider(2)

// 	updateMessage, err := iconP.MsgUpdateClient(cl, msg)
// 	assert.NoError(t, err)
// 	fmt.Printf("%x \n ", updateMessage)

// 	// err = iconP.SendMessagesToMempool(ctx, []provider.RelayerMessage{updateMessage}, "", ctx, nil)
// 	// assert.Error(t, err)
// }

// func TestGetConsensusState(t *testing.T) {
// 	iconP := GetIconProvider(2)

// 	ctx := context.Background()

// 	op, err := iconP.QueryClientConsensusState(ctx, 200, "07-tendermint-34", clienttypes.NewHeight(0, 31600))
// 	assert.NoError(t, err)
// 	fmt.Println(op)
// }

func TestProtoMarshal(t *testing.T) {

	codec := MakeCodec(ModuleBasics, []string{})
	height := clienttypes.Height{
		RevisionHeight: 32318,
		RevisionNumber: 0,
	}
	expected, _ := hex.DecodeString("10befc01")
	b, err := codec.Marshaler.Marshal(&height)
	assert.NoError(t, err)
	assert.Equal(t, b, expected)

}

func TestDecodeProto(t *testing.T) {
	b := "0a086c6f63616c6e65741204080110031a0408c0a90722003898800140014801"
	by, _ := hex.DecodeString(b)

	var cl itm.ClientState
	codec := MakeCodec(ModuleBasics, []string{})
	err := codec.Marshaler.Unmarshal(by, &cl)
	assert.NoError(t, err)
	op := cl.LatestHeight
	fmt.Println(op)

}

// goloop rpc sendtx call \
// 	    --uri http://localhost:9082/api/v3 \
// 	    --nid 3 \
// 	    --step_limit 1000000000\
// 	    --to cxc327ce659d52f63f727c8d9e9503b8b9cced75f2 \
// 	    --method updateClient \
// 		--raw "{\"params\":{\"msg\":{\"clientId\":\"07-tendermint-0\",\"clientMessage\":\"0x0acc040a8f030a02080b12086c6f63616c6e6574188c8a02220b08bbdbb6a30610fcabd1512a480a20c50b8ec7e3a350b8f4952a23f98e94ca97960d969397625584b33ab2cf2ea45612240801122019bc479a78d29fe86494b54203583cabe3e01edf199f191dfa8f5ea8518b4add3220a7edff3caabb2dd4bfe9ac7c9dd2cd9049ec4332e0e7cf72ed8a78f6e5ea790a3a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8554220d299a92ba9789927c2f4e6bf0db8be41300d33aec0b3d53a37c0d7cf6035e3204a20d299a92ba9789927c2f4e6bf0db8be41300d33aec0b3d53a37c0d7cf6035e3205220048091bc7ddc283f77bfbf91d73c44da58c3df8a9cbc867405d8b7f3daada22f5a206e24d5124a854616c3689c32b2bd77278f45e7b12de588c712d4937f03106b156220e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8556a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85572146e5f3463982f10e154de349f627857b7acf5218912b701088c8a021a480a20a068896ce65463eb64ff67db82244d084fda128dc09bb2033d78ed3ee636f224122408011220ffc44c3d4d8af8084358d13e3e3174ef0c22b13be985e9fe00f929231929b4892267080212146e5f3463982f10e154de349f627857b7acf521891a0b08c0dbb6a30610f7b2c56722404bef8bdcbf7863684d589abe2af674ff3d418ce4516a4b209e1b1011f6124b0c2eaa15a22ee848e9b609f3357876f50170c9692a59ac4b0fca12311ef373a502123e0a3c0a146e5f3463982f10e154de349f627857b7acf5218912220a208636f208b278b8527a22c7bad921c39ece0594dc95ad213435140be2beaf380e180a18878a02223e0a3c0a146e5f3463982f10e154de349f627857b7acf5218912220a208636f208b278b8527a22c7bad921c39ece0594dc95ad213435140be2beaf380e180a\"}}}"\ \
// 	    --key_store /Users/viveksharmapoudel/keystore/godWallet.json\
// 	    --key_password gochain

// func TestBtpHeader(t *testing.T) {
// 	a := GetIconProvider(2)
// 	h, err := a.GetBtpHeader(1125)
// 	assert.NoError(t, err)
// 	fmt.Printf("%x \n ", h.PrevNetworkSectionHash)
// }

// func TestArchwayEvent(t *testing.T) {

// 	ctx := context.Background()
// 	pro, err := GetProvider(ctx, "archway17ymdtz48qey0lpha8erch8hghj37ag4dn0qqyyrtseymvgw6lfnqgmtsrj", true)
// 	assert.NoError(t, err)

// 	archwayP := pro.(*ArchwayProvider)

// 	height := int64(6343)
// 	blockRes, err := archwayP.RPCClient.BlockResults(ctx, &height)
// 	assert.NoError(t, err)

// 	for _, tx := range blockRes.TxsResults {
// 		if tx.Code != 0 {
// 			// tx was not successful
// 			continue
// 		}

// 		// fmt.Println(tx.Events)
// 		messages := ibcMessagesFromEvents(archwayP.log, tx.Events, archwayP.ChainId(), uint64(height), archwayP.PCfg.IbcHandlerAddress, true)

// 		assert.Equal(t, len(messages), 2)

// 		for _, m := range messages {

// 			fmt.Println(m.eventType)
// 			fmt.Printf("message is %+v \n", m.info)

// 		}
// 	}

// }

// func TestDecodeMerkleProof(t *testing.T) {

// 	v := common.MustHexStrToBytes("0x0ac90612c6060a7b03ade4a5f5803a439835c636395a8d648dee57b2fc90d98dc17fa887159b69638b30303062363336663665366536353633373436393666366537336230326433663334643963373863663565353734396637373131373861386361663034653731303432636366336636396165656430356230383066333336373712f5020a4503ade4a5f5803a439835c636395a8d648dee57b2fc90d98dc17fa887159b69638b0016636c69656e745f696d706c656d656e746174696f6e7369636f6e636c69656e742d3012442261726368776179316e633574617461667636657971376c6c6b7232677635306666396532326d6e66373071676a6c763733376b746d74346573777271676a33336736221a0b0801180120012a03000238222b0801120402043e201a2120a30ef45adecacce36447237e218f8cf3ad48357e82cae6aeea7df465573854cb22290801122504083e20fd6187d3aeb814e2a15d3987fd093b63aae12f9a04ba1c871e8a06c9f85a710b2022290801122508163e2064cfaa6db5902310f5d7255b7e8733f455699291f73d3988a17dc47348d63323202229080112250a2a3e2090d36297ce6f62cdb1110921e2482c20cd630e2817648bb0b95a42ce9fe081a420222b080112040c403e201a2120a8753c7dfe3f41e2bb9936721c8e0547e2c74a46a6d25c3f144d784204ceb86e1ace020a2e03ade4a5f5803a439835c636395a8d648dee57b2fc90d98dc17fa887159b69638b636f6e74726163745f696e666f12367b22636f6e7472616374223a226372617465732e696f3a63772d6962632d636f7265222c2276657273696f6e223a22302e312e30227d1a0b0801180120012a0300021422290801122502043e205a76cca2d1f3103d95080d98bf27abb862829151eb227f6be56d3dc8990d47182022290801122504083e20fd6187d3aeb814e2a15d3987fd093b63aae12f9a04ba1c871e8a06c9f85a710b2022290801122508163e2064cfaa6db5902310f5d7255b7e8733f455699291f73d3988a17dc47348d63323202229080112250a2a3e2090d36297ce6f62cdb1110921e2482c20cd630e2817648bb0b95a42ce9fe081a420222b080112040c403e201a2120a8753c7dfe3f41e2bb9936721c8e0547e2c74a46a6d25c3f144d784204ceb86e0a84010a81010a047761736d122031710a6b9c07bb7f1d7816f5b76f65d48e53ea30ad6d8138322f31374e8733321a090801180120012a0100222508011221011107704879ce264af2b8ca54a7ad461538067d296f22b7de0482e4fdf43314b922250801122101efb0c2cf8ed06dea231b3f0f26942e24623f13012e6297b343e7e1afc3863d6d")

// 	var op commitmenttypes.MerkleProof
// 	err := proto.Unmarshal(v, &op)
// 	assert.NoError(t, err)

// 	for i, v := range op.Proofs {
// 		fmt.Printf("index %d \n ", i)
// 		fmt.Printf("existence proof %x  : \n ", v.GetExist())
// 		fmt.Printf("Non-existence proof %x :  \n", v.GetNonexist())

// 	}

// 	// err = op.VerifyMembership([]*ics23.ProofSpec{ics23.IavlSpec, ics23.TendermintSpec}, root, path, result.Response.Value)
// 	assert.NoError(t, err)

// }

func TestCommitmentKey(t *testing.T) {
	fmt.Printf("%x \n ", common.GetConnectionCommitmentKey("connection-0"))

}

func TestGetProofTendermint(t *testing.T) {

	ctx := context.Background()
	contractAddr := "archway17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9jfksztgw5uh69wac2pgssf05p7"
	pro, err := GetProvider(ctx, contractAddr, true)
	assert.NoError(t, err)

	archwayP := pro.(*ArchwayProvider)

	connectionKey := common.GetConnectionCommitmentKey("connection-3")

	connStorageKey := fmt.Sprintf("%s%x", getKey(STORAGEKEY__Commitments), connectionKey)
	hexStrkey, err := hex.DecodeString(connStorageKey)
	assert.NoError(t, err)
	fmt.Printf("the main key is %x \n ", hexStrkey)

	proofConnBytes, err := archwayP.QueryArchwayProof(ctx, hexStrkey, int64(2273))

	var op icn.MerkleProof
	err = proto.Unmarshal(proofConnBytes, &op)
	assert.NoError(t, err)
	for ind, xx := range op.Proofs {
		fmt.Println("index ", ind)
		fmt.Printf("Get Exist  %x \n", xx.GetExist())
		fmt.Printf("non ExistP %x \n", xx.GetNonexist())
	}

}

// func TestVerifyMembership(t *testing.T) {

// 	ctx := context.Background()
// 	contractAddr := "archway17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9jfksztgw5uh69wac2pgssf05p7"
// 	pro, err := GetProvider(ctx, contractAddr, true)
// 	assert.NoError(t, err)
// 	archwayP := pro.(*ArchwayProvider)

// 	ibcAddr, err := sdk.AccAddressFromBech32(archwayP.PCfg.IbcHandlerAddress)
// 	assert.NoError(t, err)

// 	connectionKey := common.GetConnectionCommitmentKey("connection-0")
// 	fmt.Printf("commitment key is %x \n  ", connectionKey)

// 	// map_key := []byte("state")
// 	map_key := []byte("commitments")
// 	keyV := fmt.Sprintf("03%x000B%x%x", ibcAddr.Bytes(), map_key, connectionKey)
// 	// keyV := fmt.Sprintf("03%x00077374617465", ibcAddr.Bytes())
// 	key, _ := hex.DecodeString(keyV)
// 	fmt.Printf("contract Address  %x \n ", ibcAddr.Bytes())

// 	fmt.Printf("the main key is : %x%x \n", map_key, connectionKey)

// 	req := abci.RequestQuery{
// 		Path:   fmt.Sprintf("store/wasm/key"),
// 		Data:   key,
// 		Prove:  true,
// 		Height: 31,
// 	}

// 	path := commitmenttypes.MerklePath{KeyPath: []string{
// 		"wasm",
// 		string(key),
// 	}}

// 	fmt.Println("path is ", string(key))

// 	opts := rpcclient.ABCIQueryOptions{
// 		Height: req.Height,
// 		Prove:  req.Prove,
// 	}
// 	result, err := archwayP.RPCClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
// 	assert.NoError(t, err)

// 	rootB, _ := hex.DecodeString("49215D5CBBFEFD52D85166303F42ED61C7397079BE23AC13324B1C9B619EFF8B")
// 	root := commitmenttypes.MerkleRoot{Hash: rootB}

// 	rootMarshalled, _ := proto.Marshal(&root)

// 	fmt.Printf("proto marshalled root %x \n", rootMarshalled)

// 	proof, err := commitmenttypes.ConvertProofs(result.Response.ProofOps)
// 	assert.NoError(t, err)

// 	fmt.Printf("value  %x \n  ", result.Response.Value)
// 	proofProtoMarshal, err := proto.Marshal(&proof)
// 	fmt.Printf("proof %x \n ", proofProtoMarshal)

// 	err = proof.VerifyMembership([]*ics23.ProofSpec{ics23.IavlSpec, ics23.TendermintSpec}, root, path, result.Response.Value)
// 	assert.NoError(t, err)
// 	if err != nil {
// 		fmt.Println("failed to verify Memebership ", err)
// 	}
// }

// func TestVerifyMembershipTestCC(t *testing.T) {

// 	ctx := context.Background()
// 	contractAddr := "archway1999u8suptza3rtxwk7lspve02m406xe7l622erg3np3aq05gawxsk8g4pd" //START CONTRACT
// 	// contractAddr := "archway10qt8wg0n7z740ssvf3urmvgtjhxpyp74hxqvqt7z226gykuus7eqzla6h5"
// 	pro, err := GetProvider(ctx, contractAddr, true)
// 	assert.NoError(t, err)
// 	archwayP := pro.(*ArchwayProvider)

// 	ibcAddr, err := sdk.AccAddressFromBech32(archwayP.PCfg.IbcHandlerAddress)
// 	assert.NoError(t, err)

// 	// map_key := []byte("state")
// 	map_key := []byte("test_maphello")
// 	keyV := fmt.Sprintf("03%x0008%x", ibcAddr.Bytes(), map_key)
// 	// keyV := fmt.Sprintf("03%x00077374617465", ibcAddr.Bytes())
// 	key, _ := hex.DecodeString(keyV)
// 	fmt.Printf("contract Address  %x \n ", ibcAddr.Bytes())

// 	fmt.Printf("the main key is : %x \n", map_key)

// 	req := abci.RequestQuery{
// 		Path:   fmt.Sprintf("store/wasm/key"),
// 		Data:   key,
// 		Prove:  true,
// 		Height: 17084,
// 	}

// 	path := commitmenttypes.MerklePath{KeyPath: []string{
// 		"wasm",
// 		string(key),
// 	}}

// 	opts := rpcclient.ABCIQueryOptions{
// 		Height: req.Height,
// 		Prove:  req.Prove,
// 	}
// 	result, err := archwayP.RPCClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
// 	assert.NoError(t, err)

// 	rootB, _ := hex.DecodeString("7526C2B51C1FDCCD86BD4FAB4F0AF762242C50B321829B11D04E81B52DB83BBF")
// 	root := commitmenttypes.MerkleRoot{Hash: rootB}

// 	rootMarshalled, _ := proto.Marshal(&root)

// 	fmt.Printf("proto marshalled root %x \n", rootMarshalled)

// 	proof, err := commitmenttypes.ConvertProofs(result.Response.ProofOps)
// 	assert.NoError(t, err)

// 	fmt.Println("value \n  ", result.Response.Value)
// 	proofProtoMarshal, err := proto.Marshal(&proof)
// 	fmt.Printf("proof %x \n ", proofProtoMarshal)

// 	err = proof.VerifyMembership([]*ics23.ProofSpec{ics23.IavlSpec, ics23.TendermintSpec}, root, path, result.Response.Value)
// 	assert.NoError(t, err)
// 	if err != nil {
// 		fmt.Println("failed to verify Memebership ", err)
// 	}
// }

func TestGenRoot(t *testing.T) {

	rootB, _ := hex.DecodeString("99306EBA529FB6416B0984146B97C9C76386F226E9541A47197FA7ADA530EDA3")
	root := commitmenttypes.MerkleRoot{Hash: rootB}

	rootMarshalled, _ := proto.Marshal(&root)

	fmt.Printf("proto marshalled root %x \n", rootMarshalled)

}

func TestStringToHex(t *testing.T) {

	// type YY struct {
	// 	Req []byte
	// }

	// b := "5b332c3234322c3232362c3136322c38322c3231392c3131382c3231302c3130302c3139352c35312c3130382c33302c35382c3130372c37362c3130312c3232332c332c37322c3231312c32372c302c31302c3135372c3135362c3235312c3234312c3235342c38382c3233332c3230375d"
	// y, _ := hex.DecodeString(b)

	// x := []byte(fmt.Sprintf(`{"req":%s}`, y))
	// var a YY
	// err := json.Unmarshal(x, &a)
	// assert.NoError(t, err)
	// fmt.Printf("%x", a.Req)

	var byteArray []byte
	str := "[3,242,226,162,82,219,118,210,100,195,51,108,30,58,107,76,101,223,3,72,211,27,0,10,157,156,251,241,254,88,233,207]"

	err := json.Unmarshal([]byte(str), &byteArray)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("%x \n", byteArray)

}

func TestProtoUnmarshal(t *testing.T) {
	val, _ := hex.DecodeString("080210021a110a046d6f636b12096368616e6e656c2d30220c636f6e6e656374696f6e2d302a0769637332302d31")
	var channelS chantypes.Channel
	err := proto.Unmarshal(val, &channelS)
	assert.NoError(t, err)
	assert.Equal(t, channelS.State, chantypes.State(2))
}
