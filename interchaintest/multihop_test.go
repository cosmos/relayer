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
	"golang.org/x/sync/errgroup"
)

func TestWasmBuild(t *testing.T) {
	var (
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	chainConfig := ibc.ChainConfig{
		Name:    "wasm-1",
		ChainID: "wasm-1",
		Type:    "cosmos",
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/polymerdao/wasm",
				Version:    "v0.40.0-rc.0-ibcx",
			},
		},
		Bin:            "wasmd",
		Bech32Prefix:   "wasm",
		Denom:          "umlg",
		GasPrices:      "0.0umlg",
		TrustingPeriod: "336h",
	}
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          chainConfig.Name,
			ChainName:     chainConfig.Name,
			Version:       "v0.40.0-rc.0-ibcx",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig:   chainConfig,
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

func TestRelayerMultihop(t *testing.T) {
	var (
		r    = relayerinterchaintest.NewRelayer(t, relayerinterchaintest.RelayerConfig{})
		rep  = testreporter.NewNopReporter()
		eRep = rep.RelayerExecReporter(t)
		ctx  = context.Background()
		nv   = 1
		nf   = 0
	)
	wasmChainConfig := ibc.ChainConfig{
		Type: "cosmos",
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/polymerdao/wasm",
				Version:    "v0.40.0-rc.0-ibcx",
			},
		},
		Bin:            "wasmd",
		Bech32Prefix:   "wasm",
		Denom:          "umlg",
		GasPrices:      "0.0umlg",
		TrustingPeriod: "336h",
	}
	wasm1Config := wasmChainConfig
	wasm1Config.ChainID = "wasm-1"
	wasm1Config.Name = "wasm-1"
	wasm2Config := wasmChainConfig
	wasm2Config.ChainID = "wasm-2"
	wasm2Config.Name = "wasm-2"
	require.NotEqual(t, wasm1Config.ChainID, wasm2Config.ChainID)
	// Define chains involved in test
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          wasm1Config.Name,
			ChainName:     wasm1Config.Name,
			Version:       "v0.40.0-rc.0-ibcx",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig:   wasm1Config,
		},
		{
			Name:          "osmosis",
			ChainName:     "osmosis",
			Version:       "v11.0.1",
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			Name:          wasm2Config.Name,
			ChainName:     wasm2Config.Name,
			Version:       "v0.40.0-rc.0-ibcx",
			NumValidators: &nv,
			NumFullNodes:  &nf,
			ChainConfig:   wasm2Config,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	wasm1, osmosis, wasm2 := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain), chains[2].(*cosmos.CosmosChain)
	wasm1Cfg, osmosisCfg, wasm2Cfg := wasm1.Config(), osmosis.Config(), wasm2.Config()

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

	// get ibc chans
	wasm1Chans, err := r.GetChannels(ctx, eRep, wasm1Cfg.ChainID)
	require.NoError(t, err)
	require.Len(t, wasm1Chans, 1)

	osmosisChans, err := r.GetChannels(ctx, eRep, osmosisCfg.ChainID)
	require.NoError(t, err)
	require.Len(t, osmosisChans, 0)

	wasm2Chans, err := r.GetChannels(ctx, eRep, wasm2Cfg.ChainID)
	require.NoError(t, err)
	require.Len(t, wasm2Chans, 1)

	wasm1IBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(wasm1Chans[0].Counterparty.PortID, wasm1Chans[0].Counterparty.ChannelID, wasm1Cfg.Denom)).IBCDenom()
	wasm2IBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(wasm2Chans[0].Counterparty.PortID, wasm2Chans[0].Counterparty.ChannelID, wasm2Cfg.Denom)).IBCDenom()

	var eg errgroup.Group

	const transferAmount = int64(1_000_000)

	eg.Go(func() error {
		wasm1Height, err := wasm1.Height(ctx)
		if err != nil {
			return err
		}
		// Fund gaia user with ibc denom osmo
		tx, err := wasm1.SendIBCTransfer(ctx, wasm1Chans[0].ChannelID, wasm2User.KeyName(), ibc.WalletAmount{
			Amount:  transferAmount,
			Denom:   wasm1Cfg.Denom,
			Address: wasm2Address,
		}, ibc.TransferOptions{})
		if err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, osmosis, wasm1Height, wasm1Height+10, tx.Packet)
		return err
	})

	eg.Go(func() error {
		wasm2Height, err := wasm2.Height(ctx)
		if err != nil {
			return err
		}
		// Fund gaia user with ibc denom juno
		tx, err := wasm2.SendIBCTransfer(ctx, wasm2Chans[0].ChannelID, wasm2User.KeyName(), ibc.WalletAmount{
			Amount:  transferAmount,
			Denom:   wasm2Cfg.Denom,
			Address: wasm1Address,
		}, ibc.TransferOptions{})
		if err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, wasm2, wasm2Height, wasm2Height+10, tx.Packet)
		return err
	})

	require.NoError(t, eg.Wait())

	wasm1OnWasm2Balance, err := wasm2.GetBalance(ctx, wasm2Address, wasm1IBCDenom)
	require.NoError(t, err)

	require.Equal(t, transferAmount, wasm1OnWasm2Balance)

	wasm2OnWasm1Balance, err := wasm1.GetBalance(ctx, wasm1Address, wasm2IBCDenom)
	require.NoError(t, err)

	require.Equal(t, transferAmount, wasm2OnWasm1Balance)
}
