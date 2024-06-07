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
	interchaintestcosmos "github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v8/relayer"
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

	numVals := 5
	numFullNodes := 0

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "gaia",
			Version:       "v7.0.0",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,

			ChainConfig: ibc.ChainConfig{
				GasPrices: "0.0uatom",
			},
		},
		{
			Name:          "osmosis",
			Version:       "v11.0.0",
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
			ChainConfig: ibc.ChainConfig{
				GasPrices: "0.0uosmo",
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA, chainB := chains[0], chains[1]

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	image := relayertest.BuildRelayerImage(t)
	rf := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		interchaintestrelayer.CustomDockerImage(image, "latest", "100:1000"),
		interchaintestrelayer.ImagePull(false),
		interchaintestrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
	)

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

	// Get all chains
	for _, chain := range [](*interchaintestcosmos.CosmosChain){chainA.(*interchaintestcosmos.CosmosChain), chainB.(*interchaintestcosmos.CosmosChain)} {

		addrs := []string{}

		// loop through nodes to collect rpc addrs
		for _, node := range chain.Validators {

			cl := node.DockerClient
			inspect, _ := cl.ContainerInspect(ctx, node.ContainerID())
			require.NoError(t, err)

			portBindings := inspect.HostConfig.PortBindings
			hostPort := ""

			// find the host port mapped to 26657 on the container
			for port, bindings := range portBindings {
				if port == "26657/tcp" {
					for _, binding := range bindings {
						hostPort = binding.HostPort
						break
					}
					break
				}
			}

			rpc := fmt.Sprintf("http://127.0.0.1:%s", hostPort)
			addrs = append(addrs, rpc)
		}

		// TODO: REMOVE ME
		t.Log(addrs)

		value := cfg.ProviderConfigs[chain.Config().ChainID].Value.(*cosmos.CosmosProviderConfig)
		value.RPCAddr = addrs[numVals-1]
		value.BackupRPCAddrs = addrs[:numVals-1]

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

	t.Log("========== BEFORE SHUT DOWN LAST VALIDATOR ==========")
	t.Log("========== BEFORE SHUT DOWN LAST VALIDATOR ==========")
	t.Log("========== BEFORE SHUT DOWN LAST VALIDATOR ==========")

	// wait 10 blocks
	require.NoError(t, testutil.WaitForBlocks(ctx, 10, chainA, chainB))

	// turn off last node
	val := chainA.(*interchaintestcosmos.CosmosChain).Validators[numVals-1]
	err = val.PauseContainer(ctx)
	require.NoError(t, err)

	// wait 10 blocks
	require.NoError(t, testutil.WaitForBlocks(ctx, 10, chainA, chainB))

	t.Log("========== AFTER SHUT DOWN LAST VALIDATOR ==========")
	t.Log("========== AFTER SHUT DOWN LAST VALIDATOR ==========")
	t.Log("========== AFTER SHUT DOWN LAST VALIDATOR ==========")

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
