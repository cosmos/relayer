package interchaintest_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestScenarioInterchainAccounts is a test case that performs simulations and assertions around some basic
// features and packet flows surrounding interchain accounts. See: https://github.com/cosmos/interchain-accounts-demo
func TestScenarioInterchainAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get both chains
	nf := 0
	nv := 1
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "icad",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig: ibc.ChainConfig{
				Images:                 []ibc.DockerImage{{Repository: "ghcr.io/cosmos/ibc-go-icad", Version: "v0.5.0"}},
				UsingNewGenesisCommand: true,
			},
		},
		{
			Name:          "icad",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig: ibc.ChainConfig{
				Images:                 []ibc.DockerImage{{Repository: "ghcr.io/cosmos/ibc-go-icad", Version: "v0.5.0"}},
				UsingNewGenesisCommand: true,
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	// Get a relayer instance
	r := relayerinterchaintest.
		NewRelayerFactory(relayerinterchaintest.RelayerConfig{}).
		Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "test-path"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(chain1).
		AddChain(chain2).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Relayer: r,
			Path:    pathName,
		})

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
		// Uncomment this to load blocks, txs, msgs, and events into sqlite db as test runs
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
	}))

	t.Parallel()

	// Fund a user account on chain1 and chain2
	const userFunds = int64(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1, chain2)
	chain1User := users[0]
	chain2User := users[1]

	// Generate a new IBC path
	err = r.GeneratePath(ctx, eRep, chain1.Config().ChainID, chain2.Config().ChainID, pathName)
	require.NoError(t, err)

	// Create new clients
	err = r.CreateClients(ctx, eRep, pathName, ibc.CreateClientOptions{TrustingPeriod: "330h"})
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, chain1, chain2)
	require.NoError(t, err)

	// Create a new connection
	err = r.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, chain1, chain2)
	require.NoError(t, err)

	// Query for the newly created connection
	connections, err := r.GetConnections(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(connections))

	// Start the relayer and set the cleanup function.
	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Register a new interchain account on chain2, on behalf of the user acc on chain1
	chain1Addr := chain1User.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(chain1.Config().Bech32Prefix)

	registerICA := []string{
		chain1.Config().Bin, "tx", "intertx", "register",
		"--from", chain1Addr,
		"--connection-id", connections[0].ID,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", chain1.GetRPCAddress(),
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	_, _, err = chain1.Exec(ctx, registerICA, nil)
	require.NoError(t, err)

	ir := cosmos.DefaultEncoding().InterfaceRegistry

	c2h, err := chain2.Height(ctx)
	require.NoError(t, err)

	channelFound := func(found *chantypes.MsgChannelOpenConfirm) bool {
		return found.PortId == "icahost"
	}

	// Wait for channel open confirm
	_, err = cosmos.PollForMessage(ctx, chain2, ir,
		c2h, c2h+30, channelFound)
	require.NoError(t, err)

	// Query for the newly registered interchain account
	queryICA := []string{
		chain1.Config().Bin, "query", "intertx", "interchainaccounts", connections[0].ID, chain1Addr,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", chain1.GetRPCAddress(),
	}
	stdout, _, err := chain1.Exec(ctx, queryICA, nil)
	require.NoError(t, err)

	icaAddr := parseInterchainAccountField(stdout)
	require.NotEmpty(t, icaAddr)

	// Get initial account balances
	chain2Addr := chain2User.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(chain2.Config().Bech32Prefix)

	chain2OrigBal, err := chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)

	icaOrigBal, err := chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)

	// Send funds to ICA from user account on chain2
	const transferAmount = 10000
	transfer := ibc.WalletAmount{
		Address: icaAddr,
		Denom:   chain2.Config().Denom,
		Amount:  transferAmount,
	}
	err = chain2.SendFunds(ctx, chain2User.KeyName(), transfer)
	require.NoError(t, err)

	chain2Bal, err := chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain2OrigBal-transferAmount, chain2Bal)

	icaBal, err := chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, icaOrigBal+transferAmount, icaBal)

	// Build bank transfer msg
	rawMsg, err := json.Marshal(map[string]any{
		"@type":        "/cosmos.bank.v1beta1.MsgSend",
		"from_address": icaAddr,
		"to_address":   chain2Addr,
		"amount": []map[string]any{
			{
				"denom":  chain2.Config().Denom,
				"amount": strconv.Itoa(transferAmount),
			},
		},
	})
	require.NoError(t, err)

	// Send bank transfer msg to ICA on chain2 from the user account on chain1
	sendICATransfer := []string{
		chain1.Config().Bin, "tx", "intertx", "submit", string(rawMsg),
		"--connection-id", connections[0].ID,
		"--from", chain1Addr,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", chain1.GetRPCAddress(),
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	_, _, err = chain1.Exec(ctx, sendICATransfer, nil)
	require.NoError(t, err)

	c1h, err := chain1.Height(ctx)
	require.NoError(t, err)

	ackFound := func(found *chantypes.MsgAcknowledgement) bool {
		return found.Packet.Sequence == 1 &&
			found.Packet.SourcePort == "icacontroller-"+chain1Addr &&
			found.Packet.DestinationPort == "icahost"
	}

	// Wait for ack
	_, err = cosmos.PollForMessage(ctx, chain1, ir,
		c1h, c1h+10, ackFound)
	require.NoError(t, err)

	// Assert that the funds have been received by the user account on chain2
	chain2Bal, err = chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain2OrigBal, chain2Bal)

	// Assert that the funds have been removed from the ICA on chain2
	icaBal, err = chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, icaOrigBal, icaBal)

	// Stop the relayer and wait for the process to terminate
	err = r.StopRelayer(ctx, eRep)
	require.NoError(t, err)

	// Send another bank transfer msg to ICA on chain2 from the user account on chain1.
	// This message should timeout and the channel will be closed when we re-start the relayer.
	_, _, err = chain1.Exec(ctx, sendICATransfer, nil)
	require.NoError(t, err)

	// Wait for approximately one minute to allow packet timeout threshold to be hit
	time.Sleep(70 * time.Second)

	// Restart the relayer and wait for NextSeqRecv proof to be delivered and packet timed out
	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	c2h, err = chain2.Height(ctx)
	require.NoError(t, err)

	chanCloseFound := func(found *chantypes.MsgChannelCloseConfirm) bool {
		return found.PortId == "icahost"
	}

	// Wait for channel close confirm
	_, err = cosmos.PollForMessage(ctx, chain2, ir, c2h, c2h+30, chanCloseFound)
	require.NoError(t, err)

	// Assert that the packet timed out and that the acc balances are correct
	chain2Bal, err = chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain2OrigBal, chain2Bal)

	icaBal, err = chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, icaOrigBal, icaBal)

	// Assert that the channel ends are both closed
	chain1Chans, err := r.GetChannels(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(chain1Chans))
	require.Subset(t, []string{"STATE_CLOSED", "Closed"}, []string{chain1Chans[0].State})

	chain2Chans, err := r.GetChannels(ctx, eRep, chain2.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(chain2Chans))
	require.Subset(t, []string{"STATE_CLOSED", "Closed"}, []string{chain2Chans[0].State})

	// Attempt to open another channel for the same ICA
	_, _, err = chain1.Exec(ctx, registerICA, nil)
	require.NoError(t, err)

	// Wait for channel handshake to finish
	c2h, err = chain2.Height(ctx)
	require.NoError(t, err)

	// Wait for channel open confirm
	_, err = cosmos.PollForMessage(ctx, chain2, ir,
		c2h, c2h+30, channelFound)
	require.NoError(t, err)

	// Assert that a new channel has been opened and the same ICA is in use
	stdout, _, err = chain1.Exec(ctx, queryICA, nil)
	require.NoError(t, err)

	newICA := parseInterchainAccountField(stdout)
	require.NotEmpty(t, newICA)
	require.Equal(t, icaAddr, newICA)

	chain1Chans, err = r.GetChannels(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 2, len(chain1Chans))
	require.Subset(t, []string{"STATE_OPEN", "Open"}, []string{chain1Chans[1].State})

	chain2Chans, err = r.GetChannels(ctx, eRep, chain2.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 2, len(chain2Chans))
	require.Subset(t, []string{"STATE_OPEN", "Open"}, []string{chain2Chans[1].State})
}

// parseInterchainAccountField takes a slice of bytes which should be returned when querying for an ICA via
// the 'intertx interchainaccounts' cmd and splices out the actual address portion.
func parseInterchainAccountField(stdout []byte) string {
	// After querying an ICA the stdout should look like the following,
	// interchain_account_address: cosmos1p76n3mnanllea4d3av0v0e42tjj03cae06xq8fwn9at587rqp23qvxsv0j
	// So we split the string at the : and then grab the address and return.
	parts := strings.SplitN(string(stdout), ":", 2)
	icaAddr := strings.TrimSpace(parts[1])
	return icaAddr
}
