package interchaintest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v9/modules/apps/transfer/types"
	chantypes "github.com/cosmos/ibc-go/v9/modules/core/04-channel/types"
	relayertest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	icrelayer "github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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
		icrelayer.CustomDockerImage(image, "latest", "100:1000"),
		icrelayer.ImagePull(false),
		icrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
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

	rly := r.(*rly.CosmosRelayer)

	// Get all chains
	for _, chain := range [](*cosmos.CosmosChain){chainA.(*cosmos.CosmosChain), chainB.(*cosmos.CosmosChain)} {

		addrs := []string{}

		// loop through nodes to collect rpc addrs
		for _, node := range chain.Validators {
			rpc := fmt.Sprintf("http://%s:26657", node.Name())
			addrs = append(addrs, rpc)
		}

		cmd := []string{"rly", "chains", "set-rpc-addr", chain.Config().Name, addrs[numVals-1], "--home", rly.HomeDir()}
		res := r.Exec(ctx, eRep, cmd, nil)
		require.NoError(t, res.Err)

		cmd = []string{"rly", "chains", "set-backup-rpc-addrs", chain.Config().Name, strings.Join(addrs[:numVals-1], ","), "--home", rly.HomeDir()}
		res = r.Exec(ctx, eRep, cmd, nil)
		require.NoError(t, res.Err)
	}

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

	// wait 10 blocks
	require.NoError(t, testutil.WaitForBlocks(ctx, 10, chainA, chainB))

	// turn off last nodes on both chains
	val := chainA.(*cosmos.CosmosChain).Validators[numVals-1]
	err = val.StopContainer(ctx)
	require.NoError(t, err)

	val = chainB.(*cosmos.CosmosChain).Validators[numVals-1]
	err = val.StopContainer(ctx)
	require.NoError(t, err)

	// wait 10 blocks
	require.NoError(t, testutil.WaitForBlocks(ctx, 10, chainA, chainB))

	// send ibc tx from chain a to b
	tx, err := chainA.SendIBCTransfer(ctx, channel.ChannelID, userA.KeyName(), transferAB, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// get chain b height
	bHeight, err := chainB.Height(ctx)
	require.NoError(t, err)

	// Poll for MsgRecvPacket on b chain
	_, err = cosmos.PollForMessage[*chantypes.MsgRecvPacket](ctx, chainB.(*cosmos.CosmosChain), cosmos.DefaultEncoding().InterfaceRegistry, bHeight, bHeight+20, nil)
	require.NoError(t, err)

	// get chain a height
	aHeight, err := chainA.Height(ctx)
	require.NoError(t, err)

	// poll for acknowledge on 'a' chain
	_, err = testutil.PollForAck(ctx, chainA.(*cosmos.CosmosChain), aHeight, aHeight+30, tx.Packet)
	require.NoError(t, err)

	// Compose the ibc denom for balance assertions on the counterparty and assert balances.
	denom := transfertypes.NewDenom(
		chainA.Config().Denom,
		transfertypes.NewHop(
			channel.Counterparty.PortID,
			channel.Counterparty.ChannelID,
		),
	).IBCDenom()
	// validate user balances on both chains
	userABal, err = chainA.GetBalance(ctx, userA.FormattedAddress(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, userABal.Equal(initBal.Sub(transferAmount)))

	userBBal, err = chainB.GetBalance(ctx, userB.FormattedAddress(), denom)
	require.NoError(t, err)
	require.True(t, userBBal.Equal(transferAmount))
}
