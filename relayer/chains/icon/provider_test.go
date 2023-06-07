package icon

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/common"
	icn "github.com/icon-project/IBC-Integration/libraries/go/common/icon"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	testCA = "cx54f9e239d347fcdf7f46976601a48231949bb671"
)

func TestConnectionDecode(t *testing.T) {

	input := types.HexBytes("0a0f30372d74656e6465726d696e742d3012230a0131120d4f524445525f4f524445524544120f4f524445525f554e4f524445524544180322200a0f30372d74656e6465726d696e742d30120d636f6e6e656374696f6e2d3533")
	var conn conntypes.ConnectionEnd
	_, err := HexBytesToProtoUnmarshal(input, &conn)
	if err != nil {
		fmt.Println("error occured", err)
		return
	}

	assert.Equal(t, conn.ClientId, "07-tendermint-0")
}

func GetMockIconProvider(network_id int, contractAddress string) *IconProvider {

	absPath, _ := filepath.Abs("../../../env/godWallet.json")

	pcfg := IconProviderConfig{
		Keystore:          absPath,
		Password:          "gochain",
		ICONNetworkID:     3,
		BTPNetworkID:      int64(network_id),
		BTPNetworkTypeID:  1,
		IbcHandlerAddress: contractAddress,
		RPCAddr:           "http://localhost:9082/api/v3",
		// RPCAddr: "http://localhost:9999",
		Timeout: "20s",
	}
	log, _ := zap.NewProduction()
	p, _ := pcfg.NewProvider(log, "", false, "icon")

	iconProvider, _ := p.(*IconProvider)
	return iconProvider
}

func TestNetworkSectionHashCheck(t *testing.T) {

	prevNetworkSectionHash, _ := hex.DecodeString("b791b4b069c561ca31093f825f083f6cc3c8e5ad5135625becd2ff77a8ccfa1e")
	messageRoot, _ := hex.DecodeString("84d8e19eb09626e4a94212d3a9db54bc16a75dfd791858c0fab3032b944f657a")
	nextProofContextHash, _ := hex.DecodeString("d090304264eeee3c3562152f2dc355601b0b423a948824fd0a012c11c3fc2fb4")
	header := types.BTPBlockHeader{
		MainHeight:             27,
		Round:                  0,
		NextProofContextHash:   nextProofContextHash,
		NetworkID:              1,
		UpdateNumber:           0,
		PrevNetworkSectionHash: prevNetworkSectionHash,
		MessageCount:           1,
		MessageRoot:            messageRoot,
	}
	networkSectionhash := types.NewNetworkSection(&header).Hash()
	expectNetworkSection, _ := hex.DecodeString("aa517deb1e03f1d461e0f463fa5ebd0126d8a9153fde80778d7d1a1bdfa050fc")
	assert.Equal(t, networkSectionhash, expectNetworkSection)
}

func TestMsgOpenTryProof(t *testing.T) {
	connOpenTry := common.MustHexStrToBytes("0x0a0c69636f6e636c69656e742d301a3f0a1d2f74656e6465726d696e742e6c696768742e436c69656e745374617465121e0a086c6f63616c6e65741204080110031a0508809c9c3938f1574001480122260a0f30372d74656e6465726d696e742d30120c636f6e6e656374696f6e2d301a050a0369626332230a0131120d4f524445525f4f524445524544120f4f524445525f554e4f5244455245443a0410b2e70442280a0208010a22122020e8ad866d375237abaf35961d5086ab95e51a4671d00fb582ecb0171e6509f14a4c0a240801122060a6eacf908ed8c0448ceee36a02bac97e9c269d094ebdeb68d94e0d61e214790a240801122035216273847e1b55f467bfe28faccc9b4cdf7b4fb28e2e78b6aa4a190bafc375524a0a221220fa7a698e60b3ce425bbafba23a24cf7801cdca2ad19419bff6b9ff8e94c92e2e0a240801122035216273847e1b55f467bfe28faccc9b4cdf7b4fb28e2e78b6aa4a190bafc3755a0310f157622e617263687761793164787979787968797035686b7a676c3372776b6a666b63773530333032636137676539647072")
	msgRoot := common.MustHexStrToBytes("4d9b87ce82ba0a1b32791f085cb2785b9e84a9e17f727a8022457186397ccc0f")
	// consensusStateByte := common.MustHexStrToBytes("0a0c0896cfc6a30610fd9ac78e0312220a201b6b08c8498a9e9735222491c808b2d34bff22e7e847fb3bb5ca6ab2e506bb741a20e2bdc5125e6de6ff7159ae2618d02488b6c3b78abc51e04b93f46ec8d762bfa7")

	codec := MakeCodec(ModuleBasics, []string{})

	// MsgOpenTry
	var msgOpenTry conntypes.MsgConnectionOpenTry
	err := codec.Marshaler.Unmarshal(connOpenTry, &msgOpenTry)
	assert.NoError(t, err)

	// checking clientState Proof

	fmt.Printf("vlaue is %x : -> \n ", common.GetClientStatePath(msgOpenTry.Counterparty.ClientId))
	key := common.GetClientStateCommitmentKey(msgOpenTry.Counterparty.ClientId)
	fmt.Println("clientState value :", msgOpenTry.ClientState.Value)

	approved, err := VerifyProof(key, msgOpenTry.ClientState.Value, msgRoot, msgOpenTry.ProofClient)
	assert.NoError(t, err)
	assert.True(t, approved, "failed to verify client state")

	//checking consensus state
	var cs exported.ClientState
	err = codec.Marshaler.UnpackAny(msgOpenTry.ClientState, &cs)
	assert.NoError(t, err)
	key = common.GetConsensusStateCommitmentKey(msgOpenTry.Counterparty.ClientId,
		big.NewInt(0),
		big.NewInt(int64(cs.GetLatestHeight().GetRevisionHeight())))

	// approved, err = VerifyProof(key, consensusStateByte, msgRoot, msgOpenTry.ProofConsensus)
	// assert.NoError(t, err)
	// assert.True(t, approved, "failed to validate consensus state")

	// checking connectionState
	expectedConn := icn.ConnectionEnd{
		ClientId: msgOpenTry.Counterparty.ClientId,
		Versions: []*icn.Version{(*icn.Version)(msgOpenTry.CounterpartyVersions[0])},
		State:    icn.ConnectionEnd_STATE_INIT,

		Counterparty: &icn.Counterparty{
			ClientId:     msgOpenTry.ClientId,
			ConnectionId: "",
			Prefix:       &defaultChainPrefix,
		},
	}
	key = common.GetConnectionCommitmentKey("connection-0")
	fmt.Printf("connection Path : %x \n", key)
	expectedConnByte, err := codec.Marshaler.Marshal(&expectedConn)
	assert.NoError(t, err)
	fmt.Printf("connection value: %x \n", expectedConnByte)
	fmt.Printf("connection value hashed: %x \n", common.Sha3keccak256(expectedConnByte))
	approved, err = VerifyProof(key, expectedConnByte, msgRoot, msgOpenTry.ProofInit)
	assert.NoError(t, err)
}

// func TestConnectionProof(t *testing.T) {

// 	icp := GetMockIconProvider(1, testCA)

// 	height := int64(22612)
// 	clientID := "07-tendermint-0"
// 	connID := "connection-0"
// 	ctx := context.Background()

// 	// proof generation
// 	connHandshakeProof, err := icp.ConnectionHandshakeProof(ctx, provider.ConnectionInfo{
// 		ClientID: clientID,
// 		ConnID:   connID,
// 	}, uint64(height))
// 	assert.NoError(t, err)

// 	// btpHeader
// 	btpHeader, err := icp.GetBtpHeader(height)
// 	assert.NoError(t, err)
// 	messageRoot := btpHeader.MessageRoot

// 	// clientState
// 	cs, err := icp.QueryClientState(ctx, height, clientID)
// 	assert.NoError(t, err)

// 	// checking the clientstate
// 	clientState, err := icp.QueryClientStateResponse(ctx, height, clientID)
// 	leaf := getCommitmentHash(common.GetClientStateCommitmentKey(clientID), clientState.ClientState.Value)
// 	// checking for the clientState
// 	var clientProofs icn.MerkleProofs
// 	err = proto.Unmarshal(connHandshakeProof.ClientStateProof, &clientProofs)
// 	assert.NoError(t, err)
// 	assert.True(t, cryptoutils.VerifyMerkleProof(messageRoot, leaf, clientProofs.Proofs))

// 	// checking the consensusState
// 	consensusHeight := cs.GetLatestHeight()
// 	key := common.GetConsensusStateCommitmentKey(clientID,
// 		big.NewInt(int64(consensusHeight.GetRevisionNumber())),
// 		big.NewInt(int64(consensusHeight.GetRevisionHeight())))
// 	consensusState, err := icp.QueryClientConsensusState(ctx, height, clientID, cs.GetLatestHeight())
// 	assert.NoError(t, err)
// 	fmt.Println("val:", consensusState.ConsensusState)

// 	commitmentHash := getCommitmentHash(key, consensusState.ConsensusState.Value)

// 	var consensuProofs icn.MerkleProofs
// 	err = proto.Unmarshal(connHandshakeProof.ConsensusStateProof, &consensuProofs)
// 	assert.NoError(t, err)
// 	assert.True(t, cryptoutils.VerifyMerkleProof(btpHeader.MessageRoot, commitmentHash, consensuProofs.Proofs))

// 	// checking the connectionState
// 	expectedConn := icn.ConnectionEnd{
// 		ClientId: clientID,
// 		Versions: []*icn.Version{DefaultIBCVersion},
// 		State:    icn.ConnectionEnd_STATE_INIT,
// 		// DelayPeriod: 0,

// 		Counterparty: &icn.Counterparty{
// 			ClientId:     "iconclient-0",
// 			ConnectionId: "",
// 			Prefix: &icn.MerklePrefix{
// 				KeyPrefix: []byte("ibc"),
// 			},
// 		},
// 	}

// 	expectedConnByte, err := proto.Marshal(&expectedConn)
// 	assert.NoError(t, err)

// 	callParam := icp.prepareCallParams(MethodGetConnection, map[string]interface{}{
// 		"connectionId": connID,
// 	}, callParamsWithHeight(types.NewHexInt(height)))

// 	var conn_string_ types.HexBytes
// 	err = icp.client.Call(callParam, &conn_string_)
// 	actual_connection, err := conn_string_.Value()
// 	assert.NoError(t, err)

// 	fmt.Println("exect connection ", conn_string_)
// 	fmt.Printf("exect marshal byte %x \n ", expectedConnByte)
// 	assert.Equal(t, expectedConnByte, actual_connection)
// 	commitmentHash = getCommitmentHash(common.GetConnectionCommitmentKey(connID), expectedConnByte)

// 	// proofs:
// 	var connectionProofs icn.MerkleProofs
// 	err = proto.Unmarshal(connHandshakeProof.ConnectionStateProof, &connectionProofs)
// 	assert.NoError(t, err)
// 	assert.True(t, cryptoutils.VerifyMerkleProof(btpHeader.MessageRoot, commitmentHash, connectionProofs.Proofs))
// }

// func TestClientProofOnly(t *testing.T) {

// 	icp := GetMockIconProvider(1, testCA)

// 	height := int64(22612)
// 	clientID := "07-tendermint-0"
// 	connID := "connection-0"
// 	ctx := context.Background()

// 	// proof generation
// 	connHandshakeProof, err := icp.ConnectionHandshakeProof(ctx, provider.ConnectionInfo{
// 		ClientID: clientID,
// 		ConnID:   connID,
// 	}, uint64(height))
// 	assert.NoError(t, err)

// 	// btpHeader
// 	btpHeader, err := icp.GetBtpHeader(height)
// 	assert.NoError(t, err)
// 	messageRoot := btpHeader.MessageRoot

// 	fmt.Printf("actual root %x \n", messageRoot)

// 	// checking the clientstate
// 	clientState, err := icp.QueryClientStateResponse(ctx, height, clientID)
// 	leaf := getCommitmentHash(common.GetClientStateCommitmentKey(clientID), clientState.ClientState.Value)
// 	// checking for the clientState
// 	fmt.Printf("Commitment path %x \n", common.GetClientStateCommitmentKey(clientID))
// 	fmt.Printf("clientState value %x \n", clientState.ClientState.Value)
// 	fmt.Printf("Leaf is %x \n", leaf)

// 	var clientProofs icn.MerkleProofs
// 	err = proto.Unmarshal(connHandshakeProof.ClientStateProof, &clientProofs)
// 	assert.NoError(t, err)
// 	fmt.Printf("calcuated root %x \n", cryptoutils.CalculateRootFromProof(leaf, clientProofs.Proofs))
// 	assert.True(t, cryptoutils.VerifyMerkleProof(messageRoot, leaf, clientProofs.Proofs))

// }

// func TestConsensusProofOny(t *testing.T) {

// 	icp := GetMockIconProvider(1, testCA)
// 	height := int64(22612)
// 	clientID := "07-tendermint-0"
// 	connID := "connection-0"
// 	ctx := context.Background()

// 	// clientState
// 	cs, err := icp.QueryClientState(ctx, height, clientID)
// 	assert.NoError(t, err)

// 	// proof generation
// 	connHandshakeProof, err := icp.ConnectionHandshakeProof(ctx, provider.ConnectionInfo{
// 		ClientID: clientID,
// 		ConnID:   connID,
// 	}, uint64(height))
// 	assert.NoError(t, err)

// 	btpHeader, err := icp.GetBtpHeader(height)
// 	assert.NoError(t, err)

// 	consensusHeight := cs.GetLatestHeight()
// 	key := common.GetConsensusStateCommitmentKey(clientID,
// 		big.NewInt(int64(consensusHeight.GetRevisionNumber())),
// 		big.NewInt(int64(consensusHeight.GetRevisionHeight())))
// 	consensusState, err := icp.QueryClientConsensusState(ctx, height, clientID, cs.GetLatestHeight())
// 	assert.NoError(t, err)

// 	commitmentHash := getCommitmentHash(key, consensusState.ConsensusState.Value)

// 	var consensuProofs icn.MerkleProofs
// 	err = proto.Unmarshal(connHandshakeProof.ConsensusStateProof, &consensuProofs)
// 	assert.NoError(t, err)
// 	assert.True(t, cryptoutils.VerifyMerkleProof(btpHeader.MessageRoot, commitmentHash, consensuProofs.Proofs))

// }

// func TestConnectionProofOny(t *testing.T) {

// 	icp := GetMockIconProvider(1, testCA)
// 	height := int64(22612)
// 	clientID := "07-tendermint-0"
// 	connID := "connection-0"
// 	ctx := context.Background()

// 	// proof generation
// 	connHandshakeProof, err := icp.ConnectionHandshakeProof(ctx, provider.ConnectionInfo{
// 		ClientID: clientID,
// 		ConnID:   connID,
// 	}, uint64(height))
// 	assert.NoError(t, err)

// 	btpHeader, err := icp.GetBtpHeader(height)
// 	assert.NoError(t, err)

// 	// checking the connectionState
// 	expectedConn := icn.ConnectionEnd{
// 		ClientId: clientID,
// 		Versions: []*icn.Version{DefaultIBCVersion},
// 		State:    icn.ConnectionEnd_STATE_INIT,
// 		// DelayPeriod: 0,

// 		Counterparty: &icn.Counterparty{
// 			ClientId:     "iconclient-0",
// 			ConnectionId: "",
// 			Prefix: &icn.MerklePrefix{
// 				KeyPrefix: []byte("ibc"),
// 			},
// 		},
// 	}

// 	expectedConnByte, err := proto.Marshal(&expectedConn)
// 	assert.NoError(t, err)

// 	callParam := icp.prepareCallParams(MethodGetConnection, map[string]interface{}{
// 		"connectionId": connID,
// 	}, callParamsWithHeight(types.NewHexInt(height)))

// 	var conn_string_ types.HexBytes
// 	err = icp.client.Call(callParam, &conn_string_)
// 	actual_connection, err := conn_string_.Value()
// 	assert.NoError(t, err)

// 	assert.Equal(t, expectedConnByte, actual_connection)
// 	commitmentHash := getCommitmentHash(common.GetConnectionCommitmentKey(connID), actual_connection)

// 	fmt.Printf("test connection commitmenthash %x \n", commitmentHash)

// 	// proofs:
// 	var connectionProofs icn.MerkleProofs
// 	err = proto.Unmarshal(connHandshakeProof.ConnectionStateProof, &connectionProofs)
// 	assert.NoError(t, err)

// 	assert.True(t, cryptoutils.VerifyMerkleProof(btpHeader.MessageRoot, commitmentHash, connectionProofs.Proofs))

// }

// func TestChannelOpenTry(t *testing.T) {

// 	icp := GetMockIconProvider(10, testCA)

// 	ctx := context.Background()
// 	p, err := icp.ChannelProof(ctx, provider.ChannelInfo{Height: 39704, PortID: "mock", ChannelID: "channel-0"}, 39704)
// 	assert.NoError(t, err)

// 	fmt.Println("the proof is ", p)
// }

func TestCase(t *testing.T) {

	// var byteArray []byte
	// str := "[237,251,49,138,154,148,89,201,134,105,90,10,197,188,15,78,147,228,42,239,95,31,53,224,29,119,46,191,132,161,62,222]"

	// err := json.Unmarshal([]byte(str), &byteArray)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// fmt.Printf("%x \n ", byteArray)

	b, _ := hex.DecodeString("0a0f30372d74656e6465726d696e742d3112230a0131120d4f524445525f4f524445524544120f4f524445525f554e4f5244455245441803222b0a0c69636f6e636c69656e742d30120c636f6e6e656374696f6e2d301a0d0a0b636f6d6d69746d656e7473")

	byteHash := common.Sha3keccak256(b)

	common.Sha3keccak256()
	fmt.Printf("heashed value %x \n", byteHash)

}

// goloop rpc sendtx call \
//     --uri http://localhost:9082/api/v3  \
//     --nid 3 \
//     --step_limit 1000000000\
//     --to cx41b7ad302add7ab50e3e49e9c0ebd778121f797b \
//     --method sendCallMessage \
//     --param _to=eth \
// 	--param _data=0x6e696c696e \
//     --key_store /Users/viveksharmapoudel/keystore/godWallet.json \
//     --key_password gochain

// func TestIConProof(t *testing.T) {

// 	icp := GetMockIconProvider(2, testCA)

// 	ctx := context.Background()
// 	keyhash, _ := hex.DecodeString("d6cae344c4204e849379faefa0048b0e38ce3ae9b10eded15e53e150f4912cf9331088538c700235f7ef61cb7a4a242399696e6bd2d5f8775f99cfd704345c67")

// 	p, err := icp.QueryIconProof(ctx, 4690, keyhash)
// 	assert.NoError(t, err)

// 	fmt.Println("the proof is ", p)
// }

func TestHash(t *testing.T) {
	// b, _ := hex.DecodeString("000000000000000000000000000000000000000002fb02d168e50d427cf9b439c3b643e3967b0ee9141da9296543e488d78182e42392cf99")
	// assert.Equal(common.Sha3keccak256(b))
}

// goloop rpc sendtx call \
//     --uri http://localhost:9082/api/v3  \
//     --nid 3 \
//     --step_limit 1000000000\
//     --to cxc3c1f693b1616860d9f709d9c85b5f613ea2dbdb \
//     --method sendCallMessage \
//     --param _to=eth \
//         --param _data=0x6e696c696e \
//     --key_store /Users/viveksharmapoudel/keystore/godWallet.json \
//     --key_password gochain
