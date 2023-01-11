package finality

import (
	"fmt"
	"github.com/ChainSafe/chaindb"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	"github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	grandpatypes "github.com/cosmos/relayer/v2/relayer/chains/substrate/finality/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConstructGrandpaStructs(t *testing.T) {
	paraClient, err := rpcclient.NewSubstrateAPI("ws://localhost:9188")
	require.NoError(t, err)
	relayClient, err := rpcclient.NewSubstrateAPI("ws://localhost:9955")
	require.NoError(t, err)
	paraID := 2000
	relayChain := 0
	memDB, err := chaindb.NewBadgerDB(&chaindb.Config{
		InMemory: true,
	})
	require.NoError(t, err)
	clientState := grandpatypes.ClientState{
		LatestRelayHash:    nil,
		LatestRelayHeight:  0,
		CurrentSetId:       0,
		XFrozenHeight:      nil,
		RelayChain:         0,
		ParaId:             0,
		LatestParaHeight:   0,
		CurrentAuthorities: nil,
	}
	grandpa := NewGrandpa(paraClient, relayClient, uint32(paraID), int32(relayChain), memDB, &clientState)

	t.Run("sends in a single batch when there are no limits", func(t *testing.T) {
		//d, err := hex.DecodeString("1900000000000000d08840544af2b9f45471440c52495767be1bd8cc507fb4e136e6bf38761ee2270b18000014d08840544af2b9f45471440c52495767be1bd8cc507fb4e136e6bf38761ee2270b1800001f9cd97b8959bd110915e2307d4524626f7cd9fa264ff19ebaec11624d764442da4758bb98299315e31cb0679b841fd75b2aabc9a7b143d85d54c5e81dcfbb091dfe3e22cc0d45c70779c1095f7489a8ef3cf52d62fbd8c2fa38c9f1723502b5d08840544af2b9f45471440c52495767be1bd8cc507fb4e136e6bf38761ee2270b180000a8ae0959e5c3cc83ab3567513443efc0534c932866db6da660e7148af6737bb011c8f617abc1ff00fef18c32288e205f53b15dcb641b549a2183187b8b99fe0d439660b36c6c03afafca027b910b4fecf99801834c62a5e6006f27d978de234fd08840544af2b9f45471440c52495767be1bd8cc507fb4e136e6bf38761ee2270b18000065c2e24ee967783d3adbda8fffd16592b1377a3851c21770136e00f2f32a5a9df4818e3a390e408327da3eee86f4852874aa62b56d7f9e4d53053626d7012c06568cb4a574c6d178feb39c27dfc8b3f789e5f5423e19c71633c748b9acf086b5d08840544af2b9f45471440c52495767be1bd8cc507fb4e136e6bf38761ee2270b180000e11a2c6c1d1afacd69a62411704b772806b474823deefd5176213610810e3af22a7d1fb1d5960a875083cc62368671a3fe8ce8569b012d6acfe4c3509e351e0988dc3417d5058ec4b4503e0c12ea1a0a89be200fe98922423d4334014fa6b0eed08840544af2b9f45471440c52495767be1bd8cc507fb4e136e6bf38761ee2270b1800006432338d5b6febf71a4b2f4808300a5656e495e05e412b3424d30e7de794edb13ef0ab286b72c8aee4e58d0310ba3e874fea7e9746f97a31672b85aea52bc60fd17c2d7823ebf260fd138f2d7e27d114c0145d968b5ff5006125f2414fadae6900")
		//justification := GrandpaJustification{}
		//err = justification.Decode(codec.NewDecoder(bytes.NewReader(d)))
		//justification.prettyPrint()

		//head, err := relayClient.RPC.Chain.GetFinalizedHead()
		relayHead, err := relayClient.RPC.Chain.GetFinalizedHead()
		relayHeader, err := relayClient.RPC.Chain.GetHeader(relayHead)

		require.NoError(t, err)
		setId, err := grandpa.getCurrentSetId(relayHead)
		require.NoError(t, err)
		fmt.Println(setId)

		latestFinalizedParaHeader, err := grandpa.getParachainHeader(relayHead)
		require.NoError(t, err)
		fmt.Println(latestFinalizedParaHeader)

		latestFinalizedParaHeight := latestFinalizedParaHeader.Number - 10
		latestFinalizedRelayHeight := relayHeader.Number - 10
		println("para height = ", latestFinalizedParaHeight)
		println("relay height = ", latestFinalizedRelayHeight)
		clientState := grandpatypes.ClientState{
			LatestParaHeight:  uint32(latestFinalizedParaHeight),
			LatestRelayHeight: uint32(latestFinalizedRelayHeight),
		}

		blockNums := make([]types.BlockNumber, 0)
		for i := types.BlockNumber(0); i < 5; i++ {
			blockNums = append(blockNums, latestFinalizedParaHeight+i)
		}

		p, err := grandpa.queryFinalizedParachainHeadersWithProof(&clientState, uint32(latestFinalizedRelayHeight), blockNums)
		require.NoError(t, err)
		fmt.Println(p)

		//bn := uint64(220)
		//fp, err := grandpa.constructFinalityProof(bn+1, bn)
		//require.NoError(t, err)
		//fmt.Println(fp)

		//header, err := grandpa.constructHeader()
		//require.NoError(t, err)
		//grandpa.grandpaJustification()
	})
	//	require.Equal(t, result.SuccessfulSrcBatches, 1)
	//	require.Equal(t, result.SuccessfulDstBatches, 1)
	//	require.NoError(t, result.SrcSendError)
	//	require.NoError(t, result.DstSendError)
	//
	//	require.Equal(t, []provider.RelayerMessage{srcMsg}, srcSent)
	//	require.Equal(t, []provider.RelayerMessage{dstMsg}, dstSent)
	//})
}

func TestTrieDBMut(t *testing.T) {
	trie := NewTrieDBMut()
	defer trie.Free()

	assert.Equal(t, trie.Root().Hex(), "0x03170a2e7597b7b7e3d84c05391d139a62b157e78786d8c082f29dcf4c111314")

	trie.Insert([]byte("key"), []byte("value"))
	assert.Equal(t, trie.Root().Hex(), "0x434590ba666a2d9ed9f2ca8bde0a2e876b1a744878e8522e9bc2b88c91e6c2c0")

	trie.Insert([]byte("key2"), []byte("value2"))
	root := trie.Root()
	assert.Equal(t, root.Hex(), "0x7ff11e72bc15dbb00caf226cf4b2d0f9337dc5a93686b735f475b491873cd2a1")

	proof := trie.GenerateTrieProof(root, []byte("key"))
	defer proof.Free()
	res := trie.VerifyTrieProof(root, []byte("key"), []byte("value"), proof)
	assert.Equal(t, res, true)
}

func TestTrieDBMutReadProofCheck(t *testing.T) {
	// this proof was constructed in the test "test_read_proof_check"
	proofEncoded := []string{
		"409901910105050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505050505", "800c004c4044400303030303030303030303030303030380d16a677a16524bef8badf443d5aaeed985cb8e5f7571e02a09f64c12e7ce69ea", "8048008089ed3d18cf2f5691d424ef3f5b0a79466fbe54eed8d66cff3df95cbf7a407fba80915d72d4c51fe5fbe0555e8b06df295d9cc5c77405f0678fbb82ad652946a091",
		"c50b65790800a90fa10f0101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010180f1229154b723fe17f7a8327b2c91fbec5908da10bc4f2eeb1fde7cb62fdcb382",
	}
	proof := make([][]byte, 0)
	for _, p := range proofEncoded {
		proof = append(proof, common.FromHex(p))
	}
	root := types.NewH256(common.FromHex("5a28562884fa61b7fa449bbd4411604bff2e6069e5ce6fd0930095080abc6cd4"))
	trieProof := NewTrieProof(proof)
	ReadProofCheck(types.Hash(root), trieProof, []byte("key"))
}
