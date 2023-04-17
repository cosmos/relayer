package icon

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/wallet"
	"github.com/icon-project/goloop/module"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func NewTestClient() *Client {
	uri := "https://lisbon.net.solidwallet.io/api/v3"
	l := zap.NewNop()
	return NewClient(uri, l)
}

func getTestWallet() (module.Wallet, error) {

	keyStore_file := "/Users/viveksharmapoudel/keystore/god_wallet.json"
	kpass := "gochain"

	keystore_bytes, err := ioutil.ReadFile(keyStore_file)
	if err != nil {
		return nil, err
	}

	wallet, err := wallet.NewFromKeyStore(keystore_bytes, []byte(kpass))
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func TestClientSetup(t *testing.T) {
	l := zap.Logger{}
	i := NewClient("https://ctz.solidwallet.io/api/v3", &l)

	hash := "0x5306e343d648250f0567e9b549d3c03430aa0ab5a80dffc944cb0db3dbe4ed74"
	param := &types.TransactionHashParam{Hash: types.HexBytes(hash)}
	res, err := i.GetTransactionResult(param)
	require.NoError(t, err)
	assert.Equal(t, types.HexInt("0x1"), res.Status)
}

func TestTransaction(t *testing.T) {

	c := NewTestClient()

	rpcWallet, err := getTestWallet()
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}

	txParam := &types.TransactionParam{
		Version:     types.NewHexInt(types.JsonrpcApiVersion),
		FromAddress: types.Address(rpcWallet.Address().String()),
		ToAddress:   types.Address("cx6e24351b49133f2337a01c968cb864958ffadce8"),
		Timestamp:   types.NewHexInt(time.Now().UnixNano() / int64(time.Microsecond)),
		NetworkID:   types.NewHexInt(2),
		StepLimit:   types.NewHexInt(int64(1000000000)),
		DataType:    "call",
		Data: types.CallData{
			Method: "sendEvent",
		},
	}

	err = c.SignTransaction(rpcWallet, txParam)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}

	op, err := c.SendTransaction(txParam)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}

	time.Sleep(4 * time.Second)

	finalOp, err := c.GetTransactionResult(&types.TransactionHashParam{Hash: *op})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, types.HexInt("0x1"), finalOp.Status)

	t.Log(finalOp)

}

func TestCallFunction(t *testing.T) {

	c := NewTestClient()

	w, err := getTestWallet()
	if err != nil {
		t.Fatal(err)
		return
	}
	var op types.HexBytes
	err = c.Call(&types.CallParam{
		FromAddress: types.Address(w.Address().String()),
		ToAddress:   types.Address("cx6e24351b49133f2337a01c968cb864958ffadce8"),
		DataType:    "call",
		Data: &types.CallData{
			Method: "name",
		},
	}, &op)

	if err != nil {
		t.Fatal((err))
		return
	}

	assert.Equal(t, types.HexBytes("Handler"), op)

	t.Log(op)

}

func TestGetTransaction(t *testing.T) {

	c := NewTestClient()
	hashString := "0xa9d333b24d990aeb418582c1467a4e6fd86a1bf9fb57e8fa95a77cb632a52301"
	op, err := c.GetTransactionResult(&types.TransactionHashParam{Hash: types.HexBytes(hashString)})
	if err != nil {
		t.Log(err)
		return
	}

	var p icon.Packet
	packetByte, err := types.HexBytes(op.EventLogs[0].Indexed[1]).Value()
	if err != nil {
		t.Fatal(err)
	}
	_, err = codec.RLP.UnmarshalFromBytes(packetByte, &p)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Data:", p)

}
