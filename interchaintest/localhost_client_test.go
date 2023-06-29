package interchaintest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestLocalhost_TokenTransfers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 0
	image := ibc.DockerImage{
		Repository: "ghcr.io/cosmos/ibc-go-simd",
		Version:    "v7.1.0-rc0",
		UidGid:     "",
	}
	cdc := cosmos.DefaultEncoding()
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "ibc-go-simd",
			ChainName:     "simd",
			Version:       "main",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
			ChainConfig: ibc.ChainConfig{
				Type:                   "cosmos",
				Name:                   "simd",
				ChainID:                "chain-a",
				Images:                 []ibc.DockerImage{image},
				Bin:                    "simd",
				Bech32Prefix:           "cosmos",
				Denom:                  "stake",
				CoinType:               "118",
				GasPrices:              "0.0stake",
				GasAdjustment:          1.1,
				EncodingConfig:         &cdc,
				UsingNewGenesisCommand: true,
			}}},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathLocalhost = "chainA-localhost"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddRelayer(r, "relayer")

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	const relayerKey = "relayer-key"
	const mnemonic = "all unit ordinary card sword document left illegal frog chuckle assume gift south settle can explain wagon beef story praise gorilla arch close good"

	// initialize a new acc for the relayer along with a couple user accs
	initBal := int64(10_000_000)
	_, err = interchaintest.GetAndFundTestUserWithMnemonic(ctx, relayerKey, mnemonic, initBal, chainA)
	require.NoError(t, err)

	users := interchaintest.GetAndFundTestUsers(t, ctx, "test-key", initBal, chainA, chainA)
	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	userA, userB := users[0], users[1]

	// assert initial balances are correct
	userABal, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, initBal, userABal)

	userBBal, err := chainA.GetBalance(ctx, userB.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, initBal, userBBal)

	// configure the relayer for a localhost connection
	err = r.AddChainConfiguration(ctx, eRep, chainA.Config(), relayerKey, chainA.GetHostRPCAddress(), chainA.GetHostGRPCAddress())
	require.NoError(t, err)

	err = r.RestoreKey(ctx, eRep, chainA.Config(), relayerKey, mnemonic)
	require.NoError(t, err)

	err = r.GeneratePath(ctx, eRep, chainA.Config().ChainID, chainA.Config().ChainID, pathLocalhost)
	require.NoError(t, err)

	updateCmd := []string{
		"paths", "update", pathLocalhost,
		"--src-client-id", ibcexported.LocalhostClientID,
		"--src-connection-id", ibcexported.LocalhostConnectionID,
		"--dst-client-id", ibcexported.LocalhostClientID,
		"--dst-connection-id", ibcexported.LocalhostConnectionID,
	}
	res := r.Exec(ctx, eRep, updateCmd, nil)
	require.NoError(t, res.Err)

	// initialize new channels
	err = r.CreateChannel(ctx, eRep, pathLocalhost, ibc.DefaultChannelOpts())
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 10, chainA)
	require.NoError(t, err)

	channels, err := r.GetChannels(ctx, eRep, chainA.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 2, len(channels))

	channel := channels[0]

	// compose the ibc denom for balance assertions
	denom := transfertypes.GetPrefixedDenom(channel.Counterparty.PortID, channel.Counterparty.ChannelID, chainA.Config().Denom)
	trace := transfertypes.ParseDenomTrace(denom)

	// start the relayer
	require.NoError(t, r.StartRelayer(ctx, eRep, pathLocalhost))

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// compose and send a localhost IBC transfer which should be successful
	const transferAmount = int64(1_000)
	transfer := ibc.WalletAmount{
		Address: userB.FormattedAddress(),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	cmd := []string{
		chainA.Config().Bin, "tx", "ibc-transfer", "transfer", "transfer",
		channel.ChannelID,
		transfer.Address,
		fmt.Sprintf("%d%s", transfer.Amount, transfer.Denom),
		"--from", userA.FormattedAddress(),
		"--gas-prices", "0.0stake",
		"--gas-adjustment", "1.2",
		"--keyring-backend", "test",
		"--absolute-timeouts",
		"--packet-timeout-timestamp", "9999999999999999999",
		"--output", "json",
		"-y",
		"--home", chainA.HomeDir(),
		"--node", chainA.GetRPCAddress(),
		"--chain-id", chainA.Config().ChainID,
	}
	_, _, err = chainA.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	// assert that the updated balances are correct
	newBalA, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userABal-transferAmount, newBalA)

	newBalB, err := chainA.GetBalance(ctx, userB.FormattedAddress(), trace.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, transferAmount, newBalB)

	// compose and send another localhost IBC transfer which should succeed
	cmd = []string{
		chainA.Config().Bin, "tx", "ibc-transfer", "transfer", "transfer",
		channel.ChannelID,
		transfer.Address,
		fmt.Sprintf("%d%s", transfer.Amount, transfer.Denom),
		"--from", userA.FormattedAddress(),
		"--gas-prices", "0.0stake",
		"--gas-adjustment", "1.2",
		"--keyring-backend", "test",
		"--absolute-timeouts",
		"--packet-timeout-timestamp", "9999999999999999999",
		"--output", "json",
		"-y",
		"--home", chainA.HomeDir(),
		"--node", chainA.GetRPCAddress(),
		"--chain-id", chainA.Config().ChainID,
	}
	_, _, err = chainA.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	// assert that the balances are updated
	tmpBalA := newBalA
	newBalA, err = chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, tmpBalA-transferAmount, newBalA)

	tmpBalB := newBalB
	newBalB, err = chainA.GetBalance(ctx, userB.FormattedAddress(), trace.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, tmpBalB+transferAmount, newBalB)
}

func TestLocalhost_InterchainAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 0
	image := ibc.DockerImage{
		Repository: "ghcr.io/cosmos/ibc-go-simd",
		Version:    "v7.1.0-rc0",
		UidGid:     "",
	}
	cdc := cosmos.DefaultEncoding()
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "ibc-go-simd",
			ChainName:     "simd",
			Version:       "main",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
			ChainConfig: ibc.ChainConfig{
				Type:                   "cosmos",
				Name:                   "simd",
				ChainID:                "chain-a",
				Images:                 []ibc.DockerImage{image},
				Bin:                    "simd",
				Bech32Prefix:           "cosmos",
				Denom:                  "stake",
				CoinType:               "118",
				GasPrices:              "0.0stake",
				GasAdjustment:          1.1,
				EncodingConfig:         &cdc,
				UsingNewGenesisCommand: true,
			}}},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathLocalhost = "chainA-localhost"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddRelayer(r, "relayer")

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	const (
		relayerKey = "relayer-key"
		mnemonic   = "all unit ordinary card sword document left illegal frog chuckle assume gift south settle can explain wagon beef story praise gorilla arch close good"
	)

	// initialize a new acc for the relayer along with a new user acc
	const initBal = int64(10_000_000)
	_, err = interchaintest.GetAndFundTestUserWithMnemonic(ctx, relayerKey, mnemonic, initBal, chainA)
	require.NoError(t, err)

	users := interchaintest.GetAndFundTestUsers(t, ctx, "test-key", initBal, chainA)
	userA := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	// assert initial balance is correct
	userABal, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, initBal, userABal)

	// configure the relayer for a localhost connection
	err = r.AddChainConfiguration(ctx, eRep, chainA.Config(), relayerKey, chainA.GetHostRPCAddress(), chainA.GetHostGRPCAddress())
	require.NoError(t, err)

	err = r.RestoreKey(ctx, eRep, chainA.Config(), relayerKey, mnemonic)
	require.NoError(t, err)

	err = r.GeneratePath(ctx, eRep, chainA.Config().ChainID, chainA.Config().ChainID, pathLocalhost)
	require.NoError(t, err)

	updateCmd := []string{
		"paths", "update", pathLocalhost,
		"--src-client-id", ibcexported.LocalhostClientID,
		"--src-connection-id", ibcexported.LocalhostConnectionID,
		"--dst-client-id", ibcexported.LocalhostClientID,
		"--dst-connection-id", ibcexported.LocalhostConnectionID,
	}
	res := r.Exec(ctx, eRep, updateCmd, nil)
	require.NoError(t, res.Err)

	// start the relayer
	require.NoError(t, r.StartRelayer(ctx, eRep, pathLocalhost))

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// register a new interchain account
	registerCmd := []string{
		chainA.Config().Bin, "tx", "interchain-accounts", "controller", "register", ibcexported.LocalhostConnectionID,
		"--from", userA.FormattedAddress(),
		"--gas-prices", "0.0stake",
		"--gas-adjustment", "1.2",
		"--keyring-backend", "test",
		"--output", "json",
		"-y",
		"--home", chainA.HomeDir(),
		"--node", chainA.GetRPCAddress(),
		"--chain-id", chainA.Config().ChainID,
	}

	_, _, err = chainA.Exec(ctx, registerCmd, nil)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 10, chainA)
	require.NoError(t, err)

	channels, err := r.GetChannels(ctx, eRep, chainA.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 2, len(channels))

	// query for the newly created ica
	queryCmd := []string{
		chainA.Config().Bin, "q", "interchain-accounts", "controller", "interchain-account",
		userA.FormattedAddress(), ibcexported.LocalhostConnectionID,
		"--home", chainA.HomeDir(),
		"--node", chainA.GetRPCAddress(),
		"--chain-id", chainA.Config().ChainID,
	}
	stdout, _, err := chainA.Exec(ctx, queryCmd, nil)
	require.NoError(t, err)

	icaAddr := parseInterchainAccountField(stdout)
	require.NotEmpty(t, icaAddr)

	// asser the ICA balance, send some funds to the ICA, then re-assert balances
	icaBal, err := chainA.GetBalance(ctx, icaAddr, chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, int64(0), icaBal)

	const transferAmount = 1000
	transfer := ibc.WalletAmount{
		Address: icaAddr,
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}
	err = chainA.SendFunds(ctx, userA.KeyName(), transfer)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	newBalICA, err := chainA.GetBalance(ctx, icaAddr, chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, int64(transferAmount), newBalICA)

	newBalA, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, userABal-transferAmount, newBalA)

	// compose msg to send to ICA
	rawMsg, err := json.Marshal(map[string]any{
		"@type":        "/cosmos.bank.v1beta1.MsgSend",
		"from_address": icaAddr,
		"to_address":   userA.FormattedAddress(),
		"amount": []map[string]any{
			{
				"denom":  chainA.Config().Denom,
				"amount": strconv.Itoa(transferAmount),
			},
		},
	})
	require.NoError(t, err)

	generateCmd := []string{
		chainA.Config().Bin, "tx", "interchain-accounts", "host", "generate-packet-data", string(rawMsg),
	}
	msgBz, _, err := chainA.Exec(ctx, generateCmd, nil)
	require.NoError(t, err)

	// send tx to our ICA
	sendCmd := []string{
		chainA.Config().Bin, "tx", "interchain-accounts", "controller", "send-tx",
		ibcexported.LocalhostConnectionID, string(msgBz),
		"--from", userA.FormattedAddress(),
		"--gas-prices", "0.0stake",
		"--gas-adjustment", "1.2",
		"--keyring-backend", "test",
		"--output", "json",
		"-y",
		"--home", chainA.HomeDir(),
		"--node", chainA.GetRPCAddress(),
		"--chain-id", chainA.Config().ChainID,
	}
	_, _, err = chainA.Exec(ctx, sendCmd, nil)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	// assert updated balances are correct
	finalBalICA, err := chainA.GetBalance(ctx, icaAddr, chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, newBalICA-transferAmount, finalBalICA)

	finalBalA, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, newBalA+int64(transferAmount), finalBalA)
}
