package stride_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	rlystride "github.com/cosmos/relayer/v2/relayer/chains/cosmos/stride"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

// TestStrideICAandICQ is a test case that performs simulations and assertions around interchain accounts
// and the client implementation of interchain queries. See: https://github.com/Stride-Labs/interchain-queries
func TestScenarioStrideICAandICQ(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	nf := 0
	nv := 1
	logger := zaptest.NewLogger(t)

	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(logger, []*interchaintest.ChainSpec{
		{
			Name:          "stride",
			ChainName:     "stride",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "stride",
				ChainID: "stride-1",
				Images: []ibc.DockerImage{{
					Repository: "ghcr.io/strangelove-ventures/heighliner/stride",
					Version:    "andrew-test_admin_v5.1.1",
					UidGid:     "1025:1025",
				}},
				Bin:            "strided",
				Bech32Prefix:   "stride",
				Denom:          "ustrd",
				GasPrices:      "0.0ustrd",
				TrustingPeriod: TrustingPeriod,
				GasAdjustment:  1.1,
				ModifyGenesis:  ModifyGenesisStride(),
				EncodingConfig: StrideEncoding(),
			}},
		{
			Name:          "gaia",
			ChainName:     "gaia",
			Version:       "v8.0.0",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig: ibc.ChainConfig{
				ModifyGenesis:  ModifyGenesisStrideCounterparty(),
				TrustingPeriod: TrustingPeriod,
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	stride, gaia := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)
	strideCfg, gaiaCfg := stride.Config(), gaia.Config()

	r := relayerinterchaintest.NewRelayer(t, relayerinterchaintest.RelayerConfig{})

	// Build the network; spin up the chains and configure the relayer
	const pathStrideGaia = "stride-gaia"
	const relayerName = "relayer"

	clientOpts := ibc.DefaultClientOpts()
	clientOpts.TrustingPeriod = TrustingPeriod

	ic := interchaintest.NewInterchain().
		AddChain(stride).
		AddChain(gaia).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:           stride,
			Chain2:           gaia,
			Relayer:          r,
			Path:             pathStrideGaia,
			CreateClientOpts: clientOpts,
		})

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// Uncomment this to load blocks, txs, msgs, and events into sqlite db as test runs
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	}))

	t.Parallel()

	t.Cleanup(func() {
		_ = ic.Close()
	})

	logger.Info("TestScenarioStrideICAandICQ [1]")

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, stride, gaia)
	strideUser, gaiaUser := users[0], users[1]

	strideFullNode := stride.Validators[0]

	// Start the relayer
	err = r.StartRelayer(ctx, eRep, pathStrideGaia)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [2]")

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occurred while stopping the relayer: %s", err)
			}
		},
	)

	// Recover stride admin key
	err = stride.RecoverKey(ctx, StrideAdminAccount, StrideAdminMnemonic)
	require.NoError(t, err)

	strideAdminAddrBytes, err := stride.GetAddress(ctx, StrideAdminAccount)
	require.NoError(t, err)

	strideAdminAddr, err := types.Bech32ifyAddressBytes(strideCfg.Bech32Prefix, strideAdminAddrBytes)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [3]")

	err = stride.SendFunds(ctx, interchaintest.FaucetAccountKeyName, ibc.WalletAmount{
		Address: strideAdminAddr,
		Amount:  userFunds,
		Denom:   strideCfg.Denom,
	})
	require.NoError(t, err, "failed to fund stride admin account")

	logger.Info("TestScenarioStrideICAandICQ [4]")

	// get native chain user addresses
	strideAddr := strideUser.FormattedAddress()
	require.NotEmpty(t, strideAddr)
	logger.Info("TestScenarioStrideICAandICQ [5]", zap.String("stride addr", strideAddr))

	gaiaAddress := gaiaUser.FormattedAddress()
	require.NotEmpty(t, gaiaAddress)
	logger.Info("TestScenarioStrideICAandICQ [6]", zap.String("gaia addr", gaiaAddress))

	// get ibc paths
	gaiaConns, err := r.GetConnections(ctx, eRep, gaiaCfg.ChainID)
	require.NoError(t, err)

	gaiaChans, err := r.GetChannels(ctx, eRep, gaiaCfg.ChainID)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [7]")

	atomIBCDenom := transfertypes.ParseDenomTrace(
		transfertypes.GetPrefixedDenom(
			gaiaChans[0].Counterparty.PortID,
			gaiaChans[0].Counterparty.ChannelID,
			gaiaCfg.Denom,
		),
	).IBCDenom()

	var eg errgroup.Group

	// Fund stride user with ibc transfers
	gaiaHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [8]")

	// Fund stride user with ibc denom atom
	tx, err := gaia.SendIBCTransfer(ctx, gaiaChans[0].ChannelID, gaiaUser.KeyName(), ibc.WalletAmount{
		Amount:  1_000_000_000_000,
		Denom:   gaiaCfg.Denom,
		Address: strideAddr,
	}, ibc.TransferOptions{})
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [9]")

	_, err = testutil.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+40, tx.Packet)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [10]")

	require.NoError(t, eg.Wait())

	logger.Info("TestScenarioStrideICAandICQ [11]")

	// Register gaia host zone
	res, err := strideFullNode.ExecTx(ctx, StrideAdminAccount,
		"stakeibc", "register-host-zone",
		gaiaConns[0].Counterparty.ConnectionId, gaiaCfg.Denom, gaiaCfg.Bech32Prefix,
		atomIBCDenom, gaiaChans[0].Counterparty.ChannelID, "1",
		"--gas", "1000000",
	)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [12]", zap.String("execTx res", res))

	gaiaHeight, err = gaia.Height(ctx)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [13]")

	// Wait for the ICA accounts to be setup
	// Poll for 4 MsgChannelOpenConfirm with timeout after 15 blocks.
	chanCount := 0
	_, err = cosmos.PollForMessage(
		ctx, gaia, gaiaCfg.EncodingConfig.InterfaceRegistry, gaiaHeight, gaiaHeight+40,
		func(found *chantypes.MsgChannelOpenConfirm) bool { chanCount++; return chanCount == 4 },
	)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [14]")

	// Get validator address
	gaiaVal1Address, err := gaia.Validators[0].KeyBech32(ctx, "validator", "val")
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [15]")

	// Add gaia validator
	res, err = strideFullNode.ExecTx(ctx, StrideAdminAccount,
		"stakeibc", "add-validator",
		gaiaCfg.ChainID, "gval1", gaiaVal1Address,
		"10", "5",
	)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [16]", zap.String("execTx res", res))

	var gaiaHostZone HostZoneWrapper

	// query gaia host zone
	stdout, _, err := strideFullNode.ExecQuery(ctx,
		"stakeibc", "show-host-zone", gaiaCfg.ChainID,
	)
	require.NoError(t, err)
	err = json.Unmarshal(stdout, &gaiaHostZone)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [17]", zap.String("execQuery res", string(stdout)))

	// Liquid stake some atom
	res, err = strideFullNode.ExecTx(ctx, strideUser.KeyName(),
		"stakeibc", "liquid-stake",
		"1000000000000", gaiaCfg.Denom,
	)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [18]", zap.String("execTx res", res))

	strideHeight, err := stride.Height(ctx)
	require.NoError(t, err)

	logger.Info("TestScenarioStrideICAandICQ [19]")

	// Poll for MsgSubmitQueryResponse with timeout after 20 blocks
	resp, err := cosmos.PollForMessage(
		ctx, stride, strideCfg.EncodingConfig.InterfaceRegistry, strideHeight, strideHeight+40,
		func(found *rlystride.MsgSubmitQueryResponse) bool { return true },
	)

	logger.Info("TestScenarioStrideICAandICQ [20]", zap.String("[poll for msg] resp", resp.String()))
	if err != nil {
		logger.Info("error poll: " + err.Error())
	}
	require.NoError(t, err)
}
