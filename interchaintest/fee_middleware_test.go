package interchaintest_test

import (
	"context"
	"fmt"
	"testing"

	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	ibc "github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestFeeMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "juno", Version: "v13.0.0", ChainConfig: ibc.ChainConfig{ChainID: "chain--aa", GasPrices: "0.0ujuno"}},
		{Name: "juno", Version: "v13.0.0", ChainConfig: ibc.ChainConfig{ChainID: "chain--bb", GasPrices: "0.0ujuno"}}},
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

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// height, err := chainB.Height(ctx)
	// require.NoError(t, err)
	// _, err = cosmos.PollForMessage[chantypes.MsgChannelOpenConfirm](ctx, chainB, chainB.Config().EncodingConfig.InterfaceRegistry, 1, height+10, nil)
	// require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 10, chainA, chainB)
	require.NoError(t, err)

	// ChainID of both chains
	chainID_A := chainA.Config().ChainID
	// chainID_B := chainB.Config().ChainID

	// Query for Channel of both the chains
	// channel, err := ibc.GetTransferChannel(ctx, r, eRep, chainID_A, chainID_B)
	// require.NoError(t, err)
	channelA, err := r.GetChannels(ctx, eRep, chainID_A)
	require.NoError(t, err)
	channel := channelA[0]

	//// Fund a user account on chain1 and chain2
	const userFunds = int64(1_000_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chainA, chainB)
	walletA := users[0]
	userAddressA := walletA.FormattedAddress()
	walletB := users[1]
	userAddressB := walletB.FormattedAddress()

	// Query for the Addresses of both the chains
	WalletA, _ := r.GetWallet(chainA.Config().ChainID)
	rlyAddressA := WalletA.FormattedAddress()
	// addressA := string(walletA.Address())

	WalletB, _ := r.GetWallet(chainB.Config().ChainID)
	rlyAddressB := WalletB.FormattedAddress()

	// addressB := string(walletB.Address())

	// Query for the newly registered CounterpartyPayee
	cmd := []string{
		"tx", "reg-cpt",
		"chain_name", chainA.Config().Name,
		"channel_id", channel.ChannelID,
		"port_id", "transfer",
		"relay_addr", rlyAddressA,
		"counterparty_payee", rlyAddressB,
	}

	// env := []string{addressA, addressB}
	_ = r.Exec(ctx, eRep, cmd, nil)
	require.NoError(t, err)

	// Wait for relayer to start up and finish channel handshake
	err = testutil.WaitForBlocks(ctx, 15, chainA, chainB)
	require.NoError(t, err)

	// Get initial account balances
	userAOrigBal, err := chainA.GetBalance(ctx, userAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userFunds, userAOrigBal)

	userBOrigBal, err := chainB.GetBalance(ctx, userAddressB, chainB.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userFunds, userBOrigBal)

	rlyAOrigBal, err := chainA.GetBalance(ctx, rlyAddressA, chainB.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userFunds, rlyAOrigBal)

	rlyBOrigBal, err := chainB.GetBalance(ctx, rlyAddressB, chainB.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userFunds, rlyBOrigBal)

	// send tx
	const txAmount = 1000
	transfer := ibc.WalletAmount{Address: userAddressB, Denom: chainA.Config().Denom, Amount: txAmount}
	_, err = chainA.SendIBCTransfer(ctx, channel.ChannelID, userAddressA, transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	// Incentivizing async packet by returning MsgPayPacketFeeAsync
	packetFeeAsync := []string{
		chainA.Config().Bin, "tx", "ibc-fee", "pay-packet-fee", "transfer", channel.ChannelID, "1",
		"--recv-fee", fmt.Sprintf("1000%s", chainA.Config().Denom),
		"--ack-fee", fmt.Sprintf("1000%s", chainA.Config().Denom),
		"--timeout-fee", fmt.Sprintf("1000%s", chainA.Config().Denom),
	}
	_, _, err = chainA.Validators[0].Exec(ctx, packetFeeAsync, nil)
	require.NoError(t, err)

	// Start the relayer and set the cleanup function.
	err = r.StartRelayer(ctx, eRep, pathChainAChainB)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Wait for relayer to run
	err = testutil.WaitForBlocks(ctx, 5, chainA, chainB)
	require.NoError(t, err)

	// Get balance after the fees
	chainABal, err := chainA.GetBalance(ctx, userAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userAOrigBal-(txAmount+3000), chainABal)

	chainBBal, err := chainB.GetBalance(ctx, userAddressB, chainB.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userBOrigBal+txAmount, chainBBal)

	rlyABal, err := chainA.GetBalance(ctx, rlyAddressA, chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, rlyAOrigBal+2000, rlyABal)

	// // Query for the newly created connection
	// connections, err := r.GetConnections(ctx, eRep, chainA.Config().ChainID)
	// require.NoError(t, err)
	// require.Equal(t, 1, len(connections))

	// // Build bank transfer msg
	// rawMsg, err := json.Marshal(map[string]any{
	// 	"@type":        "/cosmos.bank.v1beta1.MsgSend",
	// 	"from_address": addressA,
	// 	"to_address":   addressB,
	// 	"amount": []map[string]any{
	// 		{
	// 			"denom":  chainB.Config().Denom,
	// 			"amount": 5,
	// 		},
	// 	},
	// })
	// require.NoError(t, err)

	// // Send bank transfer msg to chainB from the user account on chain1
	// sendTransfer := []string{
	// 	chainA.Config().Bin, "tx", "intertx", "submit", string(rawMsg),
	// 	"--connection-id", connections[0].ID,
	// 	"--from", addressA,
	// 	"--chain-id", chainID_A,
	// 	"--home", chainA.HomeDir(),
	// 	"--node", chainA.GetRPCAddress(),
	// 	"--keyring-backend", keyring.BackendTest,
	// }
	// _, _, err = chainA.Exec(ctx, sendTransfer, nil)
	// require.NoError(t, err)

	// // Wait for tx to be relayed
	// err = testutil.WaitForBlocks(ctx, 10, chainB)
	// require.NoError(t, err)

}

// 	chainBBal, err := chainB.GetBalance(ctx, chainBAddr, chainB.Config().Denom)
// 	require.NoError(t, err)
// 	require.Equal(t, chainBOrigBal-transferAmount, chainBBal)

// 	icaBal, err := chainB.GetBalance(ctx, icaAddr, chainB.Config().Denom)
// 	require.NoError(t, err)
// 	require.Equal(t, icaOrigBal+transferAmount, icaBal)

// 	// Build bank transfer msg
// 	rawMsg, err := json.Marshal(map[string]any{
// 		"@type":        "/cosmos.bank.v1beta1.MsgSend",
// 		"from_address": icaAddr,
// 		"to_address":   chainBAddr,
// 		"amount": []map[string]any{
// 			{
// 				"denom":  chainB.Config().Denom,
// 				"amount": strconv.Itoa(transferAmount),
// 			},
// 		},
// 	})
// 	require.NoError(t, err)

// 	// Send bank transfer msg to ICA on chainB from the user account on chain1
// 	sendICATransfer := []string{
// 		chainA.Config().Bin, "tx", "intertx", "submit", string(rawMsg),
// 		"--connection-id", connections[0].ID,
// 		"--from", chainAAddr,
// 		"--chain-id", chainA.Config().ChainID,
// 		"--home", chainA.HomeDir(),
// 		"--node", chainA.GetRPCAddress(),
// 		"--keyring-backend", keyring.BackendTest,
// 		"-y",
// 	}
// 	_, _, err = chainA.Exec(ctx, sendICATransfer, nil)
// 	require.NoError(t, err)

// 	// Wait for tx to be relayed
// 	err = testutil.WaitForBlocks(ctx, 10, chainB)
// 	require.NoError(t, err)

// 	// Assert that the funds have been received by the user account on chainB
// 	chainBBal, err = chainB.GetBalance(ctx, chainBAddr, chainB.Config().Denom)
// 	require.NoError(t, err)
// 	require.Equal(t, chainBOrigBal, chainBBal)

// 	// Assert that the funds have been removed from the ICA on chainB
// 	icaBal, err = chainB.GetBalance(ctx, icaAddr, chainB.Config().Denom)
// 	require.NoError(t, err)
// 	require.Equal(t, icaOrigBal, icaBal)

// 	// Stop the relayer and wait for the process to terminate
// 	err = r.StopRelayer(ctx, eRep)
// 	require.NoError(t, err)

// 	err = testutil.WaitForBlocks(ctx, 5, chainA, chainB)
// 	require.NoError(t, err)

// 	// Send another bank transfer msg to ICA on chainB from the user account on chainA.
// 	// This message should timeout and the channel will be closed when we re-start the relayer.
// 	_, _, err = chainA.Exec(ctx, sendICATransfer, nil)
// 	require.NoError(t, err)

// 	// Wait for approximately one minute to allow packet timeout threshold to be hit
// 	time.Sleep(70 * time.Second)

// 	// Restart the relayer and wait for NextSeqRecv proof to be delivered and packet timed out
// 	err = r.StartRelayer(ctx, eRep, pathChainAChainB)
// 	require.NoError(t, err)

// 	err = testutil.WaitForBlocks(ctx, 15, chainA, chainB)
// 	require.NoError(t, err)

// 	// Assert that the packet timed out and that the acc balances are correct
// 	chainBBal, err = chainB.GetBalance(ctx, chainBAddr, chainB.Config().Denom)
// 	require.NoError(t, err)
// 	require.Equal(t, chainBOrigBal, chainBBal)

// 	icaBal, err = chainB.GetBalance(ctx, icaAddr, chainB.Config().Denom)
// 	require.NoError(t, err)
// 	require.Equal(t, icaOrigBal, icaBal)

// 	// Assert that the channel ends are both closed
// 	chainAChans, err := r.GetChannels(ctx, eRep, chainA.Config().ChainID)
// 	require.NoError(t, err)
// 	require.Equal(t, 1, len(chainAChans))
// 	require.Subset(t, []string{"STATE_CLOSED", "Closed"}, []string{chainAChans[0].State})

// 	chainBChans, err := r.GetChannels(ctx, eRep, chainB.Config().ChainID)
// 	require.NoError(t, err)
// 	require.Equal(t, 1, len(chainBChans))
// 	require.Subset(t, []string{"STATE_CLOSED", "Closed"}, []string{chainBChans[0].State})

// 	// Attempt to open another channel for the same ICA
// 	_, _, err = chainA.Exec(ctx, registerICA, nil)
// 	require.NoError(t, err)

// 	// Wait for channel handshake to finish
// 	err = testutil.WaitForBlocks(ctx, 15, chainA, chainB)
// 	require.NoError(t, err)

// 	// Assert that a new channel has been opened and the same ICA is in use
// 	stdout, _, err = chainA.Exec(ctx, queryICA, nil)
// 	require.NoError(t, err)

// 	newICA := parseInterchainAccountField(stdout)
// 	require.NotEmpty(t, newICA)
// 	require.Equal(t, icaAddr, newICA)

// 	chainAChans, err = r.GetChannels(ctx, eRep, chainA.Config().ChainID)
// 	require.NoError(t, err)
// 	require.Equal(t, 2, len(chainAChans))
// 	require.Subset(t, []string{"STATE_OPEN", "Open"}, []string{chainAChans[1].State})

// 	chainBChans, err = r.GetChannels(ctx, eRep, chainB.Config().ChainID)
// 	require.NoError(t, err)
// 	require.Equal(t, 2, len(chainBChans))
// 	require.Subset(t, []string{"STATE_OPEN", "Open"}, []string{chainBChans[1].State})

// func parseInterchainAccountField(stdout []byte) string {
// 	// After querying an ICA the stdout should look like the following,
// 	// interchain_account_address: cosmos1p76n3mnanllea4d3av0v0e42tjj03cae06xq8fwn9at587rqp23qvxsv0j
// 	// So we split the string at the : and then grab the address and return.
// 	parts := strings.SplitN(string(stdout), ":", 2)
// 	icaAddr := strings.TrimSpace(parts[1])
// 	return icaAddr
// }
