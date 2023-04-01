package interchaintest_test

import (
	"context"
	"testing"

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

	// TODO: create connections but not channels
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

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

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
	const relayerName = "relayer"

	// TODO: create connections but not channels
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
		})

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

	// Start the relayers
	err = r.StartRelayer(ctx, eRep, pathWasm1Osmosis, pathOsmosisWasm2)
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

	// TODO: create multihop channel
}
