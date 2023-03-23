package interchaintest

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/go-bip39"
	ibcgo "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/processor"
	ibctestv5 "github.com/strangelove-ventures/interchaintest/v7"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

// protoTxProvider is a type which can provide a proto transaction. It is a
// workaround to get access to the wrapper TxBuilder's method GetProtoTx().
type protoTxProvider interface {
	GetProtoTx() *txtypes.Tx
}

type chainFeegrantInfo struct {
	granter    string
	grantees   []string
	lastSigner string //whichever grantee signed the last known TX on chain
}

func nextGrantee(gi *chainFeegrantInfo) string {
	for i, grantee := range gi.grantees {
		if gi.lastSigner == grantee && i < len(gi.grantees)-1 {
			return gi.grantees[i+1]
		} else if gi.lastSigner == grantee {
			return gi.grantees[0]
		}
	}

	return gi.grantees[0]
}

func genMnemonic(t *testing.T) string {
	// read entropy seed straight from tmcrypto.Rand and convert to mnemonic
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		t.Fail()
	}

	mn, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		t.Fail()
	}

	return mn
}

// TestScenarioFeegrantBasic Feegrant on a single chain
// Run this test with e.g. go test -timeout 300s -run ^TestScenarioFeegrantBasic$ github.com/cosmos/relayer/v2/ibctest.
//
// Helpful to debug:
// docker ps -a --format {{.Names}} then e.g. docker logs gaia-1-val-0-TestScenarioFeegrantBasic 2>&1 -f
func TestScenarioFeegrantBasic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	nv := 1
	nf := 0

	//In order to have this image locally you'd need to build it with heighliner, e.g.,
	//from within the local "gaia" directory, run the following command:
	//../heighliner/heighliner build -c gaia --local -f ../heighliner/chains.yaml
	// gaiaImage := ibc.DockerImage{
	// 	Repository: "gaia",
	// 	Version:    "local",
	// 	UidGid:     "1025:1025", //the heighliner user string. this isn't exposed on ibctest
	// }

	// gaiaChainSpec := &ibctestv5.ChainSpec{
	// 	ChainName:     "gaia",
	// 	NumValidators: &nv,
	// 	NumFullNodes:  &nf,
	// 	ChainConfig: ibc.ChainConfig{
	// 		Type: "cosmos",
	// 		Name: "gaia",
	// 		//ChainID:        "gaia-1", //I believe this will be auto-generated?
	// 		Images:         []ibc.DockerImage{gaiaImage},
	// 		Bin:            "gaiad",
	// 		Bech32Prefix:   "cosmos",
	// 		Denom:          "uatom",
	// 		GasPrices:      "0.01uatom",
	// 		TrustingPeriod: "504h",
	// 		GasAdjustment:  1.3,
	// 	}}

	// Chain Factory
	cf := ibctestv5.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctestv5.ChainSpec{
		{Name: "gaia", ChainName: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
		//gaiaChainSpec,
		{Name: "osmosis", ChainName: "osmosis", Version: "v14.0.1", NumValidators: &nv, NumFullNodes: &nf},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// Relayer Factory to construct relayer
	r := NewRelayerFactory(RelayerConfig{
		Processor:           relayer.ProcessorEvents,
		InitialBlockHistory: 100,
	}).Build(t, nil, "")

	processor.PathProcMessageCollector = make(chan *processor.PathProcessorMessageResp, 50)

	// Prep Interchain
	const ibcPath = "gaia-osmosis"
	ic := ibctestv5.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(ibctestv5.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    ibcPath,
		})

	// Reporter/logs
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	client, network := ibctestv5.DockerSetup(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctestv5.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))

	// Get Channel ID
	gaiaChans, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannel := gaiaChans[0]
	osmosisChannel := gaiaChans[0].Counterparty

	// Create and Fund User Wallets
	fundAmount := int64(10_000_000)

	// Tiny amount of funding, not enough to pay for a single TX fee (the GRANTER should be paying the fee)
	granteeFundAmount := int64(10)
	granteeKeyPrefix := "grantee1"
	grantee2KeyPrefix := "grantee2"
	grantee3KeyPrefix := "grantee3"
	granterKeyPrefix := "default"
	coinType := "118"

	mnemonicAny := genMnemonic(t)
	gaiaGranterWallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, granterKeyPrefix, mnemonicAny, int64(fundAmount), gaia)
	require.NoError(t, err)

	mnemonicAny = genMnemonic(t)
	gaiaGranteeWallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, granteeKeyPrefix, mnemonicAny, int64(granteeFundAmount), gaia)
	require.NoError(t, err)

	mnemonicAny = genMnemonic(t)
	gaiaGrantee2Wallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, grantee2KeyPrefix, mnemonicAny, int64(granteeFundAmount), gaia)
	require.NoError(t, err)

	mnemonicAny = genMnemonic(t)
	gaiaGrantee3Wallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, grantee3KeyPrefix, mnemonicAny, int64(granteeFundAmount), gaia)
	require.NoError(t, err)

	mnemonicAny = genMnemonic(t)
	osmosisUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "recipient", mnemonicAny, int64(fundAmount), osmosis)
	require.NoError(t, err)

	mnemonicAny = genMnemonic(t)
	gaiaUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "recipient", mnemonicAny, int64(fundAmount), gaia)
	require.NoError(t, err)

	mnemonic := gaiaGranterWallet.Mnemonic()
	fmt.Printf("Wallet mnemonic: %s\n", mnemonic)

	rand.Seed(time.Now().UnixNano())

	//IBC chain config is unrelated to RELAYER config so this step is necessary
	if err := r.RestoreKey(ctx,
		eRep,
		gaia.Config().ChainID,
		gaiaGranterWallet.KeyName(),
		coinType,
		gaiaGranterWallet.Mnemonic(),
	); err != nil {
		t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
	}

	//IBC chain config is unrelated to RELAYER config so this step is necessary
	if err := r.RestoreKey(ctx,
		eRep,
		gaia.Config().ChainID,
		gaiaGranteeWallet.KeyName(),
		coinType,
		gaiaGranteeWallet.Mnemonic(),
	); err != nil {
		t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
	}

	//IBC chain config is unrelated to RELAYER config so this step is necessary
	if err := r.RestoreKey(ctx,
		eRep,
		gaia.Config().ChainID,
		gaiaGrantee2Wallet.KeyName(),
		coinType,
		gaiaGrantee2Wallet.Mnemonic(),
	); err != nil {
		t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
	}

	//IBC chain config is unrelated to RELAYER config so this step is necessary
	if err := r.RestoreKey(ctx,
		eRep,
		gaia.Config().ChainID,
		gaiaGrantee3Wallet.KeyName(),
		coinType,
		gaiaGrantee3Wallet.Mnemonic(),
	); err != nil {
		t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
	}

	//IBC chain config is unrelated to RELAYER config so this step is necessary
	if err := r.RestoreKey(ctx,
		eRep,
		osmosis.Config().ChainID,
		osmosisUser.KeyName(),
		coinType,
		osmosisUser.Mnemonic(),
	); err != nil {
		t.Fatalf("failed to restore granter key to relayer for chain %s: %s", osmosis.Config().ChainID, err.Error())
	}

	//IBC chain config is unrelated to RELAYER config so this step is necessary
	if err := r.RestoreKey(ctx,
		eRep,
		osmosis.Config().ChainID,
		gaiaUser.KeyName(),
		coinType,
		gaiaUser.Mnemonic(),
	); err != nil {
		t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
	}

	gaiaGranteeAddr := gaiaGranteeWallet.FormattedAddress()
	gaiaGrantee2Addr := gaiaGrantee2Wallet.FormattedAddress()
	gaiaGrantee3Addr := gaiaGrantee3Wallet.FormattedAddress()
	gaiaGranterAddr := gaiaGranterWallet.FormattedAddress()

	granteeCsv := gaiaGranteeWallet.KeyName() + "," + gaiaGrantee2Wallet.KeyName() + "," + gaiaGrantee3Wallet.KeyName()

	//You MUST run the configure feegrant command prior to starting the relayer, otherwise it'd be like you never set it up at all (within this test)
	//Note that Gaia supports feegrants, but Osmosis does not (x/feegrant module, or any compatible module, is not included in Osmosis SDK app modules)
	localRelayer := r.(*Relayer)
	res := localRelayer.sys().Run(logger, "chains", "configure", "feegrant", "basicallowance", gaia.Config().ChainID, gaiaGranterWallet.KeyName(), "--grantees", granteeCsv, "--overwrite-granter")
	if res.Err != nil {
		fmt.Printf("configure feegrant results: %s\n", res.Stdout.String())
		t.Fatalf("failed to rly config feegrants: %v", res.Err)
	}

	//Map of feegranted chains and the feegrant info for the chain
	feegrantedChains := map[string]*chainFeegrantInfo{}
	feegrantedChains[gaia.Config().ChainID] = &chainFeegrantInfo{granter: gaiaGranterAddr, grantees: []string{gaiaGranteeAddr, gaiaGrantee2Addr, gaiaGrantee3Addr}}

	time.Sleep(7 * time.Second) //commit a block
	r.StartRelayer(ctx, eRep, ibcPath)

	// Send Transaction
	amountToSend := int64(1_000)

	gaiaDstAddress := types.MustBech32ifyAddressBytes(osmosis.Config().Bech32Prefix, gaiaUser.Address())
	osmosisDstAddress := types.MustBech32ifyAddressBytes(gaia.Config().Bech32Prefix, osmosisUser.Address())

	gaiaHeight, err := gaia.Height(ctx)
	require.NoError(t, err)

	osmosisHeight, err := osmosis.Height(ctx)
	require.NoError(t, err)

	var eg errgroup.Group
	var gaiaTx ibc.Tx

	eg.Go(func() error {
		gaiaTx, err = gaia.SendIBCTransfer(ctx, gaiaChannel.ChannelID, gaiaUser.KeyName(), ibc.WalletAmount{
			Address: gaiaDstAddress,
			Denom:   gaia.Config().Denom,
			Amount:  amountToSend,
		},
			ibc.TransferOptions{},
		)
		if err != nil {
			return err
		}
		if err := gaiaTx.Validate(); err != nil {
			return err
		}

		_, err = testutil.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+10, gaiaTx.Packet)
		return err
	})

	eg.Go(func() error {
		tx, err := osmosis.SendIBCTransfer(ctx, osmosisChannel.ChannelID, osmosisUser.KeyName(), ibc.WalletAmount{
			Address: osmosisDstAddress,
			Denom:   osmosis.Config().Denom,
			Amount:  amountToSend,
		},
			ibc.TransferOptions{},
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
		return err
	})

	eg.Go(func() error {
		tx, err := osmosis.SendIBCTransfer(ctx, osmosisChannel.ChannelID, osmosisUser.KeyName(), ibc.WalletAmount{
			Address: osmosisDstAddress,
			Denom:   osmosis.Config().Denom,
			Amount:  amountToSend,
		},
			ibc.TransferOptions{},
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
		return err
	})

	eg.Go(func() error {
		tx, err := osmosis.SendIBCTransfer(ctx, osmosisChannel.ChannelID, osmosisUser.KeyName(), ibc.WalletAmount{
			Address: osmosisDstAddress,
			Denom:   osmosis.Config().Denom,
			Amount:  amountToSend,
		},
			ibc.TransferOptions{},
		)
		if err != nil {
			return err
		}
		if err := tx.Validate(); err != nil {
			return err
		}
		_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+10, tx.Packet)
		return err
	})

	require.NoError(t, err)
	require.NoError(t, eg.Wait())

	r.StopRelayer(ctx, eRep)
	feegrantMsgSigners := map[string][]string{} //chain to list of signers

	for len(processor.PathProcMessageCollector) > 0 {
		select {
		case curr, ok := <-processor.PathProcMessageCollector:
			if ok && curr.Error == nil && curr.SuccessfulTx {
				cProv := curr.DestinationChain.(*cosmos.CosmosProvider)
				chain := cProv.PCfg.ChainID
				feegrantInfo, isFeegrantedChain := feegrantedChains[chain]

				sdkConf := sdk.GetConfig()
				sdkConf.SetBech32PrefixForAccount(cProv.PCfg.AccountPrefix, cProv.PCfg.AccountPrefix+"pub")
				sdkConf.SetBech32PrefixForValidator(cProv.PCfg.AccountPrefix+"valoper", cProv.PCfg.AccountPrefix+"valoperpub")
				sdkConf.SetBech32PrefixForConsensusNode(cProv.PCfg.AccountPrefix+"valcons", cProv.PCfg.AccountPrefix+"valconspub")

				hash, err := hex.DecodeString(curr.Response.TxHash)
				require.Nil(t, err)
				txResp, err := cProv.RPCClient.Tx(ctx, hash, true)
				require.Nil(t, err)

				require.Nil(t, err)
				dc := cProv.Cdc.TxConfig.TxDecoder()
				tx, err := dc(txResp.Tx)
				require.Nil(t, err)
				builder, err := cProv.Cdc.TxConfig.WrapTxBuilder(tx)
				require.Nil(t, err)
				txFinder := builder.(protoTxProvider)
				fullTx := txFinder.GetProtoTx()
				isFeegrantedMsg := false

				msgType := ""
				for _, m := range fullTx.GetMsgs() {
					msgType = types.MsgTypeURL(m)
					//We want all IBC transfers (on an open channel/connection) to be feegranted in round robin fashion
					if msgType == "/ibc.core.channel.v1.MsgRecvPacket" || msgType == "/ibc.core.channel.v1.MsgAcknowledgement" {
						isFeegrantedMsg = true
						break
					}
				}

				//It's required that TXs be feegranted in a round robin fashion for this chain and message type
				if isFeegrantedChain && isFeegrantedMsg {
					signers := fullTx.GetSigners()
					require.Equal(t, len(signers), 1)
					granter := fullTx.FeeGranter()

					//Feegranter for the TX that was signed on chain must be the relayer chain's configured feegranter
					require.Equal(t, granter.String(), feegrantInfo.granter)
					require.NotEmpty(t, granter)

					for _, msg := range fullTx.GetMsgs() {
						msgType = types.MsgTypeURL(msg)
						//We want all IBC transfers (on an open channel/connection) to be feegranted in round robin fashion
						if msgType == "/ibc.core.channel.v1.MsgRecvPacket" {
							c := msg.(*chantypes.MsgRecvPacket)
							appData := c.Packet.GetData()
							tokenTransfer := &ibcgo.FungibleTokenPacketData{}
							err := tokenTransfer.Unmarshal(appData)
							if err == nil {
								fmt.Printf("%+v\n", tokenTransfer)
							} else {
								fmt.Println(string(appData))
							}
						}
					}

					//Grantee for the TX that was signed on chain must be a configured grantee in the relayer's chain feegrants.
					//In addition, the grantee must be used in round robin fashion
					//expectedGrantee := nextGrantee(feegrantInfo)
					actualGrantee := signers[0].String()
					signerList, ok := feegrantMsgSigners[chain]
					if ok {
						signerList = append(signerList, actualGrantee)
						feegrantMsgSigners[chain] = signerList
					} else {
						feegrantMsgSigners[chain] = []string{actualGrantee}
					}
					fmt.Printf("Chain: %s, msg type: %s, height: %d, signer: %s, granter: %s\n", chain, msgType, curr.Response.Height, actualGrantee, granter.String())
					//require.Equal(t, expectedGrantee, actualGrantee)
					feegrantInfo.lastSigner = actualGrantee
				}
			}
		default:
			fmt.Println("Unknown channel message")
		}
	}

	for chain, signers := range feegrantMsgSigners {
		require.Equal(t, chain, "gaia-1")
		signerCountMap := map[string]int{}

		for _, signer := range signers {
			count, ok := signerCountMap[signer]
			if ok {
				signerCountMap[signer] = count + 1
			} else {
				signerCountMap[signer] = 1
			}
		}

		highestCount := 0
		for _, count := range signerCountMap {
			if count > highestCount {
				highestCount = count
			}
		}

		//At least one feegranter must have signed a TX
		require.GreaterOrEqual(t, highestCount, 1)

		//All of the feegrantees must have signed at least one TX
		expectedFeegrantInfo := feegrantedChains[chain]
		require.Equal(t, len(signerCountMap), len(expectedFeegrantInfo.grantees))

		// verify that TXs were signed in a round robin fashion.
		// no grantee should have signed more TXs than any other grantee (off by one is allowed).
		for signer, count := range signerCountMap {
			fmt.Printf("signer %s signed %d feegranted TXs \n", signer, count)
			require.LessOrEqual(t, highestCount-count, 1)
		}
	}

	// Trace IBC Denom
	gaiaDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(osmosisChannel.PortID, osmosisChannel.ChannelID, gaia.Config().Denom))
	gaiaIbcDenom := gaiaDenomTrace.IBCDenom()

	osmosisDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(gaiaChannel.PortID, gaiaChannel.ChannelID, osmosis.Config().Denom))
	osmosisIbcDenom := osmosisDenomTrace.IBCDenom()

	// Test destination wallets have increased funds
	gaiaIBCBalance, err := osmosis.GetBalance(ctx, gaiaDstAddress, gaiaIbcDenom)
	require.NoError(t, err)
	require.Equal(t, amountToSend, gaiaIBCBalance)

	osmosisIBCBalance, err := gaia.GetBalance(ctx, osmosisDstAddress, osmosisIbcDenom)
	require.NoError(t, err)
	require.Equal(t, 3*amountToSend, osmosisIBCBalance)

	// Test grantee still has exact amount expected
	gaiaGranteeIBCBalance, err := gaia.GetBalance(ctx, gaiaGranteeAddr, gaia.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, granteeFundAmount, gaiaGranteeIBCBalance)

	// Test granter has less than they started with, meaning fees came from their account
	gaiaGranterIBCBalance, err := gaia.GetBalance(ctx, gaiaGranterAddr, gaia.Config().Denom)
	require.NoError(t, err)
	require.Less(t, gaiaGranterIBCBalance, fundAmount)
}
