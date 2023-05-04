package interchaintest_test

import (
	"context"
	"fmt"
	"testing"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	relayerinterchaintest "github.com/cosmos/relayer/v2/interchaintest"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func wasmChainSpec(suffix string, nv, nf int) *interchaintest.ChainSpec {
	chainName := "wasm"
	chainID := "wasm" + suffix
	denom := "uand"
	switch suffix {
	case "-1":
		denom += "one"
	case "-2":
		denom += "two"
	default:
		panic("invalid suffix")
	}
	return &interchaintest.ChainSpec{
		Name:          chainName,
		ChainName:     chainID,
		Version:       "v0.40.0-rc.0-ibcx",
		NumValidators: &nv,
		NumFullNodes:  &nf,
		ChainConfig: ibc.ChainConfig{
			Name:    chainName,
			ChainID: chainID,
			Type:    "cosmos",
			Images: []ibc.DockerImage{
				{
					Repository: "ghcr.io/polymerdao/wasm",
					Version:    "v0.40.0-rc.0-ibcx",
					UidGid:     "1025:1025",
				},
			},
			Bin:            "wasmd",
			Bech32Prefix:   "wasm",
			Denom:          denom,
			GasAdjustment:  1.3,
			GasPrices:      "0.0" + denom,
			TrustingPeriod: "336h",
		},
	}
}

func TestWasmBuild(t *testing.T) {
	var (
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		wasmChainSpec("-1", nv, nf),
	})
	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chain := chains[0].(*cosmos.CosmosChain)
	ic := interchaintest.NewInterchain().AddChain(chain)
	client, network := interchaintest.DockerSetup(t)
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})
}

func TestOsmosisBuild(t *testing.T) {
	var (
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "osmosis",
			ChainName:     "osmosis",
			Version:       "v11.0.1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	})
	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chain := chains[0].(*cosmos.CosmosChain)
	ic := interchaintest.NewInterchain().AddChain(chain)
	client, network := interchaintest.DockerSetup(t)
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})
}

func TestRelayerPathWithWasm(t *testing.T) {
	var (
		r    = relayerinterchaintest.NewRelayer(t, relayerinterchaintest.RelayerConfig{})
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		wasmChainSpec("-1", nv, nf),
		{
			Name:          "osmosis",
			ChainName:     "osmosis",
			Version:       "v11.0.1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	wasm, osmosis := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)
	wasmCfg, osmosisCfg := wasm.Config(), osmosis.Config()

	// Build the network; spin up the chains and configure the relayer
	const pathWasmOsmosis = "wasm-osmosis"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(wasm).
		AddChain(osmosis).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  wasm,
			Chain2:  osmosis,
			Relayer: r,
			Path:    pathWasmOsmosis,
		})

	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create clients/connections/channels
	err = r.GeneratePath(ctx, eRep, wasm.Config().ChainID, osmosis.Config().ChainID, pathWasmOsmosis)
	require.NoError(t, err)
	err = r.CreateClients(ctx, eRep, pathWasmOsmosis, ibc.DefaultClientOpts())
	require.NoError(t, err)
	// Wait a few blocks for the clients to be created.
	err = testutil.WaitForBlocks(ctx, 2, wasm, osmosis)
	err = r.CreateConnections(ctx, eRep, pathWasmOsmosis)
	require.NoError(t, err)
	// Wait a few blocks for the connections to be created.
	err = testutil.WaitForBlocks(ctx, 2, wasm, osmosis)
	err = r.CreateChannel(ctx, eRep, pathWasmOsmosis, ibc.DefaultChannelOpts())
	require.NoError(t, err)

	// Start the relayers
	err = r.StartRelayer(ctx, eRep, pathWasmOsmosis)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, wasm, osmosis)
	wasmUser, osmosisUser := users[0].(*cosmos.CosmosWallet), users[1].(*cosmos.CosmosWallet)

	// Wait a few blocks for user accounts to be created on chain.
	err = testutil.WaitForBlocks(ctx, 5, wasm, osmosis)
	require.NoError(t, err)

	wasmAddress := wasmUser.FormattedAddress()
	require.NotEmpty(t, wasmAddress)

	osmosisAddress := osmosisUser.FormattedAddress()
	require.NotEmpty(t, osmosisAddress)

	// get ibc chans
	wasmChans, err := r.GetChannels(ctx, eRep, wasmCfg.ChainID)
	require.NoError(t, err)
	require.Len(t, wasmChans, 1)

	osmosisChans, err := r.GetChannels(ctx, eRep, osmosisCfg.ChainID)
	require.NoError(t, err)
	require.Len(t, osmosisChans, 1)

	wasmIBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(wasmChans[0].Counterparty.PortID, wasmChans[0].Counterparty.ChannelID, wasmCfg.Denom)).IBCDenom()
	osmosisIBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(osmosisChans[0].Counterparty.PortID, osmosisChans[0].Counterparty.ChannelID, osmosisCfg.Denom)).IBCDenom()

	const transferAmount = int64(1_000_000)

	wasmHeight, err := wasm.Height(ctx)
	require.NoError(t, err)

	// Fund osmosis user with ibc denom wasm1
	tx, err := wasm.SendIBCTransfer(ctx, wasmChans[0].ChannelID, wasmUser.KeyName(), ibc.WalletAmount{
		Amount:  transferAmount,
		Denom:   wasmCfg.Denom,
		Address: osmosisAddress,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	_, err = testutil.PollForAck(ctx, wasm, wasmHeight, wasmHeight+10, tx.Packet)
	require.NoError(t, err)

	osmosisHeight, err := osmosis.Height(ctx)
	require.NoError(t, err)

	// Fund wasm user with ibc denom osmosis
	tx, err = osmosis.SendIBCTransfer(ctx, osmosisChans[0].ChannelID, osmosisUser.KeyName(), ibc.WalletAmount{
		Amount:  transferAmount,
		Denom:   osmosisCfg.Denom,
		Address: wasmAddress,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
	require.NoError(t, err)

	wasmOnOsmosisBalance, err := osmosis.GetBalance(ctx, osmosisAddress, wasmIBCDenom)
	require.NoError(t, err)

	require.Equal(t, transferAmount, wasmOnOsmosisBalance)

	osmosisOnWasmBalance, err := wasm.GetBalance(ctx, wasmAddress, osmosisIBCDenom)
	require.NoError(t, err)

	require.Equal(t, transferAmount, osmosisOnWasmBalance)
}

func getClientHeight(t *testing.T, ctx context.Context, chain *cosmos.CosmosChain, height uint64) clienttypes.Height {
	defaultClientID := "07-tendermint-0"
	validator := chain.Validators[0]
	cmd := []string{"ibc", "client", "state", defaultClientID}
	if height > 0 {
		cmd = append(cmd, "--height", fmt.Sprint(height))
	}
	stdout, _, err := validator.ExecQuery(ctx, cmd...)
	require.NoError(t, err)
	queryResp := clienttypes.QueryClientStateResponse{}
	err = defaultEncoding().Codec.UnmarshalJSON(stdout, &queryResp)
	require.NoError(t, err)
	clientState, err := clienttypes.UnpackClientState(queryResp.ClientState)
	require.NoError(t, err)
	clientHeight := clientState.GetLatestHeight().(clienttypes.Height)
	t.Logf("%s height: %d (client height: %s)", chain.Config().ChainID, height, clientHeight.String())
	return clientHeight
}

func TestForcedClientUpdate(t *testing.T) {
	var (
		r    = relayerinterchaintest.NewRelayer(t, relayerinterchaintest.RelayerConfig{})
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		wasmChainSpec("-1", nv, nf),
		{
			Name:          "osmosis",
			ChainName:     "osmosis",
			Version:       "v11.0.1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	wasm1, osmosis := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	// Build the network; spin up the chains and configure the relayer
	const pathWasm1Osmosis = "wasm1-osmosis"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(wasm1).
		AddChain(osmosis).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  wasm1,
			Chain2:  osmosis,
			Relayer: r,
			Path:    pathWasm1Osmosis,
		})
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create clients and connections

	// Single hop wasm1 -> osmosis
	err = r.GeneratePath(ctx, eRep, wasm1.Config().ChainID, osmosis.Config().ChainID, pathWasm1Osmosis)
	require.NoError(t, err)
	err = r.CreateClients(ctx, eRep, pathWasm1Osmosis, ibc.DefaultClientOpts())
	require.NoError(t, err)
	// Wait a few blocks for the clients to be created.
	err = testutil.WaitForBlocks(ctx, 2, wasm1, osmosis)
	require.NoError(t, err)
	err = r.CreateConnections(ctx, eRep, pathWasm1Osmosis)
	require.NoError(t, err)
	// Wait a few blocks for the connections to be created.
	err = testutil.WaitForBlocks(ctx, 2, wasm1, osmosis)

	// Wait for enough blocks for the client on osmosis to be clearly obsolete
	err = testutil.WaitForBlocks(ctx, 10, wasm1)
	require.NoError(t, err)

	wasm1Height, err := wasm1.Height(ctx)
	require.NoError(t, err)

	preUpdateHeight := getClientHeight(t, ctx, osmosis, 0)
	require.Greater(t, wasm1Height, preUpdateHeight.GetRevisionHeight())

	err = r.UpdateClients(ctx, eRep, pathWasm1Osmosis)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, osmosis)

	postUpdateHeight := getClientHeight(t, ctx, osmosis, 0)
	require.True(t, postUpdateHeight.GT(preUpdateHeight))
}

// TestRelayerMultihop sets up this topology to test multihop channels:
// wasm1 		   <-> 			  osmosis 				<-> wasm2
// 07-tendermint-0 <-> 07-tendermint-0, 07-tendermint-1 <-> 07-tendermint-0
// connection-0    <->    connection-0, connection-1    <-> connection-0
//
// Connection hops:
// wasm1: connection-0/connection-1
// wasm2: connection-0/connection-0
func TestRelayerMultihop(t *testing.T) {
	var (
		r    = relayerinterchaintest.NewRelayer(t, relayerinterchaintest.RelayerConfig{})
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		wasmChainSpec("-1", nv, nf),
		{
			Name:          "osmosis",
			ChainName:     "osmosis",
			Version:       "v11.0.1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		wasmChainSpec("-2", nv, nf),
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	wasm1, osmosis, wasm2 := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain), chains[2].(*cosmos.CosmosChain)

	// Build the network; spin up the chains and configure the relayer
	const pathWasm1Osmosis = "wasm1-osmosis"
	const pathOsmosisWasm2 = "osmosis-wasm2"
	const pathWasm1Wasm2 = "wasm1-wasm2"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(wasm1).
		AddChain(osmosis).
		AddChain(wasm2).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  wasm1,
			Chain2:  osmosis,
			Relayer: r,
			Path:    pathWasm1Osmosis,
		}).
		AddLink(interchaintest.InterchainLink{
			Chain1:  osmosis,
			Chain2:  wasm2,
			Relayer: r,
			Path:    pathOsmosisWasm2,
		}).
		AddLink(interchaintest.InterchainLink{
			Chain1:  wasm1,
			Chain2:  wasm2,
			Relayer: r,
			Path:    pathWasm1Wasm2,
		})

	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create clients and connections

	// Single hop wasm1 -> osmosis
	err = r.GeneratePath(ctx, eRep, wasm1.Config().ChainID, osmosis.Config().ChainID, pathWasm1Osmosis)
	require.NoError(t, err)
	err = r.CreateClients(ctx, eRep, pathWasm1Osmosis, ibc.DefaultClientOpts())
	require.NoError(t, err)
	// Wait a few blocks for the clients to be created.
	err = testutil.WaitForBlocks(ctx, 2, wasm1, osmosis)
	require.NoError(t, err)
	err = r.CreateConnections(ctx, eRep, pathWasm1Osmosis)
	require.NoError(t, err)

	// Single hop osmosis -> wasm2
	err = r.GeneratePath(ctx, eRep, osmosis.Config().ChainID, wasm2.Config().ChainID, pathOsmosisWasm2)
	require.NoError(t, err)
	err = r.CreateClients(ctx, eRep, pathOsmosisWasm2, ibc.DefaultClientOpts())
	require.NoError(t, err)
	// Wait a few blocks for the clients to be created.
	err = testutil.WaitForBlocks(ctx, 2, osmosis, wasm2)
	err = r.CreateConnections(ctx, eRep, pathOsmosisWasm2)
	require.NoError(t, err)

	// Multihop wasm1 -> wasm2
	err = r.GeneratePath(ctx, eRep, wasm1.Config().ChainID, wasm2.Config().ChainID, pathWasm1Wasm2,
		osmosis.Config().ChainID)
	require.NoError(t, err)

	// Create multihop channel
	err = r.CreateChannel(ctx, eRep, pathWasm1Wasm2, ibc.DefaultChannelOpts())
	require.NoError(t, err)

	// Wait a few blocks for the channel to be created.
	err = testutil.WaitForBlocks(ctx, 2, osmosis, wasm2)
	require.NoError(t, err)

	wasm1Cfg, osmosisCfg, wasm2Cfg := wasm1.Config(), osmosis.Config(), wasm2.Config()

	wasm1Chans, err := r.GetChannels(ctx, eRep, wasm1Cfg.ChainID)
	require.NoError(t, err)
	require.Len(t, wasm1Chans, 1)
	require.Equal(t, "STATE_OPEN", wasm1Chans[0].State)

	wasm2Chans, err := r.GetChannels(ctx, eRep, wasm2Cfg.ChainID)
	require.NoError(t, err)
	require.Len(t, wasm2Chans, 1)
	require.Equal(t, "STATE_OPEN", wasm2Chans[0].State)

	osmosisChans, err := r.GetChannels(ctx, eRep, osmosisCfg.ChainID)
	require.NoError(t, err)
	require.Len(t, osmosisChans, 0)

	/* TODO: enable
	// Start the relayers
	err = r.StartRelayer(ctx, eRep, pathWasm1Osmosis, pathOsmosisWasm2, pathWasm1Wasm2)
	require.NoError(t, err)
	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Wait a few blocks for the relayer to start.
	err = testutil.WaitForBlocks(ctx, 2, wasm1, osmosis, wasm2)
	require.NoError(t, err)

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, wasm1, osmosis, wasm2)
	wasm1User, osmosisUser, wasm2User := users[0].(*cosmos.CosmosWallet), users[1].(*cosmos.CosmosWallet), users[2].(*cosmos.CosmosWallet)

	// Wait a few blocks for user accounts to be created on chain.
	err = testutil.WaitForBlocks(ctx, 5, wasm1, osmosis, wasm2)
	require.NoError(t, err)

	wasm1Address := wasm1User.FormattedAddress()
	require.NotEmpty(t, wasm1Address)

	osmosisAddress := osmosisUser.FormattedAddress()
	require.NotEmpty(t, osmosisAddress)

	wasm2Address := wasm2User.FormattedAddress()
	require.NotEmpty(t, wasm2Address)

	wasm1IBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(wasm1Chans[0].Counterparty.PortID, wasm1Chans[0].Counterparty.ChannelID, wasm1Cfg.Denom)).IBCDenom()
	wasm2IBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(wasm2Chans[0].Counterparty.PortID, wasm2Chans[0].Counterparty.ChannelID, wasm2Cfg.Denom)).IBCDenom()

	const transferAmount = int64(1_000_000)

	wasm1Height, err := wasm1.Height(ctx)
	require.NoError(t, err)
	// Fund wasm2 user with ibc denom from wasm1
	t.Logf("Initiating transfer from %s to %s", wasm1.Config().ChainID, wasm2.Config().ChainID)
	tx, err := wasm1.SendIBCTransfer(ctx, wasm1Chans[0].ChannelID, wasm1User.KeyName(), ibc.WalletAmount{
		Amount:  transferAmount,
		Denom:   wasm1Cfg.Denom,
		Address: wasm2Address,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	_, err = testutil.PollForAck(ctx, wasm1, wasm1Height, wasm1Height+10, tx.Packet)
	require.NoError(t, err)

	wasm2Height, err := wasm2.Height(ctx)
	require.NoError(t, err)
	// Fund wasm1 user with ibc denom from wasm2
	t.Logf("Initiating transfer from %s to %s", wasm1.Config().ChainID, wasm2.Config().ChainID)
	tx, err = wasm2.SendIBCTransfer(ctx, wasm2Chans[0].ChannelID, wasm2User.KeyName(), ibc.WalletAmount{
		Amount:  transferAmount,
		Denom:   wasm2Cfg.Denom,
		Address: wasm1Address,
	}, ibc.TransferOptions{})
	require.NoError(t, err)
	_, err = testutil.PollForAck(ctx, wasm2, wasm2Height, wasm2Height+10, tx.Packet)
	require.NoError(t, err)

	wasm1OnWasm2Balance, err := wasm2.GetBalance(ctx, wasm2Address, wasm1IBCDenom)
	require.NoError(t, err)
	require.Equal(t, transferAmount, wasm1OnWasm2Balance)

	wasm2OnWasm1Balance, err := wasm1.GetBalance(ctx, wasm1Address, wasm2IBCDenom)
	require.NoError(t, err)
	require.Equal(t, transferAmount, wasm2OnWasm1Balance)

	*/
}
