package test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	ibctmtypes "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	ibctesting "github.com/cosmos/cosmos-sdk/x/ibc/testing"
	ibctestingmock "github.com/cosmos/cosmos-sdk/x/ibc/testing/mock"
	"github.com/cosmos/relayer/relayer"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmprotoversion "github.com/tendermint/tendermint/proto/tendermint/version"
	tmtypes "github.com/tendermint/tendermint/types"
	tmversion "github.com/tendermint/tendermint/version"
)

var (
	gaiaChains = []testChain{
		{"ibc-0", 0, gaiaTestConfig},
		{"ibc-1", 1, gaiaTestConfig},
	}
)

func TestGaiaToGaiaStreamingRelayer(t *testing.T) {
	chains := spinUpTestChains(t, gaiaChains...)

	var (
		src         = chains.MustGet("ibc-0")
		dst         = chains.MustGet("ibc-1")
		testDenom   = "samoleans"
		testCoin    = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		twoTestCoin = sdk.NewCoin(testDenom, sdk.NewInt(2000))
	)

	path, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	// query initial balances to compare against at the end
	srcExpected, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	dstExpected, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)

	// create path
	_, err = src.CreateClients(dst)
	require.NoError(t, err)
	testClientPair(t, src, dst)

	_, err = src.CreateOpenConnections(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testConnectionPair(t, src, dst)

	_, err = src.CreateOpenChannels(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testChannelPair(t, src, dst)

	// send a couple of transfers to the queue on src
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress().String(), 0, 0))
	require.NoError(t, src.SendTransferMsg(dst, testCoin, dst.MustGetAddress().String(), 0, 0))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress().String(), 0, 0))
	require.NoError(t, dst.SendTransferMsg(src, testCoin, src.MustGetAddress().String(), 0, 0))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.WaitForNBlocks(1))

	// start the relayer process in it's own goroutine
	rlyDone, err := relayer.RunStrategy(src, dst, path.MustGetStrategy())
	require.NoError(t, err)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.WaitForNBlocks(1))
	require.NoError(t, dst.WaitForNBlocks(1))

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(dst, twoTestCoin, dst.MustGetAddress().String(), 0, 0))
	require.NoError(t, dst.SendTransferMsg(src, twoTestCoin, src.MustGetAddress().String(), 0, 0))

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(6))

	// kill relayer routine
	rlyDone()

	// check balance on src against expected
	srcGot, err := src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-4000, srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-4000, dstGot.AmountOf(testDenom).Int64())

	// check balance on src against expected
	srcGot, err = src.QueryBalance(src.Key)
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-4000, srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err = dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-4000, dstGot.AmountOf(testDenom).Int64())
}

func TestGaiaReuseIdentifiers(t *testing.T) {
	chains := spinUpTestChains(t, gaiaChains...)

	var (
		src = chains.MustGet("ibc-0")
		dst = chains.MustGet("ibc-1")
	)

	_, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	// create path
	_, err = src.CreateClients(dst)
	require.NoError(t, err)
	testClientPair(t, src, dst)

	_, err = src.CreateOpenConnections(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testConnectionPair(t, src, dst)

	_, err = src.CreateOpenChannels(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testChannelPair(t, src, dst)

	expectedSrc := src
	expectedDst := dst

	// clear old config
	src.PathEnd.ClientID = ""
	src.PathEnd.ConnectionID = ""
	src.PathEnd.ChannelID = ""
	dst.PathEnd.ClientID = ""
	dst.PathEnd.ConnectionID = ""
	dst.PathEnd.ChannelID = ""

	_, err = src.CreateClients(dst)
	require.NoError(t, err)
	testClientPair(t, src, dst)

	_, err = src.CreateOpenConnections(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testConnectionPair(t, src, dst)

	_, err = src.CreateOpenChannels(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testChannelPair(t, src, dst)

	require.Equal(t, expectedSrc, src)
	require.Equal(t, expectedDst, dst)
}

func TestGaiaMisbehaviourMonitoring(t *testing.T) {
	chains := spinUpTestChains(t, gaiaChains...)

	var (
		src = chains.MustGet("ibc-0")
		dst = chains.MustGet("ibc-1")
	)

	path, err := genTestPathAndSet(src, dst, "transfer", "transfer")
	require.NoError(t, err)

	// create path
	_, err = src.CreateClients(dst)
	require.NoError(t, err)
	testClientPair(t, src, dst)

	_, err = src.CreateOpenConnections(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testConnectionPair(t, src, dst)

	_, err = src.CreateOpenChannels(dst, 3, src.GetTimeout())
	require.NoError(t, err)
	testChannelPair(t, src, dst)

	// start the relayer process in it's own goroutine
	rlyDone, err := relayer.RunStrategy(src, dst, path.MustGetStrategy())
	require.NoError(t, err)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.WaitForNBlocks(1))
	require.NoError(t, dst.WaitForNBlocks(1))

	latestHeight, err := dst.QueryLatestHeight()
	require.NoError(t, err)

	header, err := dst.QueryHeaderAtHeight(latestHeight)
	require.NoError(t, err)

	clientStateRes, err := src.QueryClientState(latestHeight)
	require.NoError(t, err)

	// unpack any into ibc tendermint client state
	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	require.NoError(t, err)

	// cast from interface to concrete type
	clientState, ok := clientStateExported.(*ibctmtypes.ClientState)
	require.True(t, ok, "error when casting exported clientstate")

	height := clientState.GetLatestHeight().(clienttypes.Height)
	heightPlus1 := clienttypes.NewHeight(height.RevisionNumber, height.RevisionHeight+1)

	// setup validator for signing duplicate header
	// use key for dst
	privKey := getSDKPrivKey(1)
	privVal := ibctestingmock.PV{
		PrivKey: privKey,
	}
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	validator := tmtypes.NewValidator(pubKey, header.ValidatorSet.Proposer.VotingPower)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	signers := []tmtypes.PrivValidator{privVal}

	// creating duplicate header
	newHeader := createTMClientHeader(t, dst.ChainID, int64(heightPlus1.RevisionHeight), height,
		header.GetTime().Add(time.Minute), valSet, valSet, signers, header)

	// update client with duplicate header
	updateMsg, err := clienttypes.NewMsgUpdateClient(src.PathEnd.ClientID, newHeader, src.MustGetAddress())
	require.NoError(t, err)

	res, success, err := src.SendMsg(updateMsg)
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, uint32(0), res.Code)

	// wait for packet processing
	require.NoError(t, dst.WaitForNBlocks(6))

	// kill relayer routine
	rlyDone()

	clientStateRes, err = src.QueryClientState(0)
	require.NoError(t, err)

	// unpack any into ibc tendermint client state
	clientStateExported, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	require.NoError(t, err)

	// cast from interface to concrete type
	clientState, ok = clientStateExported.(*ibctmtypes.ClientState)
	require.True(t, ok, "error when casting exported clientstate")

	// clientstate should be frozen
	require.True(t, clientState.IsFrozen())
}

func createTMClientHeader(t *testing.T, chainID string, blockHeight int64, trustedHeight clienttypes.Height,
	timestamp time.Time, tmValSet, tmTrustedVals *tmtypes.ValidatorSet, signers []tmtypes.PrivValidator,
	oldHeader *ibctmtypes.Header) *ibctmtypes.Header {
	var (
		valSet      *tmproto.ValidatorSet
		trustedVals *tmproto.ValidatorSet
	)
	require.NotNil(t, tmValSet)

	vsetHash := tmValSet.Hash()

	tmHeader := tmtypes.Header{
		Version:            tmprotoversion.Consensus{Block: tmversion.BlockProtocol, App: 2},
		ChainID:            chainID,
		Height:             blockHeight,
		Time:               timestamp,
		LastBlockID:        ibctesting.MakeBlockID(make([]byte, tmhash.Size), 10_000, make([]byte, tmhash.Size)),
		LastCommitHash:     oldHeader.Header.LastCommitHash,
		DataHash:           tmhash.Sum([]byte("data_hash")),
		ValidatorsHash:     vsetHash,
		NextValidatorsHash: vsetHash,
		ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
		AppHash:            tmhash.Sum([]byte("app_hash")),
		LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
		EvidenceHash:       tmhash.Sum([]byte("evidence_hash")),
		ProposerAddress:    tmValSet.Proposer.Address, //nolint:staticcheck
	}
	hhash := tmHeader.Hash()
	blockID := ibctesting.MakeBlockID(hhash, 3, tmhash.Sum([]byte("part_set")))
	voteSet := tmtypes.NewVoteSet(chainID, blockHeight, 1, tmproto.PrecommitType, tmValSet)

	commit, err := tmtypes.MakeCommit(blockID, blockHeight, 1, voteSet, signers, timestamp)
	require.NoError(t, err)

	signedHeader := &tmproto.SignedHeader{
		Header: tmHeader.ToProto(),
		Commit: commit.ToProto(),
	}

	if tmValSet != nil {
		valSet, err = tmValSet.ToProto()
		if err != nil {
			panic(err)
		}
	}

	if tmTrustedVals != nil {
		trustedVals, err = tmTrustedVals.ToProto()
		if err != nil {
			panic(err)
		}
	}

	// The trusted fields may be nil. They may be filled before relaying messages to a client.
	// The relayer is responsible for querying client and injecting appropriate trusted fields.
	return &ibctmtypes.Header{
		SignedHeader:      signedHeader,
		ValidatorSet:      valSet,
		TrustedHeight:     trustedHeight,
		TrustedValidators: trustedVals,
	}
}
