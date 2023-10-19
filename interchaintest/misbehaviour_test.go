package interchaintest_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	simappparams "cosmossdk.io/simapp/params"
	"github.com/cometbft/cometbft/crypto/tmhash"
	cometproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cometprotoversion "github.com/cometbft/cometbft/proto/tendermint/version"
	comettypes "github.com/cometbft/cometbft/types"
	cometversion "github.com/cometbft/cometbft/version"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdked25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/gogoproto/proto"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	ibctypes "github.com/cosmos/ibc-go/v7/modules/core/types"
	ibccomettypes "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"
	ibcmocks "github.com/cosmos/ibc-go/v7/testing/mock"
	"github.com/cosmos/ibc-go/v7/testing/simapp"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestRelayerMisbehaviourDetection(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	numVals := 1
	numFullNodes := 0
	logger := zaptest.NewLogger(t)

	cf := interchaintest.NewBuiltinChainFactory(logger, []*interchaintest.ChainSpec{
		{Name: "gaia", Version: "v9.0.0-rc1", NumValidators: &numVals, NumFullNodes: &numFullNodes, ChainConfig: ibc.ChainConfig{ChainID: "chain-a", GasPrices: "0.0uatom", Bech32Prefix: "cosmos"}},
		{Name: "gaia", Version: "v9.0.0-rc1", NumValidators: &numVals, NumFullNodes: &numFullNodes, ChainConfig: ibc.ChainConfig{ChainID: "chain-b", GasPrices: "0.0uatom", Bech32Prefix: "cosmos"}}},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA, chainB := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathChainAChainB = "chainA-chainB"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:           chainA,
			Chain2:           chainB,
			Relayer:          r,
			Path:             pathChainAChainB,
			CreateClientOpts: ibc.CreateClientOptions{TrustingPeriod: "15m"},
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Parallel()

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// create a new user account and wait a few blocks for it to be created on chain
	user := interchaintest.GetAndFundTestUsers(t, ctx, "user-1", 10_000_000, chainA)[0]
	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	// Start the relayer
	require.NoError(t, r.StartRelayer(ctx, eRep, pathChainAChainB))

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// query latest height on chainB
	latestHeight, err := chainB.Height(ctx)
	require.NoError(t, err)

	// query header at height on chainB
	h := int64(latestHeight)
	header, err := queryHeaderAtHeight(ctx, t, h, chainB)
	require.NoError(t, err)

	// query tm client state on chainA
	const clientID = "07-tendermint-0"

	cmd := []string{"ibc", "client", "state", clientID}

	stdout, stderr, err := chainA.Validators[0].ExecQuery(ctx, cmd...)
	require.NoError(t, err)
	require.Empty(t, stderr)

	queryResp := clienttypes.QueryClientStateResponse{}
	err = defaultEncoding().Codec.UnmarshalJSON(stdout, &queryResp)
	require.NoError(t, err)

	clientState, err := clienttypes.UnpackClientState(queryResp.ClientState)
	require.NoError(t, err)

	// get latest height from prev client state above & create new height + 1
	height := clientState.GetLatestHeight().(clienttypes.Height)
	newHeight := clienttypes.NewHeight(height.RevisionNumber, height.RevisionHeight+1)

	// create a validator for signing duplicate header
	keyBz, err := chainB.Validators[0].ReadFile(ctx, "config/priv_validator_key.json")
	require.NoError(t, err)

	pvk := cosmos.PrivValidatorKeyFile{}
	err = json.Unmarshal(keyBz, &pvk)
	require.NoError(t, err)

	decodedKeyBz, err := base64.StdEncoding.DecodeString(pvk.PrivKey.Value)
	require.NoError(t, err)

	privKey := &sdked25519.PrivKey{
		Key: decodedKeyBz,
	}

	privVal := ibcmocks.PV{PrivKey: privKey}
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	val := comettypes.NewValidator(pubKey, header.ValidatorSet.Proposer.VotingPower)
	valSet := comettypes.NewValidatorSet([]*comettypes.Validator{val})
	signers := []comettypes.PrivValidator{privVal}

	// create a duplicate header
	newHeader := createTMClientHeader(
		t,
		chainB.Config().ChainID,
		int64(newHeight.RevisionHeight),
		height,
		header.GetTime().Add(time.Minute),
		valSet,
		valSet,
		signers,
		header,
	)

	// attempt to update client with duplicate header
	b := cosmos.NewBroadcaster(t, chainA)

	m, ok := newHeader.(proto.Message)
	require.True(t, ok)

	protoAny, err := codectypes.NewAnyWithValue(m)
	require.NoError(t, err)

	msg := &clienttypes.MsgUpdateClient{
		ClientId:      clientID,
		ClientMessage: protoAny,
		Signer:        user.FormattedAddress(),
	}
	logger.Info("Misbehaviour test, MsgUpdateClient", zap.String("Signer", user.FormattedAddress()))

	resp, err := cosmos.BroadcastTx(ctx, b, user, msg)
	require.NoError(t, err)
	assertTransactionIsValid(t, resp)

	// wait for inclusion in a block
	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	// query tm client state on chainA to assert it is now frozen
	stdout, stderr, err = chainA.Validators[0].ExecQuery(ctx, cmd...)
	require.NoError(t, err)
	require.Empty(t, stderr)

	newQueryResp := clienttypes.QueryClientStateResponse{}
	err = defaultEncoding().Codec.UnmarshalJSON(stdout, &newQueryResp)
	require.NoError(t, err)

	newClientState, err := clienttypes.UnpackClientState(newQueryResp.ClientState)
	require.NoError(t, err)

	tmClientState, ok := newClientState.(*ibccomettypes.ClientState)
	require.True(t, ok)
	require.NotEqual(t, uint64(0), tmClientState.FrozenHeight.RevisionHeight)
}

func assertTransactionIsValid(t *testing.T, resp sdk.TxResponse) {
	t.Helper()
	require.NotNil(t, resp)
	require.NotEqual(t, 0, resp.GasUsed)
	require.NotEqual(t, 0, resp.GasWanted)
	require.Equal(t, uint32(0), resp.Code)
	require.NotEmpty(t, resp.Data)
	require.NotEmpty(t, resp.TxHash)
	require.NotEmpty(t, resp.Events)
}

func queryHeaderAtHeight(
	ctx context.Context,
	t *testing.T,
	height int64,
	chain *cosmos.CosmosChain,
) (*ibccomettypes.Header, error) {
	t.Helper()
	var (
		page    = 1
		perPage = 100000
	)

	res, err := chain.Validators[0].Client.Commit(ctx, &height)
	require.NoError(t, err)

	val, err := chain.Validators[0].Client.Validators(ctx, &height, &page, &perPage)
	require.NoError(t, err)

	protoVal, err := comettypes.NewValidatorSet(val.Validators).ToProto()
	require.NoError(t, err)

	return &ibccomettypes.Header{
		SignedHeader: res.SignedHeader.ToProto(),
		ValidatorSet: protoVal,
	}, nil
}

func createTMClientHeader(
	t *testing.T,
	chainID string,
	blockHeight int64,
	trustedHeight clienttypes.Height,
	timestamp time.Time,
	tmValSet, tmTrustedVals *comettypes.ValidatorSet,
	signers []comettypes.PrivValidator,
	oldHeader *ibccomettypes.Header,
) exported.ClientMessage {
	t.Helper()
	var (
		valSet      *cometproto.ValidatorSet
		trustedVals *cometproto.ValidatorSet
	)
	require.NotNil(t, tmValSet)

	vsetHash := tmValSet.Hash()

	tmHeader := comettypes.Header{
		Version:            cometprotoversion.Consensus{Block: cometversion.BlockProtocol, App: 2},
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
		ProposerAddress:    tmValSet.Proposer.Address,
	}

	hhash := tmHeader.Hash()
	blockID := ibctesting.MakeBlockID(hhash, 3, tmhash.Sum([]byte("part_set")))
	voteSet := comettypes.NewVoteSet(chainID, blockHeight, 1, cometproto.PrecommitType, tmValSet)

	commit, err := comettypes.MakeCommit(blockID, blockHeight, 1, voteSet, signers, timestamp)
	require.NoError(t, err)

	signedHeader := &cometproto.SignedHeader{
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

	return &ibccomettypes.Header{
		SignedHeader:      signedHeader,
		ValidatorSet:      valSet,
		TrustedHeight:     trustedHeight,
		TrustedValidators: trustedVals,
	}
}

func defaultEncoding() simappparams.EncodingConfig {
	cfg := simappparams.MakeTestEncodingConfig()
	std.RegisterLegacyAminoCodec(cfg.Amino)
	std.RegisterInterfaces(cfg.InterfaceRegistry)
	simapp.ModuleBasics.RegisterLegacyAminoCodec(cfg.Amino)
	simapp.ModuleBasics.RegisterInterfaces(cfg.InterfaceRegistry)

	banktypes.RegisterInterfaces(cfg.InterfaceRegistry)
	ibctypes.RegisterInterfaces(cfg.InterfaceRegistry)
	ibccomettypes.RegisterInterfaces(cfg.InterfaceRegistry)
	transfertypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return cfg
}
