package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	tmclient "github.com/cosmos/ibc-go/v4/modules/light-clients/07-tendermint/types"
	ibctesting "github.com/cosmos/ibc-go/v4/testing"
	ibctestingmock "github.com/cosmos/ibc-go/v4/testing/mock"
	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmprotoversion "github.com/tendermint/tendermint/proto/tendermint/version"
	tmtypes "github.com/tendermint/tendermint/types"
	tmversion "github.com/tendermint/tendermint/version"
	"go.uber.org/zap/zaptest"
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

	t.Log("Querying initial balances for later comparison")
	var srcExpected, dstExpected sdk.Coins

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			srcExpected, err = src.ChainProvider.QueryBalance(egCtx, src.ChainProvider.Key())
			if srcExpected.IsZero() {
				return fmt.Errorf("(src chain) expected non-zero balance. Err: %w", err)
			}
			return err
		})
	})
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			dstExpected, err = dst.ChainProvider.QueryBalance(egCtx, dst.ChainProvider.Key())
			if dstExpected.IsZero() {
				return fmt.Errorf("(dst chain) expected non-zero balance. Err: %w", err)
			}
			return err
		})
	})
	require.NoError(t, eg.Wait())

	t.Log("Creating clients")
	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	t.Log("Creating connections")
	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	t.Log("Creating channels")
	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
	require.NoError(t, err)

	t.Log("Querying open channels to ensure successful creation")
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	dstAddr, err := dst.ChainProvider.Address()
	require.NoError(t, err)

	srcAddr, err := src.ChainProvider.Address()
	require.NoError(t, err)

	log := zaptest.NewLogger(t)

	t.Log("Sending initial transfers to source chain")
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, testCoin, dstAddr, 0, 0, channel))
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, testCoin, dstAddr, 0, 0, channel))

	t.Log("Sending initial transfers to dest chain")
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, testCoin, srcAddr, 0, 0, channel))
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, testCoin, srcAddr, 0, 0, channel))

	t.Log("Waiting for transfers to reach blocks")
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	t.Log("Starting relayer")
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, log, src, dst, filter, 2*cmd.MB, 5, "", relayer.ProcessorEvents, 20, "", nil)

	t.Log("Waiting for relayer messages to reach both chains")
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 2))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 2))

	t.Log("Initiating transfer of tokens back where they came from")
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, twoTestCoin, dstAddr, 0, 0, channel))
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, twoTestCoin, srcAddr, 0, 0, channel))

	t.Log("Waiting for transfers to be processed")
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 6))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 6))

	t.Log("Checking expected source balance")
	srcGot, err := src.ChainProvider.QueryBalance(ctx, src.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-4000, srcGot.AmountOf(testDenom).Int64())

	t.Log("Checking expected dest balance")
	dstGot, err := dst.ChainProvider.QueryBalance(ctx, dst.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-4000, dstGot.AmountOf(testDenom).Int64())
}

func TestGaiaReuseIdentifiers(t *testing.T) {
	// TODO: fix and re-enable this test
	t.Skip()
	chains := spinUpTestChains(t, []testChain{
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
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

	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
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

	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
	require.NoError(t, err)
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	require.Equal(t, expectedSrc, src)
	require.Equal(t, expectedDst, dst)

	expectedSrcClient := src.PathEnd.ClientID
	expectedDstClient := dst.PathEnd.ClientID

	// test client creation with override
	src.PathEnd.ClientID = ""
	dst.PathEnd.ClientID = ""

	_, err = src.CreateClients(ctx, dst, true, true, true, 0, "")
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
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
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

	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
	require.NoError(t, err)

	// query open channels and ensure there is no error
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	log := zaptest.NewLogger(t)
	// start the relayer process in it's own goroutine
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, log, src, dst, filter, 2*cmd.MB, 5, "", relayer.ProcessorEvents, 20, "", nil)

	// Wait for relay message inclusion in both chains
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 1))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	latestHeight, err := dst.ChainProvider.QueryLatestHeight(ctx)
	require.NoError(t, err)

	header, err := dst.ChainProvider.QueryIBCHeader(ctx, latestHeight)
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

	tmHeader, ok := header.(cosmos.CosmosIBCHeader)
	if !ok {
		t.Fatalf("got data of type %T but wanted cosmos.CosmosIBCHeader \n", header)
	}
	validator := tmtypes.NewValidator(pubKey, tmHeader.ValidatorSet.Proposer.VotingPower)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	signers := []tmtypes.PrivValidator{privVal}

	// creating duplicate header
	newHeader := createTMClientHeader(t, dst.ChainID(), int64(heightPlus1.RevisionHeight), height,
		tmHeader.SignedHeader.Time.Add(time.Minute), valSet, valSet, signers, nil)

	// update client with duplicate header
	updateMsg, err := src.ChainProvider.MsgUpdateClient(src.PathEnd.ClientID, newHeader)
	require.NoError(t, err)

	res, success, err := src.ChainProvider.SendMessage(ctx, updateMsg, "")
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
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
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

	t.Log("Querying initial balances to compare against at the end")
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

	t.Log("Creating clients")
	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	t.Log("Creating connections")
	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	t.Log("Creating channels")
	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
	require.NoError(t, err)

	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, true, "", "")
	require.NoError(t, err)

	t.Log("Ensuring two channels exist")
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

	log := zaptest.NewLogger(t)

	t.Log("Sending transfers from src to dst on first channel")
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, testCoin, dstAddr, 0, 0, channelOne))
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, testCoin, dstAddr, 0, 0, channelOne))

	t.Log("Sending transfers from dst to src on first channel")
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, testCoin, srcAddr, 0, 0, channelOne))
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, testCoin, srcAddr, 0, 0, channelOne))

	t.Log("Sending transfers from src to dst on second channel")
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, testCoin, dstAddr, 0, 0, channelTwo))
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, testCoin, dstAddr, 0, 0, channelTwo))

	t.Log("Sending transfers from dst to src on second channel")
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, testCoin, srcAddr, 0, 0, channelTwo))
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, testCoin, srcAddr, 0, 0, channelTwo))

	t.Log("Waiting for message inclusion in both chains")
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 1))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	t.Log("Starting relayer")
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, log, src, dst, filter, 2*cmd.MB, 5, "", relayer.ProcessorEvents, 20, "", nil)

	t.Log("Waiting for relayer message inclusion in both chains")
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 1))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 1))

	t.Log("Sending tokens back on first channel")
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, twoTestCoin, dstAddr, 0, 0, channelOne))
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, twoTestCoin, srcAddr, 0, 0, channelOne))

	t.Log("Sending tokens back on second channel")
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, twoTestCoin, dstAddr, 0, 0, channelTwo))
	require.NoError(t, dst.SendTransferMsg(ctx, log, src, twoTestCoin, srcAddr, 0, 0, channelTwo))

	t.Log("Waiting for packet processing")
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 6))
	require.NoError(t, dst.ChainProvider.WaitForNBlocks(ctx, 6))

	t.Log("Checking src balance")
	srcGot, err := src.ChainProvider.QueryBalance(ctx, src.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64()-8000, srcGot.AmountOf(testDenom).Int64())

	t.Log("Checking dst balance")
	dstGot, err := dst.ChainProvider.QueryBalance(ctx, dst.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64()-8000, dstGot.AmountOf(testDenom).Int64())
}

func createTMClientHeader(t *testing.T, chainID string, blockHeight int64, trustedHeight clienttypes.Height,
	timestamp time.Time, tmValSet, tmTrustedVals *tmtypes.ValidatorSet, signers []tmtypes.PrivValidator,
	oldHeader *tmclient.Header) *tmclient.Header {
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
	return &tmclient.Header{
		SignedHeader:      signedHeader,
		ValidatorSet:      valSet,
		TrustedHeight:     trustedHeight,
		TrustedValidators: trustedVals,
	}
}

// a.Config.Chains[args[0]]
func TestUnorderedChannelBlockHeightTimeout(t *testing.T) {
	chains := spinUpTestChains(t, []testChain{
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}...)

	var (
		src         = chains.MustGet("ibc-0")
		dst         = chains.MustGet("ibc-1")
		testDenom   = "samoleans"
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
	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
	require.NoError(t, err)

	// query open channels and ensure there is no error
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	dstAddr, err := dst.ChainProvider.Address()
	require.NoError(t, err)

	_, err = src.ChainProvider.Address()
	require.NoError(t, err)

	log := zaptest.NewLogger(t)

	// send a packet that should timeout after 10 blocks have passed
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, twoTestCoin, dstAddr, uint64(10), 0, channel))

	// wait for block height timeout offset to be reached
	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 11))

	// start the relayer process in it's own goroutine
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, log, src, dst, filter, 2*cmd.MB, 5, "", relayer.ProcessorEvents, 20, "", nil)

	require.NoError(t, src.ChainProvider.WaitForNBlocks(ctx, 5))

	// check balance on src against expected
	srcGot, err := src.ChainProvider.QueryBalance(ctx, src.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64(), srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.ChainProvider.QueryBalance(ctx, dst.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64(), dstGot.AmountOf(testDenom).Int64())
}

func TestUnorderedChannelTimestampTimeout(t *testing.T) {
	chains := spinUpTestChains(t, []testChain{
		{"testChain0", "ibc-0", 0, gaiaTestConfig, gaiaProviderCfg},
		{"testChain1", "ibc-1", 1, gaiaTestConfig, gaiaProviderCfg},
	}...)

	var (
		src         = chains.MustGet("ibc-0")
		dst         = chains.MustGet("ibc-1")
		testDenom   = "samoleans"
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
	_, err = src.CreateClients(ctx, dst, true, true, false, 0, "")
	require.NoError(t, err)
	testClientPair(ctx, t, src, dst)

	timeout, err := src.GetTimeout()
	require.NoError(t, err)

	_, err = src.CreateOpenConnections(ctx, dst, 3, timeout, "", 0, "")
	require.NoError(t, err)
	testConnectionPair(ctx, t, src, dst)

	err = src.CreateOpenChannels(ctx, dst, 3, timeout, DefaultSrcPortID, DefaultDstPortID, DefaultOrder, DefaultVersion, false, "", "")
	require.NoError(t, err)

	// query open channels and ensure there is no error
	channels, err := src.ChainProvider.QueryConnectionChannels(ctx, 0, src.ConnectionID())
	require.NoError(t, err)

	channel := channels[0]
	testChannelPair(ctx, t, src, dst, channel.ChannelId, channel.PortId)

	dstAddr, err := dst.ChainProvider.Address()
	require.NoError(t, err)

	_, err = src.ChainProvider.Address()
	require.NoError(t, err)

	log := zaptest.NewLogger(t)

	// send a packet that should timeout after 45 seconds
	require.NoError(t, src.SendTransferMsg(ctx, log, dst, twoTestCoin, dstAddr, 0, time.Second*15, channel))

	// wait for timestamp timeout offset to be reached
	time.Sleep(time.Second * 20)

	// start the relayer process in it's own goroutine
	filter := relayer.ChannelFilter{}
	_ = relayer.StartRelayer(ctx, log, src, dst, filter, 2*cmd.MB, 5, "", relayer.ProcessorEvents, 20, "", nil)

	time.Sleep(time.Second * 10)

	// check balance on src against expected
	srcGot, err := src.ChainProvider.QueryBalance(ctx, src.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, srcExpected.AmountOf(testDenom).Int64(), srcGot.AmountOf(testDenom).Int64())

	// check balance on dst against expected
	dstGot, err := dst.ChainProvider.QueryBalance(ctx, dst.ChainProvider.Key())
	require.NoError(t, err)
	require.Equal(t, dstExpected.AmountOf(testDenom).Int64(), dstGot.AmountOf(testDenom).Int64())
}
