package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibctmtypes "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	ibctesting "github.com/cosmos/ibc-go/v3/testing"
	ibctestingmock "github.com/cosmos/ibc-go/v3/testing/mock"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmprotoversion "github.com/tendermint/tendermint/proto/tendermint/version"
	tmtypes "github.com/tendermint/tendermint/types"
	tmversion "github.com/tendermint/tendermint/version"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultSrcPortID = "transfer"
	DefaultDstPortID = "transfer"
	DefaultOrder     = "unordered"
	DefaultVersion   = "ics20-1"
)

func chainTest(t *testing.T, tcs []testChain) {
	chains := spinUpTestChains(t, tcs...)

	var (
		src         = chains.MustGet(tcs[0].chainID)
		dst         = chains.MustGet(tcs[1].chainID)
		testDenom   = "samoleans"
		testCoin    = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		twoTestCoin = sdk.NewCoin(testDenom, sdk.NewInt(2000))
	)

	_, err := genTestPathAndSet(src, dst)
	require.NoError(t, err)

	// query initial balances to compare against at the end
	var srcExpected, dstExpected sdk.Coins

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			srcExpected, err = src.ChainProvider.QueryBalance(egCtx, src.ChainProvider.Key())
			if srcExpected.IsZero() {
				return fmt.Errorf("expected non-zero balance. Err: %w", err)
			}
			return err
		})
	})
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			dstExpected, err = dst.ChainProvider.QueryBalance(egCtx, dst.ChainProvider.Key())
			if dstExpected.IsZero() {
				return fmt.Errorf("expected non-zero balance. Err: %w", err)
			}
			return err
		})
	})
	require.NoError(t, eg.Wait())

	// create path
	_, err = src.CreateClients(ctx, dst, true, true, false)
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout)
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	_, err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false)
	require.NoError(t, err)

	// query open channels and ensure there is no error
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	// send a couple of transfers to the queue on src
	dstAddr, err := dst.ChainProvider.Address()
	require.NoError(t, err)

	srcAddr, err := src.ChainProvider.Address()
	require.NoError(t, err)

	require.NoError(t, src.SendTransferMsg(ctx, dst, testCoin, dstAddr, 0, 0, channel))
	require.NoError(t, src.SendTransferMsg(ctx, dst, testCoin, dstAddr, 0, 0, channel))

	// send a couple of transfers to the queue on dst
	require.NoError(t, dst.SendTransferMsg(ctx, src, testCoin, srcAddr, 0, 0, channel))
	require.NoError(t, dst.SendTransferMsg(ctx, src, testCoin, srcAddr, 0, 0, channel))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	// start the relayer process in it's own goroutine
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, src, dst, filter, 2*cmd.MB, 5)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 2))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 2))

	// send those tokens from dst back to dst and src back to src
	require.NoError(t, src.SendTransferMsg(ctx, dst, twoTestCoin, dstAddr, 0, 0, channel))
	require.NoError(t, dst.SendTransferMsg(ctx, src, twoTestCoin, srcAddr, 0, 0, channel))

	// wait for packet processing
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 6))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 6))

	// check balance on src against expected
	srcGot, err := src.ChainProvider.QueryBalance(ctx, src.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-4000, srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.ChainProvider.QueryBalance(ctx, dst.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-4000, dstGot.AmountOf(testDenom).Int64())
}

func TestGaiaReuseIdentifiers(t *testing.T) {
	// TODO: fix and re-enable this test
	t.Skip()
	chains := spinUpTestChains(t, []testChain{
		{"ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}...)

	var (
		src = chains.MustGet("ibc-0")
		dst = chains.MustGet("ibc-1")
	)

	_, err := genTestPathAndSet(src, dst)
	require.NoError(t, err)

	// create path
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = src.CreateClients(ctx, dst, true, true, false)
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout)
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	_, err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false)
	require.NoError(t, err)

	// query open channels and ensure there is no error
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	expectedSrc := src
	expectedDst := dst

	// clear old config
	src.PathEnd.ClientID = ""
	src.PathEnd.ConnectionID = ""
	dst.PathEnd.ClientID = ""
	dst.PathEnd.ConnectionID = ""

	_, err = src.CreateClients(ctx, dst, true, true, false)
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout)
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	_, err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false)
	require.NoError(t, err)
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	require.Equal(t, expectedSrc, src)
	require.Equal(t, expectedDst, dst)

	expectedSrcClient := src.PathEnd.ClientID
	expectedDstClient := dst.PathEnd.ClientID

	// test client creation with override
	src.PathEnd.ClientID = ""
	dst.PathEnd.ClientID = ""

	_, err = src.CreateClients(ctx, dst, true, true, true)
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	require.NotEqual(t, expectedSrcClient, src.PathEnd.ClientID)
	require.NotEqual(t, expectedDstClient, dst.PathEnd.ClientID)
}

func TestGaiaMisbehaviourMonitoring(t *testing.T) {
	// TODO: fix and re-enable this test
	// need to figure out what this feature is supposed to do
	t.Skip()
	chains := spinUpTestChains(t, []testChain{
		{"ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}...)

	var (
		src = chains.MustGet("ibc-0")
		dst = chains.MustGet("ibc-1")
	)

	_, err := genTestPathAndSet(src, dst)
	require.NoError(t, err)

	// create path
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = src.CreateClients(ctx, dst, true, true, false)
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout)
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	_, err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false)
	require.NoError(t, err)

	// query open channels and ensure there is no error
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	// start the relayer process in it's own goroutine
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, src, dst, filter, 2*cmd.MB, 5)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 1))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	latestHeight, err := dst.ChainProvider.QueryLatestHeight(ctx)
	require.NoError(t, err)

	header, err := dst.ChainProvider.QueryHeaderAtHeight(ctx, latestHeight)
	require.NoError(t, err)

	clientState, err := src.QueryTMClientState(ctx, latestHeight)
	require.NoError(t, err)

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

	tmHeader, ok := header.(*ibctmtypes.Header)
	if !ok {
		t.Fatalf("got data of type %T but wanted tmclient.Header \n", header)
	}
	validator := tmtypes.NewValidator(pubKey, tmHeader.ValidatorSet.Proposer.VotingPower)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	signers := []tmtypes.PrivValidator{privVal}

	// creating duplicate header
	newHeader := createTMClientHeader(t, dst.ChainID(), int64(heightPlus1.RevisionHeight), height,
		tmHeader.GetTime().Add(time.Minute), valSet, valSet, signers, tmHeader)

	// update client with duplicate header
	updateMsg, err := src.ChainProvider.UpdateClient(src.PathEnd.ClientID, newHeader)
	require.NoError(t, err)

	res, success, err := src.ChainProvider.SendMessage(ctx, updateMsg)
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, uint32(0), res.Code)

	// wait for packet processing
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 6))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 6))

	clientState, err = src.QueryTMClientState(ctx, 0)
	require.NoError(t, err)

	// clientstate should be frozen i.e., clientstate frozenheight should not be zero
	require.False(t, clientState.FrozenHeight.IsZero())
}

func TestRelayAllChannelsOnConnection(t *testing.T) {
	chains := spinUpTestChains(t, []testChain{
		{"ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}...)

	var (
		src         = chains.MustGet("ibc-0")
		dst         = chains.MustGet("ibc-1")
		testDenom   = "samoleans"
		testCoin    = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		twoTestCoin = sdk.NewCoin(testDenom, sdk.NewInt(2000))
	)

	_, err := genTestPathAndSet(src, dst)
	require.NoError(t, err)

	// query initial balances to compare against at the end
	var srcExpected, dstExpected sdk.Coins

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			srcExpected, err = src.ChainProvider.QueryBalance(egCtx, src.ChainProvider.Key())
			if err != nil {
				return err
			}

			if srcExpected.IsZero() {
				return fmt.Errorf("expected non-zero balance. Err: %w", err)
			}

			return nil
		})
	})
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			dstExpected, err = dst.ChainProvider.QueryBalance(egCtx, dst.ChainProvider.Key())
			if err != nil {
				return err
			}

			if dstExpected.IsZero() {
				return fmt.Errorf("expected non-zero balance. Err: %w", err)
			}

			return nil
		})
	})
	require.NoError(t, eg.Wait())

	// create path
	_, err = src.CreateClients(ctx, dst, true, true, false)
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout)
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	_, err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false)
	require.NoError(t, err)

	_, err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, true)
	require.NoError(t, err)

	// query open channels and ensure there are two
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)
	require.Equal(t, 2, len(channels))

	channelOne := channels[0]
	channelTwo := channels[1]

	// send a couple of transfers to the queue on src for first channel
	dstAddr, err := dst.ChainProvider.Address()
	require.NoError(t, err)

	srcAddr, err := src.ChainProvider.Address()
	require.NoError(t, err)

	require.NoError(t, src.SendTransferMsg(ctx, dst, testCoin, dstAddr, 0, 0, channelOne))
	require.NoError(t, src.SendTransferMsg(ctx, dst, testCoin, dstAddr, 0, 0, channelOne))

	// send a couple of transfers to the queue on dst for first channel
	require.NoError(t, dst.SendTransferMsg(ctx, src, testCoin, srcAddr, 0, 0, channelOne))
	require.NoError(t, dst.SendTransferMsg(ctx, src, testCoin, srcAddr, 0, 0, channelOne))

	// send a couple of transfers to the queue on src for second channel
	require.NoError(t, src.SendTransferMsg(ctx, dst, testCoin, dstAddr, 0, 0, channelTwo))
	require.NoError(t, src.SendTransferMsg(ctx, dst, testCoin, dstAddr, 0, 0, channelTwo))

	// send a couple of transfers to the queue on dst for second channel
	require.NoError(t, dst.SendTransferMsg(ctx, src, testCoin, srcAddr, 0, 0, channelTwo))
	require.NoError(t, dst.SendTransferMsg(ctx, src, testCoin, srcAddr, 0, 0, channelTwo))

	// Wait for message inclusion in both chains
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	// start the relayer process in it's own goroutine
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, src, dst, filter, 2*cmd.MB, 5)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 1))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	// send those tokens from dst back to dst and src back to src for first channel
	require.NoError(t, src.SendTransferMsg(ctx, dst, twoTestCoin, dstAddr, 0, 0, channelOne))
	require.NoError(t, dst.SendTransferMsg(ctx, src, twoTestCoin, srcAddr, 0, 0, channelOne))

	// send those tokens from dst back to dst and src back to src for second channel
	require.NoError(t, src.SendTransferMsg(ctx, dst, twoTestCoin, dstAddr, 0, 0, channelTwo))
	require.NoError(t, dst.SendTransferMsg(ctx, src, twoTestCoin, srcAddr, 0, 0, channelTwo))

	// wait for packet processing
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 6))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 6))

	// check balance on src against expected
	srcGot, err := src.ChainProvider.QueryBalance(ctx, src.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-8000, srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.ChainProvider.QueryBalance(ctx, dst.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-8000, dstGot.AmountOf(testDenom).Int64())
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
