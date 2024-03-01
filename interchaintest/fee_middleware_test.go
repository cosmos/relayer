package interchaintest_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRelayerFeeMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	nv := 1
	nf := 0

	// Get both chains
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "juno",
			ChainName:     "chaina",
			Version:       "v13.0.0",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig:   ibc.ChainConfig{ChainID: "chaina", GasPrices: "0.0ujuno"},
		},
		{
			Name:          "juno",
			ChainName:     "chainb",
			Version:       "v13.0.0",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig:   ibc.ChainConfig{ChainID: "chainb", GasPrices: "0.0ujuno"},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA, chainB := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathChainAChainB = "chainA-chainB"

	// Build the network
	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    pathChainAChainB,
			CreateChannelOpts: ibc.CreateChannelOptions{
				SourcePortName: "transfer",
				DestPortName:   "transfer",
				Order:          ibc.Unordered,
				Version:        "{\"fee_version\":\"ics29-1\",\"app_version\":\"ics20-1\"}",
			},
			CreateClientOpts: ibc.DefaultClientOpts(),
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

	err = testutil.WaitForBlocks(ctx, 5, chainA, chainB)
	require.NoError(t, err)

	// ChainID of ChainA
	chainIDA := chainA.Config().ChainID

	// Channel of ChainA
	chA, err := r.GetChannels(ctx, eRep, chainIDA)
	require.NoError(t, err)
	channelA := chA[0]

	// Fund a user account on chain1 and chain2
	initBal := sdkmath.NewInt(1_000_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), initBal, chainA, chainB)
	userA := users[0]
	userAddressA := userA.FormattedAddress()
	userB := users[1]
	userAddressB := userB.FormattedAddress()

	// Addresses of both the chains
	walletA, _ := r.GetWallet(chainA.Config().ChainID)
	rlyAddressA := walletA.FormattedAddress()

	walletB, _ := r.GetWallet(chainB.Config().ChainID)
	rlyAddressB := walletB.FormattedAddress()

	// register CounterpartyPayee
	cmd := []string{
		"tx", "register-counterparty",
		chainA.Config().Name,
		channelA.ChannelID,
		"transfer",
		rlyAddressA,
		rlyAddressB,
	}
	_ = r.Exec(ctx, eRep, cmd, nil)
	require.NoError(t, err)

	// Query the relayer CounterpartyPayee on a given channel
	query := []string{
		chainA.Config().Bin, "query", "ibc-fee", "counterparty-payee", channelA.ChannelID, rlyAddressA,
		"--chain-id", chainIDA,
		"--node", chainA.GetRPCAddress(),
		"--home", chainA.HomeDir(),
		"--trace",
	}
	_, _, err = chainA.Exec(ctx, query, nil)
	require.NoError(t, err)

	// Get initial account balances
	userAOrigBal, err := chainA.GetBalance(ctx, userAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userAOrigBal))

	userBOrigBal, err := chainB.GetBalance(ctx, userAddressB, chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userBOrigBal))

	rlyAOrigBal, err := chainA.GetBalance(ctx, rlyAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(rlyAOrigBal))

	rlyBOrigBal, err := chainB.GetBalance(ctx, rlyAddressB, chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(rlyBOrigBal))

	// send tx
	txAmount := sdkmath.NewInt(1000)
	transfer := ibc.WalletAmount{Address: userAddressB, Denom: chainA.Config().Denom, Amount: txAmount}
	_, err = chainA.SendIBCTransfer(ctx, channelA.ChannelID, userAddressA, transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	// Incentivizing async packet by returning MsgPayPacketFeeAsync
	packetFeeAsync := []string{
		chainA.Config().Bin, "tx", "ibc-fee", "pay-packet-fee", "transfer", channelA.ChannelID, "1",
		"--recv-fee", fmt.Sprintf("1000%s", chainA.Config().Denom),
		"--ack-fee", fmt.Sprintf("1000%s", chainA.Config().Denom),
		"--timeout-fee", fmt.Sprintf("1000%s", chainA.Config().Denom),
		"--chain-id", chainIDA,
		"--node", chainA.GetRPCAddress(),
		"--from", userA.FormattedAddress(),
		"--keyring-backend", "test",
		"--gas", "400000",
		"--yes",
		"--home", chainA.HomeDir(),
	}
	_, _, err = chainA.Exec(ctx, packetFeeAsync, nil)
	require.NoError(t, err)

	// start the relayer
	err = r.StartRelayer(ctx, eRep, pathChainAChainB)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occurred while stopping the relayer: %s", err)
			}
		},
	)

	// Wait for relayer to run
	err = testutil.WaitForBlocks(ctx, 10, chainA, chainB)
	require.NoError(t, err)

	// Assigning denom
	chainATokenDenom := transfertypes.GetPrefixedDenom(channelA.PortID, channelA.ChannelID, chainA.Config().Denom)
	chainADenomTrace := transfertypes.ParseDenomTrace(chainATokenDenom)

	// Get balances after the fees
	expectedBal := userAOrigBal.Sub(txAmount.AddRaw(1000))
	chainABal, err := chainA.GetBalance(ctx, userAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, expectedBal.Equal(chainABal))

	chainBBal, err := chainB.GetBalance(ctx, userAddressB, chainADenomTrace.IBCDenom())
	require.NoError(t, err)
	require.True(t, txAmount.Equal(chainBBal))

	rlyABal, err := chainA.GetBalance(ctx, rlyAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, rlyAOrigBal.AddRaw(1000).Equal(rlyABal))
}
