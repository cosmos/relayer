package ibctest_test

import (
	"context"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	relayeribctest "github.com/cosmos/relayer/v2/ibctest"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRelayMany_WIP(t *testing.T) {
	t.Parallel()

	cli, network := ibctest.DockerSetup(t)

	// Set up specs for 4 chains.
	// All gaia now for simplicity but could be any valid chain.
	gSpec := ibctest.ChainSpec{
		Name: "gaia", Version: "v7.0.1",
	}
	chainAFE := gSpec
	chainBFE := gSpec
	chainCFE := gSpec
	chainDFE := gSpec

	chainAFE.ChainID = "chain-a"
	chainBFE.ChainID = "chain-b"
	chainCFE.ChainID = "chain-c"
	chainDFE.ChainID = "chain-d"

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		&chainAFE, &chainBFE, &chainCFE, &chainDFE,
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chainA, chainB, chainC, chainD := chains[0], chains[1], chains[2], chains[3]
	r := (relayeribctest.RelayerFactory{}).Build(t, nil, network).(*relayeribctest.Relayer)

	ic := ibctest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddChain(chainC).
		AddChain(chainD).
		AddRelayer(r, "r").
		AddLink(ibctest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,

			Path: "a-b",
		}).
		AddLink(ibctest.InterchainLink{
			Chain1:  chainB,
			Chain2:  chainC,
			Relayer: r,

			Path: "b-c",
		}).
		AddLink(ibctest.InterchainLink{
			Chain1:  chainC,
			Chain2:  chainD,
			Relayer: r,

			Path: "c-d",
		})

	ctx := context.Background()
	eRep := testreporter.NewReporter(newNopWriteCloser()).RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		NetworkID: network,
		Client:    cli,
	}))

	aChannels, err := r.GetChannels(ctx, eRep, chainA.Config().ChainID)
	require.NoError(t, err)
	aChannelID := aChannels[0].ChannelID

	r.StartRelayerMany(
		ctx,
		nil,
		"a-b",

		// If you enable the path b-c, the test ends up failing to find the acknowledgement,
		// and this error is logged (json formatted a little for readability):
		// logger.go:130: 2022-06-03T09:22:21.089-0400	INFO	Error building or broadcasting transaction
		// {
		//   "provider_type": "cosmos",
		//   "chain_id": "chain-b",
		//   "attempt": 1,
		//   "max_attempts": 5,
		//   "error": "rpc error: code = InvalidArgument desc = failed to execute message; message index: 0: cannot update client with ID 07-tendermint-1: trusted validators validators:<address:\"\\220\\327 \\315\\243\\257\\303|\\236\\260\\326\\350-0o\\264\\316\\177!-\" pub_key:<ed25519:\"z\\277h\\032\\377\\272\\325\\351\\3264\\312\\253[\\304\\312\\343e$R\\337GSL\\265q\\301\\014\\271\\377\\0313>\" > voting_power:100000 proposer_priority:-100000 > validators:<address:\"\\346\\005v\\252maz\\260\\326i\\035\\307\\036t\\353\\314\\\\\\322\\215'\" pub_key:<ed25519:\"\\002\\237\\237)k\\325>\\0051M\\270kD#>\\0076\\202\\\\\\230\\003]\\263\\365\\247/2\\240\\301\\260\\014V\" > voting_power:100000 proposer_priority:100000 > proposer:<address:\"\\220\\327 \\315\\243\\257\\303|\\236\\260\\326\\350-0o\\264\\316\\177!-\" pub_key:<ed25519:\"z\\277h\\032\\377\\272\\325\\351\\3264\\312\\253[\\304\\312\\343e$R\\337GSL\\265q\\301\\014\\271\\377\\0313>\" > voting_power:100000 proposer_priority:-100000 > , does not hash to latest trusted validators. Expected: ADE9985348F0D65D7B7745F265F18698DAB0FB2BA86C3FF766D03C4A6F997DCB, got: 1278074F555A801819BC6F5F65E9EACEBAC41A5815BE217605288120FAA508F6: invalid validator set: invalid request"
		// }

		// "b-c",
		"c-d",
	)
	defer r.StopRelayer(ctx, eRep)

	// A sends to B, so get B's faucet address.
	bFaucetAddrBytes, err := chainB.GetAddress(ctx, ibctest.FaucetAccountKeyName)
	require.NoError(t, err)
	bFaucetAddr, err := types.Bech32ifyAddressBytes(chainB.Config().Bech32Prefix, bFaucetAddrBytes)
	require.NoError(t, err)

	beforeATxHeight, err := chainA.Height(ctx)
	require.NoError(t, err)

	abIBCDenom := transfertypes.ParseDenomTrace(
		transfertypes.GetPrefixedDenom(aChannels[0].Counterparty.PortID, aChannels[0].Counterparty.ChannelID, chainA.Config().Denom),
	).IBCDenom()

	const abTxAmount = 13579 // Arbitrary amount that is easy to find in logs.
	abTx, err := chainA.SendIBCTransfer(ctx, aChannelID, ibctest.FaucetAccountKeyName, ibc.WalletAmount{
		Address: bFaucetAddr,
		Denom:   chainA.Config().Denom,
		Amount:  abTxAmount,
	}, nil)
	require.NoError(t, err)
	require.NoError(t, abTx.Validate())

	afterATxHeight, err := chainA.Height(ctx)
	require.NoError(t, err)

	abAck, err := test.PollForAck(ctx, chainA, beforeATxHeight, afterATxHeight+10, abTx.Packet)
	require.NoError(t, err, "failed to get acknowledgement on chain A")
	require.NoError(t, abAck.Validate(), "invalid acknowledgement on chain A")

	afterBBalance, err := chainB.GetBalance(ctx, bFaucetAddr, abIBCDenom)
	require.NoError(t, err)
	require.Equal(t, afterBBalance, int64(abTxAmount))
}
