package interchaintest_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/cosmos/relayer/v2/cmd"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"
)

// TestMemoAndReceiverLimit tests the functionality of sending transfers with a memo and receiver limit configured.
func TestMemoAndReceiverLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 0

	image := ibc.DockerImage{
		Repository: "ghcr.io/cosmos/ibc-go-simd",
		Version:    "v8.0.0",
		UidGid:     "100:1000",
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "ibc-go-simd",
			Version:       "main",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
			ChainConfig: ibc.ChainConfig{
				Type:          "cosmos",
				Name:          "simd",
				ChainID:       "chain-a",
				Images:        []ibc.DockerImage{image},
				Bin:           "simd",
				Bech32Prefix:  "cosmos",
				Denom:         "stake",
				CoinType:      "118",
				GasPrices:     "0.0stake",
				GasAdjustment: 1.1,
			},
		},
		{
			Name:          "ibc-go-simd",
			Version:       "main",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
			ChainConfig: ibc.ChainConfig{
				Type:          "cosmos",
				Name:          "simd",
				ChainID:       "chain-b",
				Images:        []ibc.DockerImage{image},
				Bin:           "simd",
				Bech32Prefix:  "cosmos",
				Denom:         "stake",
				CoinType:      "118",
				GasPrices:     "0.0stake",
				GasAdjustment: 1.1,
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA, chainB := chains[0], chains[1]

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	rf := relayertest.NewRelayerFactory(relayertest.RelayerConfig{InitialBlockHistory: 50})
	r := rf.Build(t, client, network)

	const pathName = "chainA-chainB"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    pathName,
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and fund user accs & assert initial balances.
	initBal := sdkmath.NewInt(1_000_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), initBal, chainA, chainB, chainA, chainB)

	require.NoError(t, testutil.WaitForBlocks(ctx, 2, chainA, chainB))

	userA := users[0]
	userB := users[1]
	userC := users[2]
	userD := users[3]

	userABal, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userABal))

	userCBal, err := chainA.GetBalance(ctx, userC.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userCBal))

	userBBal, err := chainB.GetBalance(ctx, userB.FormattedAddress(), chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userBBal))

	userDBal, err := chainB.GetBalance(ctx, userD.FormattedAddress(), chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userDBal))

	// Read relayer config from disk, configure memo limit, & write config back to disk.
	relayer := r.(*relayertest.Relayer)

	cfg := relayer.Sys().MustGetConfig(t)

	cfg.Global.ICS20MemoLimit = 10

	cfgOutput := new(cmd.ConfigOutputWrapper)
	cfgOutput.ProviderConfigs = cmd.ProviderConfigs{}
	cfgOutput.Global = cfg.Global
	cfgOutput.Paths = cfg.Paths

	for _, c := range cfg.ProviderConfigs {
		stuff := c.Value.(*cosmos.CosmosProviderConfig)

		cfgOutput.ProviderConfigs[stuff.ChainID] = &cmd.ProviderConfigWrapper{
			Type:  "cosmos",
			Value: stuff,
		}
	}

	cfgBz, err := yaml.Marshal(cfgOutput)
	require.NoError(t, err)

	err = relayer.Sys().WriteConfig(t, cfgBz)
	require.NoError(t, err)

	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occurred while stopping the relayer: %s", err)
			}
		},
	)

	// Send transfers that should succeed & assert balances.
	channels, err := r.GetChannels(ctx, eRep, chainA.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(channels))

	channel := channels[0]

	transferAmount := sdkmath.NewInt(1_000)

	transferAB := ibc.WalletAmount{
		Address: userB.FormattedAddress(),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	transferCD := ibc.WalletAmount{
		Address: userD.FormattedAddress(),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	_, err = chainA.SendIBCTransfer(ctx, channel.ChannelID, userA.KeyName(), transferAB, ibc.TransferOptions{})
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, channel.ChannelID, userC.KeyName(), transferCD, ibc.TransferOptions{})
	require.NoError(t, err)

	require.NoError(t, testutil.WaitForBlocks(ctx, 5, chainA, chainB))

	// Compose the ibc denom for balance assertions on the counterparty and assert balances.
	denom := transfertypes.GetPrefixedDenom(
		channel.Counterparty.PortID,
		channel.Counterparty.ChannelID,
		chainA.Config().Denom,
	)
	trace := transfertypes.ParseDenomTrace(denom)

	userABal, err = chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, userABal.Equal(initBal.Sub(transferAmount)))

	userBBal, err = chainB.GetBalance(ctx, userB.FormattedAddress(), trace.IBCDenom())
	require.NoError(t, err)
	require.True(t, userBBal.Equal(transferAmount))

	userCBal, err = chainA.GetBalance(ctx, userC.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, userCBal.Equal(initBal.Sub(transferAmount)))

	userDBal, err = chainB.GetBalance(ctx, userD.FormattedAddress(), trace.IBCDenom())
	require.NoError(t, err)
	require.True(t, userDBal.Equal(transferAmount))

	// Send transfer with memo that exceeds limit, ensure transfer failed and assert balances.
	opts := ibc.TransferOptions{
		Memo: "this memo is too long",
		Timeout: &ibc.IBCTimeout{
			Height: 10,
		},
	}

	_, err = chainA.SendIBCTransfer(ctx, channel.ChannelID, userA.KeyName(), transferAB, opts)
	require.NoError(t, err)

	require.NoError(t, testutil.WaitForBlocks(ctx, 11, chainA, chainB))

	userABal, err = chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, !userABal.Equal(initBal.Sub(transferAmount)))

	userBBal, err = chainB.GetBalance(ctx, userB.FormattedAddress(), trace.IBCDenom())
	require.NoError(t, err)
	require.True(t, userBBal.Equal(transferAmount))

	// Send transfer with receiver field that exceeds limit, ensure transfer failed and assert balances.
	var junkReceiver string

	for i := 0; i < 130; i++ {
		junkReceiver += "a"
	}

	transferCD = ibc.WalletAmount{
		Address: junkReceiver,
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	_, err = chainA.SendIBCTransfer(ctx, channel.ChannelID, userC.KeyName(), transferCD, ibc.TransferOptions{})
	require.NoError(t, err)

	require.NoError(t, testutil.WaitForBlocks(ctx, 5, chainA, chainB))

	userCBal, err = chainA.GetBalance(ctx, userC.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, !userCBal.Equal(initBal.Sub(transferAmount)))

	userDBal, err = chainB.GetBalance(ctx, userD.FormattedAddress(), trace.IBCDenom())
	require.NoError(t, err)
	require.True(t, userDBal.Equal(transferAmount))
}
