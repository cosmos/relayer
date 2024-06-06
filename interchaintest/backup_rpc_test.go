package interchaintest_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/cosmos/relayer/v2/cmd"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8"
	iccosmos "github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"
)

// TestBackupRpcs tests the functionality of falling back to secondary RPCs when the primary node goes offline or becomes unresponsive.
func TestBackupRpcs(t *testing.T) {
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
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and fund user accs & assert initial balances.
	initBal := sdkmath.NewInt(1_000_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), initBal, chainA, chainB)
	require.NoError(t, testutil.WaitForBlocks(ctx, 2, chainA, chainB))

	userA := users[0]
	userB := users[1]

	userABal, err := chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userABal))

	userBBal, err := chainB.GetBalance(ctx, userB.FormattedAddress(), chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBal.Equal(userBBal))

	// Read relayer config from disk, configure memo limit, & write config back to disk.
	relayer := r.(*relayertest.Relayer)

	cfg := relayer.Sys().MustGetConfig(t)
	cfgOutput := new(cmd.ConfigOutputWrapper)
	cfgOutput.ProviderConfigs = cmd.ProviderConfigs{}
	cfgOutput.Global = cfg.Global
	cfgOutput.Paths = cfg.Paths

	// Get nodes associated with chainA
	for _, chain := range [](*iccosmos.CosmosChain){chainA.(*iccosmos.CosmosChain), chainB.(*iccosmos.CosmosChain)} {

		addrs := []string{}

		for _, node := range chain.FullNodes {

			rpc := fmt.Sprintf("http://%s", node.HostRPCPort)
			addrs = append(addrs, rpc)
		}

		t.Log(addrs)

		value := cfg.ProviderConfigs[chain.Config().ChainID].Value.(*cosmos.CosmosProviderConfig)
		value.RPCAddr = addrs[0]
		value.BackupRPCAddrs = addrs[1:]

		cfgOutput.ProviderConfigs[chain.Config().ChainID] = &cmd.ProviderConfigWrapper{
			Type:  "cosmos",
			Value: value,
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

	_, err = chainA.SendIBCTransfer(ctx, channel.ChannelID, userA.KeyName(), transferAB, ibc.TransferOptions{})
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
}
