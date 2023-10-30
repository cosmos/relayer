package icon

import (
	"encoding/hex"
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	"github.com/icon-project/ibc-integration/libraries/go/common/icon"
	tendermint_client "github.com/icon-project/ibc-integration/libraries/go/common/tendermint"
	"github.com/stretchr/testify/assert"
)

func TestCodec(t *testing.T) {

	counterparty := &icon.Counterparty{
		ClientId:     "07-tendermint-0",
		ConnectionId: "connection-0",
		Prefix:       &icon.MerklePrefix{},
	}

	byt, e := proto.Marshal(counterparty)
	assert.NoError(t, e)
	assert.NotNil(t, byt)

	var co icon.Counterparty
	e = proto.Unmarshal(byt, &co)
	assert.NoError(t, e)
	assert.Equal(t, counterparty, &co)
}

func TestClientState(t *testing.T) {
	clS := "0a0469636f6e1204080210031a0310e80722051080b899292a0308d80432003a02105942190a090801180120012a0100120c0a02000110211804200c300142190a090801180120012a0100120c0a020001102018012001300150015801"
	clB, _ := hex.DecodeString(clS)

	var client tmclient.ClientState
	err := proto.Unmarshal(clB, &client)
	assert.NoError(t, err)
}

func TestCodecEncode(t *testing.T) {

	testData := tendermint_client.ClientState{
		ChainId: "tendermint",
		LatestHeight: &icon.Height{
			RevisionHeight: 40,
		},
	}

	codec := MakeCodec(ModuleBasics, []string{})
	data, err := codec.Marshaler.MarshalInterface(&testData)
	if err != nil {
		assert.Fail(t, "couldn't marshal interface ")
	}
	var ptr exported.ClientState
	err = codec.Marshaler.UnmarshalInterface(data, &ptr)
	if err != nil {
		assert.Fail(t, "Couldn't unmarshal interface ")
	}
	assert.Equal(t, ptr.GetLatestHeight().GetRevisionHeight(), uint64(40))

}
