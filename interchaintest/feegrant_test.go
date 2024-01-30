package interchaintest

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"
	"github.com/avast/retry-go/v4"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/go-bip39"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/client"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/strangelove-ventures/interchaintest/v8"
	cosmosv8 "github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
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
	granter  string
	grantees []string
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

// TestRelayerFeeGrant Feegrant on a single chain
// Run this test with e.g. go test -timeout 300s -run ^TestRelayerFeeGrant$ github.com/cosmos/relayer/v2/ibctest.
//
// Helpful to debug:
// docker ps -a --format {{.Names}} then e.g. docker logs gaia-1-val-0-TestRelayerFeeGrant 2>&1 -f
func TestRelayerFeeGrant(t *testing.T) {
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

	// gaiaChainSpec := &interchaintest.ChainSpec{
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

	var tests = [][]*interchaintest.ChainSpec{
		{
			{Name: "gaia", ChainName: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
			{Name: "osmosis", ChainName: "osmosis", Version: "v14.0.1", NumValidators: &nv, NumFullNodes: &nf},
		},
		{
			{Name: "gaia", ChainName: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
			{Name: "kujira", ChainName: "kujira", Version: "v0.8.7", NumValidators: &nv, NumFullNodes: &nf},
		},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s,%s", tt[0].Name, tt[1].Name)
		t.Run(testname, func(t *testing.T) {

			// Chain Factory
			cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), tt)

			chains, err := cf.Chains(t.Name())
			require.NoError(t, err)
			gaia, osmosis := chains[0], chains[1]

			// Relayer Factory to construct relayer
			r := NewRelayerFactory(RelayerConfig{
				Processor:           relayer.ProcessorEvents,
				InitialBlockHistory: 100,
			}).Build(t, nil, "")

			processor.PathProcMessageCollector = make(chan *processor.PathProcessorMessageResp, 10000)

			// Prep Interchain
			const ibcPath = "gaia-osmosis"
			ic := interchaintest.NewInterchain().
				AddChain(gaia).
				AddChain(osmosis).
				AddRelayer(r, "relayer").
				AddLink(interchaintest.InterchainLink{
					Chain1:  gaia,
					Chain2:  osmosis,
					Relayer: r,
					Path:    ibcPath,
				})

			// Reporter/logs
			rep := testreporter.NewNopReporter()
			eRep := rep.RelayerExecReporter(t)

			client, network := interchaintest.DockerSetup(t)

			// Build interchain
			require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
				TestName:  t.Name(),
				Client:    client,
				NetworkID: network,

				SkipPathCreation: false,
			}))

			t.Parallel()

			// Get Channel ID
			gaiaChans, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
			require.NoError(t, err)
			gaiaChannel := gaiaChans[0]
			osmosisChannel := gaiaChans[0].Counterparty

			// Create and Fund User Wallets
			fundAmount := sdkmath.NewInt(10_000_000)

			// Tiny amount of funding, not enough to pay for a single TX fee (the GRANTER should be paying the fee)
			granteeFundAmount := sdkmath.NewInt(10)
			granteeKeyPrefix := "grantee1"
			grantee2KeyPrefix := "grantee2"
			grantee3KeyPrefix := "grantee3"
			granterKeyPrefix := "default"

			mnemonicAny := genMnemonic(t)
			gaiaGranterWallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, granterKeyPrefix, mnemonicAny, fundAmount, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaGranteeWallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, granteeKeyPrefix, mnemonicAny, granteeFundAmount, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaGrantee2Wallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, grantee2KeyPrefix, mnemonicAny, granteeFundAmount, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaGrantee3Wallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, grantee3KeyPrefix, mnemonicAny, granteeFundAmount, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			osmosisUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "recipient", mnemonicAny, fundAmount, osmosis)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "recipient", mnemonicAny, fundAmount, gaia)
			require.NoError(t, err)

			mnemonic := gaiaGranterWallet.Mnemonic()
			fmt.Printf("Wallet mnemonic: %s\n", mnemonic)

			rand.Seed(time.Now().UnixNano())

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGranterWallet.KeyName(),
				gaiaGranterWallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGranteeWallet.KeyName(),
				gaiaGranteeWallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGrantee2Wallet.KeyName(),
				gaiaGrantee2Wallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGrantee3Wallet.KeyName(),
				gaiaGrantee3Wallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				osmosis.Config(),
				osmosisUser.KeyName(),
				osmosisUser.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", osmosis.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				osmosis.Config(),
				gaiaUser.KeyName(),
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
			res := localRelayer.Sys().Run(logger, "chains", "configure", "feegrant", "basicallowance", gaia.Config().ChainID, gaiaGranterWallet.KeyName(), "--grantees", granteeCsv, "--overwrite-granter")
			if res.Err != nil {
				fmt.Printf("configure feegrant results: %s\n", res.Stdout.String())
				t.Fatalf("failed to rly config feegrants: %v", res.Err)
			}

			//Map of feegranted chains and the feegrant info for the chain
			feegrantedChains := map[string]*chainFeegrantInfo{}
			feegrantedChains[gaia.Config().ChainID] = &chainFeegrantInfo{granter: gaiaGranterAddr, grantees: []string{gaiaGranteeAddr, gaiaGrantee2Addr, gaiaGrantee3Addr}}

			time.Sleep(14 * time.Second) //commit a couple blocks
			r.StartRelayer(ctx, eRep, ibcPath)

			// Send Transaction
			amountToSend := sdkmath.NewInt(1_000)

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

				_, err = testutil.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+20, gaiaTx.Packet)
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
				_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+20, tx.Packet)
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
				_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+20, tx.Packet)
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
				_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+20, tx.Packet)
				return err
			})

			require.NoError(t, err)
			require.NoError(t, eg.Wait())

			feegrantMsgSigners := map[string][]string{} //chain to list of signers

			for len(processor.PathProcMessageCollector) > 0 {
				select {
				case curr, ok := <-processor.PathProcMessageCollector:
					if ok && curr.Error == nil && curr.SuccessfulTx {
						cProv, cosmProv := curr.DestinationChain.(*cosmos.CosmosProvider)
						if cosmProv {
							chain := cProv.PCfg.ChainID
							feegrantInfo, isFeegrantedChain := feegrantedChains[chain]
							if isFeegrantedChain && !strings.Contains(cProv.PCfg.KeyDirectory, t.Name()) {
								//This would indicate that a parallel test is inserting msgs into the queue.
								//We can safely skip over any messages inserted by other test cases.
								fmt.Println("Skipping PathProcessorMessageResp from unrelated Parallel test case")
								continue
							}

							done := cProv.SetSDKContext()

							hash, err := hex.DecodeString(curr.Response.TxHash)
							require.Nil(t, err)
							txResp, err := TxWithRetry(ctx, cProv.RPCClient, hash)
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

							msgs := ""
							msgType := ""
							for _, m := range fullTx.GetMsgs() {
								msgType = types.MsgTypeURL(m)
								//We want all IBC transfers (on an open channel/connection) to be feegranted in round robin fashion
								if msgType == "/ibc.core.channel.v1.MsgRecvPacket" || msgType == "/ibc.core.channel.v1.MsgAcknowledgement" {
									isFeegrantedMsg = true
									msgs += msgType + ", "
								} else {
									msgs += msgType + ", "
								}
							}

							//It's required that TXs be feegranted in a round robin fashion for this chain and message type
							if isFeegrantedChain && isFeegrantedMsg {
								fmt.Printf("Msg types: %+v\n", msgs)

								signers, _, err := cProv.Cdc.Marshaler.GetMsgV1Signers(fullTx)
								require.NoError(t, err)

								require.Equal(t, len(signers), 1)
								granter := fullTx.FeeGranter(cProv.Cdc.Marshaler)

								//Feegranter for the TX that was signed on chain must be the relayer chain's configured feegranter
								require.Equal(t, feegrantInfo.granter, string(granter))
								require.NotEmpty(t, granter)

								for _, msg := range fullTx.GetMsgs() {
									msgType = types.MsgTypeURL(msg)
									//We want all IBC transfers (on an open channel/connection) to be feegranted in round robin fashion
									if msgType == "/ibc.core.channel.v1.MsgRecvPacket" {
										c := msg.(*chantypes.MsgRecvPacket)
										appData := c.Packet.GetData()
										tokenTransfer := &transfertypes.FungibleTokenPacketData{}
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
								actualGrantee := string(signers[0])
								signerList, ok := feegrantMsgSigners[chain]
								if ok {
									signerList = append(signerList, actualGrantee)
									feegrantMsgSigners[chain] = signerList
								} else {
									feegrantMsgSigners[chain] = []string{actualGrantee}
								}
								fmt.Printf("Chain: %s, msg type: %s, height: %d, signer: %s, granter: %s\n", chain, msgType, curr.Response.Height, actualGrantee, string(granter))
							}
							done()
						}
					}
				default:
					fmt.Println("Unknown channel message")
				}
			}

			for chain, signers := range feegrantMsgSigners {
				require.Equal(t, chain, gaia.Config().ChainID)
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
			require.True(t, amountToSend.Equal(gaiaIBCBalance))

			osmosisIBCBalance, err := gaia.GetBalance(ctx, osmosisDstAddress, osmosisIbcDenom)
			require.NoError(t, err)
			require.True(t, amountToSend.MulRaw(3).Equal(osmosisIBCBalance))

			// Test grantee still has exact amount expected
			gaiaGranteeIBCBalance, err := gaia.GetBalance(ctx, gaiaGranteeAddr, gaia.Config().Denom)
			require.NoError(t, err)
			require.True(t, gaiaGranteeIBCBalance.Equal(granteeFundAmount))

			// Test granter has less than they started with, meaning fees came from their account
			gaiaGranterIBCBalance, err := gaia.GetBalance(ctx, gaiaGranterAddr, gaia.Config().Denom)
			require.NoError(t, err)
			require.True(t, gaiaGranterIBCBalance.LT(fundAmount))
			r.StopRelayer(ctx, eRep)
		})
	}
}

func TxWithRetry(ctx context.Context, client client.RPCClient, hash []byte) (*coretypes.ResultTx, error) {
	var err error
	var res *coretypes.ResultTx
	if err = retry.Do(func() error {
		res, err = client.Tx(ctx, hash, true)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr); err != nil {
		return res, err
	}

	return res, err
}

// TestRelayerFeeGrantExternal Feegrant on a single chain where the granter is an externally controlled address (no private key).
// Run this test with e.g. go test -timeout 300s -run ^TestRelayerFeeGrantExternal$ github.com/cosmos/relayer/v2/ibctest.
func TestRelayerFeeGrantExternal(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	nv := 1
	nf := 0

	var tests = [][]*interchaintest.ChainSpec{
		{
			{Name: "gaia", ChainName: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
			{Name: "osmosis", ChainName: "osmosis", Version: "v14.0.1", NumValidators: &nv, NumFullNodes: &nf},
		},
		{
			{Name: "gaia", ChainName: "gaia", Version: "v7.0.3", NumValidators: &nv, NumFullNodes: &nf},
			{Name: "kujira", ChainName: "kujira", Version: "v0.8.7", NumValidators: &nv, NumFullNodes: &nf},
		},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s,%s", tt[0].Name, tt[1].Name)
		t.Run(testname, func(t *testing.T) {

			// Chain Factory
			cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), tt)

			chains, err := cf.Chains(t.Name())
			require.NoError(t, err)
			gaia, osmosis := chains[0], chains[1]

			// Relayer Factory to construct relayer
			r := NewRelayerFactory(RelayerConfig{
				Processor:           relayer.ProcessorEvents,
				InitialBlockHistory: 100,
			}).Build(t, nil, "")

			processor.PathProcMessageCollector = make(chan *processor.PathProcessorMessageResp, 10000)

			// Prep Interchain
			const ibcPath = "gaia-osmosis"
			ic := interchaintest.NewInterchain().
				AddChain(gaia).
				AddChain(osmosis).
				AddRelayer(r, "relayer").
				AddLink(interchaintest.InterchainLink{
					Chain1:  gaia,
					Chain2:  osmosis,
					Relayer: r,
					Path:    ibcPath,
				})

			// Reporter/logs
			rep := testreporter.NewNopReporter()
			eRep := rep.RelayerExecReporter(t)

			client, network := interchaintest.DockerSetup(t)

			// Build interchain
			require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
				TestName:  t.Name(),
				Client:    client,
				NetworkID: network,

				SkipPathCreation: false,
			}))

			t.Parallel()

			// Make sure feegrant codec is registered, since it is not by default
			feegrant.RegisterInterfaces(gaia.Config().EncodingConfig.InterfaceRegistry)

			// Get Channel ID
			gaiaChans, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
			require.NoError(t, err)
			gaiaChannel := gaiaChans[0]
			osmosisChannel := gaiaChans[0].Counterparty

			// Create and Fund User Wallets
			fundAmount := sdkmath.NewInt(10_000_000)

			// Tiny amount of funding, not enough to pay for a single TX fee (the GRANTER should be paying the fee)
			granteeKeyPrefix := "grantee1"
			grantee2KeyPrefix := "grantee2"
			grantee3KeyPrefix := "grantee3"
			granterKeyPrefix := "default"

			mnemonicAny := genMnemonic(t)

			gaiaGranteeWallet, err := buildUserUnfunded(ctx, granteeKeyPrefix, mnemonicAny, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaGrantee2Wallet, err := buildUserUnfunded(ctx, grantee2KeyPrefix, mnemonicAny, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaGrantee3Wallet, err := buildUserUnfunded(ctx, grantee3KeyPrefix, mnemonicAny, gaia)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			osmosisUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "recipient", mnemonicAny, fundAmount, osmosis)
			require.NoError(t, err)

			mnemonicAny = genMnemonic(t)
			gaiaUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "recipient", mnemonicAny, fundAmount, gaia)
			require.NoError(t, err)

			// Fund the granter wallet on chain
			mnemonicAny = genMnemonic(t)
			gaiaGranterWallet, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, granterKeyPrefix, mnemonicAny, fundAmount, gaia)
			require.NoError(t, err)

			// Set SDK context to the right bech32 prefix
			prefix := gaia.Config().Bech32Prefix
			done := cosmos.SetSDKConfigContext(prefix)

			// Feegrant each of the grantees
			err = Feegrant(t, gaia.(*cosmosv8.CosmosChain), gaiaGranterWallet, gaiaGranterWallet.Address(), gaiaGranteeWallet.Address(), gaiaGranterWallet.FormattedAddress(), gaiaGranteeWallet.FormattedAddress())
			require.NoError(t, err)
			err = Feegrant(t, gaia.(*cosmosv8.CosmosChain), gaiaGranterWallet, gaiaGranterWallet.Address(), gaiaGrantee2Wallet.Address(), gaiaGranterWallet.FormattedAddress(), gaiaGrantee2Wallet.FormattedAddress())
			require.NoError(t, err)
			err = Feegrant(t, gaia.(*cosmosv8.CosmosChain), gaiaGranterWallet, gaiaGranterWallet.Address(), gaiaGrantee3Wallet.Address(), gaiaGranterWallet.FormattedAddress(), gaiaGrantee3Wallet.FormattedAddress())
			require.NoError(t, err)
			done()

			mnemonic := gaiaGranterWallet.Mnemonic()
			fmt.Printf("Wallet mnemonic: %s\n", mnemonic)

			rand.Seed(time.Now().UnixNano())

			// Notably, we do not restore the key for 'gaiaGranterWallet' to the relayer config.
			// The relayer does not need the granter private key since it is owned externally.
			// Below, the relayer restores the keys for each of the grantee wallets. It signs TXs with these keys.
			// IBC chain config (above) is unrelated to RELAYER config so this step is necessary.
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGranteeWallet.KeyName(),
				gaiaGranteeWallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGrantee2Wallet.KeyName(),
				gaiaGrantee2Wallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				gaia.Config(),
				gaiaGrantee3Wallet.KeyName(),
				gaiaGrantee3Wallet.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", gaia.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				osmosis.Config(),
				osmosisUser.KeyName(),
				osmosisUser.Mnemonic(),
			); err != nil {
				t.Fatalf("failed to restore granter key to relayer for chain %s: %s", osmosis.Config().ChainID, err.Error())
			}

			//IBC chain config is unrelated to RELAYER config so this step is necessary
			if err := r.RestoreKey(ctx,
				eRep,
				osmosis.Config(),
				gaiaUser.KeyName(),
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
			res := localRelayer.Sys().Run(logger, "chains", "configure", "feegrant", "basicallowance", gaia.Config().ChainID, gaiaGranterWallet.FormattedAddress(), "--grantees", granteeCsv, "--overwrite-granter")
			if res.Err != nil {
				fmt.Printf("configure feegrant results: %s\n", res.Stdout.String())
				t.Fatalf("failed to rly config feegrants: %v", res.Err)
			}

			//Map of feegranted chains and the feegrant info for the chain
			feegrantedChains := map[string]*chainFeegrantInfo{}
			feegrantedChains[gaia.Config().ChainID] = &chainFeegrantInfo{granter: gaiaGranterAddr, grantees: []string{gaiaGranteeAddr, gaiaGrantee2Addr, gaiaGrantee3Addr}}

			time.Sleep(14 * time.Second) //commit a couple blocks
			r.StartRelayer(ctx, eRep, ibcPath)

			// Send Transaction
			amountToSend := sdkmath.NewInt(1_000)

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

				_, err = testutil.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+20, gaiaTx.Packet)
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
				_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+20, tx.Packet)
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
				_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+20, tx.Packet)
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
				_, err = testutil.PollForAck(ctx, osmosis, osmosisHeight, osmosisHeight+20, tx.Packet)
				return err
			})

			require.NoError(t, err)
			require.NoError(t, eg.Wait())

			feegrantMsgSigners := map[string][]string{} //chain to list of signers

			for len(processor.PathProcMessageCollector) > 0 {
				select {
				case curr, ok := <-processor.PathProcMessageCollector:
					if ok && curr.Error == nil && curr.SuccessfulTx {
						cProv, cosmProv := curr.DestinationChain.(*cosmos.CosmosProvider)
						if cosmProv {
							chain := cProv.PCfg.ChainID
							feegrantInfo, isFeegrantedChain := feegrantedChains[chain]
							if isFeegrantedChain && !strings.Contains(cProv.PCfg.KeyDirectory, t.Name()) {
								//This would indicate that a parallel test is inserting msgs into the queue.
								//We can safely skip over any messages inserted by other test cases.
								fmt.Println("Skipping PathProcessorMessageResp from unrelated Parallel test case")
								continue
							}

							done := cProv.SetSDKContext()

							hash, err := hex.DecodeString(curr.Response.TxHash)
							require.Nil(t, err)
							txResp, err := TxWithRetry(ctx, cProv.RPCClient, hash)
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

							msgs := ""
							msgType := ""
							for _, m := range fullTx.GetMsgs() {
								msgType = types.MsgTypeURL(m)
								//We want all IBC transfers (on an open channel/connection) to be feegranted in round robin fashion
								if msgType == "/ibc.core.channel.v1.MsgRecvPacket" || msgType == "/ibc.core.channel.v1.MsgAcknowledgement" {
									isFeegrantedMsg = true
									msgs += msgType + ", "
								} else {
									msgs += msgType + ", "
								}
							}

							//It's required that TXs be feegranted in a round robin fashion for this chain and message type
							if isFeegrantedChain && isFeegrantedMsg {
								fmt.Printf("Msg types: %+v\n", msgs)

								signers, _, err := cProv.Cdc.Marshaler.GetMsgV1Signers(fullTx)
								require.NoError(t, err)

								require.Equal(t, len(signers), 1)
								granter := fullTx.FeeGranter(cProv.Cdc.Marshaler)

								//Feegranter for the TX that was signed on chain must be the relayer chain's configured feegranter
								require.Equal(t, feegrantInfo.granter, string(granter))
								require.NotEmpty(t, granter)

								for _, msg := range fullTx.GetMsgs() {
									msgType = types.MsgTypeURL(msg)
									//We want all IBC transfers (on an open channel/connection) to be feegranted in round robin fashion
									if msgType == "/ibc.core.channel.v1.MsgRecvPacket" {
										c := msg.(*chantypes.MsgRecvPacket)
										appData := c.Packet.GetData()
										tokenTransfer := &transfertypes.FungibleTokenPacketData{}
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
								actualGrantee := string(signers[0])
								signerList, ok := feegrantMsgSigners[chain]
								if ok {
									signerList = append(signerList, actualGrantee)
									feegrantMsgSigners[chain] = signerList
								} else {
									feegrantMsgSigners[chain] = []string{actualGrantee}
								}
								fmt.Printf("Chain: %s, msg type: %s, height: %d, signer: %s, granter: %s\n", chain, msgType, curr.Response.Height, actualGrantee, string(granter))
							}
							done()
						}
					}
				default:
					fmt.Println("Unknown channel message")
				}
			}

			for chain, signers := range feegrantMsgSigners {
				require.Equal(t, chain, gaia.Config().ChainID)
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
			require.True(t, amountToSend.Equal(gaiaIBCBalance))

			osmosisIBCBalance, err := gaia.GetBalance(ctx, osmosisDstAddress, osmosisIbcDenom)
			require.NoError(t, err)
			require.True(t, amountToSend.MulRaw(3).Equal(osmosisIBCBalance))

			// Test grantee still has exact amount expected
			gaiaGranteeIBCBalance, err := gaia.GetBalance(ctx, gaiaGranteeAddr, gaia.Config().Denom)
			require.NoError(t, err)
			require.True(t, gaiaGranteeIBCBalance.Equal(sdkmath.ZeroInt()))

			// Test granter has less than they started with, meaning fees came from their account
			gaiaGranterIBCBalance, err := gaia.GetBalance(ctx, gaiaGranterAddr, gaia.Config().Denom)
			require.NoError(t, err)
			require.True(t, gaiaGranterIBCBalance.LT(fundAmount))
			r.StopRelayer(ctx, eRep)
		})
	}
}

func buildUserUnfunded(
	ctx context.Context,
	keyNamePrefix, mnemonic string,
	chain ibc.Chain,
) (ibc.Wallet, error) {
	chainCfg := chain.Config()
	keyName := fmt.Sprintf("%s-%s-%s", keyNamePrefix, chainCfg.ChainID, randLowerCaseLetterString(3))
	user, err := chain.BuildWallet(ctx, keyName, mnemonic)
	if err != nil {
		return nil, fmt.Errorf("failed to get source user wallet: %w", err)
	}

	return user, nil
}

var chars = []byte("abcdefghijklmnopqrstuvwxyz")

// RandLowerCaseLetterString returns a lowercase letter string of given length
func randLowerCaseLetterString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func Feegrant(
	t *testing.T,
	chain *cosmosv8.CosmosChain,
	granterWallet ibc.Wallet,
	granter types.AccAddress,
	grantee types.AccAddress,
	granterAddr string,
	granteeAddr string,
) error {
	// attempt to update client with duplicate header
	b := cosmosv8.NewBroadcaster(t, chain)

	thirtyMin := time.Now().Add(30 * time.Minute)
	feeGrantBasic := &feegrant.BasicAllowance{
		Expiration: &thirtyMin,
	}
	msgGrantAllowance, err := feegrant.NewMsgGrantAllowance(feeGrantBasic, granter, grantee)
	if err != nil {
		fmt.Printf("Error: feegrant.NewMsgGrantAllowance: %s", err.Error())
		return err
	}

	// ensure correct bech32 prefix
	msgGrantAllowance.Grantee = granteeAddr
	msgGrantAllowance.Granter = granterAddr

	resp, err := cosmosv8.BroadcastTx(context.Background(), b, granterWallet, msgGrantAllowance)
	require.NoError(t, err)
	assertTransactionIsValid(t, resp)
	return nil
}

func assertTransactionIsValid(t *testing.T, resp types.TxResponse) {
	t.Helper()
	require.NotNil(t, resp)
	require.NotEqual(t, 0, resp.GasUsed)
	require.NotEqual(t, 0, resp.GasWanted)
	require.Equal(t, uint32(0), resp.Code)
	require.NotEmpty(t, resp.Data)
	require.NotEmpty(t, resp.TxHash)
	require.NotEmpty(t, resp.Events)
}
