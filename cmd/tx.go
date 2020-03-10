/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

func init() {
	transactionCmd.AddCommand(createClientCmd())
	transactionCmd.AddCommand(createClientsCmd())
	transactionCmd.AddCommand(createConnectionCmd())
	transactionCmd.AddCommand(createConnectionStepCmd())
	transactionCmd.AddCommand(createChannelCmd())
	transactionCmd.AddCommand(createChannelStepCmd())
	transactionCmd.AddCommand(updateClientCmd())
	transactionCmd.AddCommand(rawTransactionCmd)
	rawTransactionCmd.AddCommand(connInit())
	rawTransactionCmd.AddCommand(connTry())
	rawTransactionCmd.AddCommand(connAck())
	rawTransactionCmd.AddCommand(connConfirm())
	rawTransactionCmd.AddCommand(chanInit())
	rawTransactionCmd.AddCommand(chanTry())
	rawTransactionCmd.AddCommand(chanAck())
	rawTransactionCmd.AddCommand(chanConfirm())
	rawTransactionCmd.AddCommand(chanCloseInit())
	rawTransactionCmd.AddCommand(chanCloseConfirm())
}

// transactionCmd represents the tx command
var transactionCmd = &cobra.Command{
	Use:     "transactions",
	Aliases: []string{"tx"},
	Short:   "IBC Transaction Commands, UNDER CONSTRUCTION",
}

func updateClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-client [src-chain-id] [dst-chain-id] [client-id]",
		Short: "update client for dst-chain on src-chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathClient(args[2]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTPATH, err)
			}
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			return SendAndPrint([]sdk.Msg{chains[src].UpdateClient(dstHeader)}, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func createClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client [src-chain-id] [dst-chain-id] [client-id]",
		Short: "create a client for dst-chain on src-chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			err = chains[src].PathClient(args[2])
			if err != nil {
				return err
			}

			return SendAndPrint([]sdk.Msg{chains[src].CreateClient(dstHeader)}, chains[src], cmd)
		},
	}

	return transactionFlags(cmd)
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id]",
		Short: "create a clients for dst-chain on src-chain and src-chain on dst-chain",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			if err = chains[src].PathClient(args[2]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTPATH, err)
			}

			if err = chains[dst].PathClient(args[3]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CLNTPATH, err)
			}

			if err = SendAndPrint([]sdk.Msg{chains[src].CreateClient(hs[dst])}, chains[src], cmd); err != nil {
				return err
			}

			return SendAndPrint([]sdk.Msg{chains[dst].CreateClient(hs[src])}, chains[dst], cmd)
		},
	}
	return transactionFlags(cmd)
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]",
		Short: "create a connection between chains, passing in identifiers",
		Long:  "Working, but not smoothly",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			ticker := time.NewTicker(to)
			for ; true; <-ticker.C {
				msgs, err := chains[src].CreateConnectionStep(chains[dst])
				if err != nil {
					return err
				}

				if !msgs.Ready() {
					break
				}

				if len(msgs.Src) > 0 {
					// Submit the transactions to src chain
					err = SendAndPrint(msgs.Src, chains[src], cmd)
					if err != nil {
						return err
					}
				}

				if len(msgs.Dst) > 0 {
					// Submit the transactions to dst chain
					err = SendAndPrint(msgs.Dst, chains[dst], cmd)
					if err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	return timeoutFlag(transactionFlags(cmd))
}

func createConnectionStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]",
		Short: "create a connection between chains, passing in identifiers",
		Long:  "This command creates the next handshake message given a specifc set of identifiers. If the command fails, you can safely run it again to repair an unfinished connection",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			msgs, err := chains[src].CreateConnectionStep(chains[dst])
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = SendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = SendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return transactionFlags(cmd)
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [ordering]",
		Short: "create a channel with the passed identifiers between chains",
		Long:  "FYI: DRAGONS HERE, not tested",
		Args:  cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if err := chains[src].FullPath(args[2], args[4], args[6], args[8]); err != nil {
				return chains[src].ErrCantSetPath(relayer.FULLPATH, err)
			}

			if err := chains[dst].FullPath(args[3], args[5], args[7], args[9]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.FULLPATH, err)
			}

			var order chanState.Order
			if order = chanState.OrderFromString(args[10]); order == chanState.NONE {
				return fmt.Errorf("invalid order passed in %s, expected 'UNORDERED' or 'ORDERED'", args[10])
			}

			err = chains[src].CreateChannel(chains[dst], to, order)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return timeoutFlag(cmd)
}

func createChannelStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [ordering]",
		Short: "create the next step in creating a channel between chains with the passed identifiers",
		Long:  "FYI: DRAGONS HERE, not tested",
		Args:  cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			ordering := chanState.OrderFromString(args[10])
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].FullPath(args[2], args[4], args[6], args[8]); err != nil {
				return chains[src].ErrCantSetPath(relayer.FULLPATH, err)
			}

			if err = chains[dst].FullPath(args[3], args[5], args[7], args[9]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.FULLPATH, err)
			}

			msgs, err := chains[src].CreateChannelStep(chains[dst], ordering)
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = SendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = SendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return transactionFlags(cmd)
}

////////////////////////////////////////
////  RAW IBC TRANSACTION COMMANDS  ////
////////////////////////////////////////

var rawTransactionCmd = &cobra.Command{
	Use:   "raw",
	Short: "raw connection and channel steps",
}

func connInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-init",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			return SendAndPrint([]sdk.Msg{chains[src].ConnInit(chains[dst])}, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func connTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-try [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-try",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			// NOTE: We query connection at height - 1 because of the way tendermint returns
			// proofs the commit for height n is contained in the header of height n + 1
			dstConnState, err := chains[dst].QueryConnection(hs[dst].Height - 1)
			if err != nil {
				return err
			}

			// We are querying the state of the client for src on dst and finding the height
			dstClientState, err := chains[dst].QueryClientState()
			if err != nil {
				return err
			}
			dstCsHeight := int64(dstClientState.ClientState.GetLatestHeight())

			// Then we need to query the consensus state for src at that height on dst
			dstConsState, err := chains[dst].QueryClientConsensusState(hs[dst].Height-1, dstCsHeight)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].UpdateClient(hs[dst]),
				chains[src].ConnTry(chains[dst], dstConnState, dstConsState, dstCsHeight),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func connAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-ack [src-chain-id] [dst-chain-id] [dst-client-id] [src-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-ack",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			// NOTE: We query connection at height - 1 because of the way tendermint returns
			// proofs the commit for height n is contained in the header of height n + 1
			dstState, err := chains[dst].QueryConnection(hs[dst].Height - 1)
			if err != nil {
				return err
			}

			// We are querying the state of the client for src on dst and finding the height
			dstClientState, err := chains[dst].QueryClientState()
			if err != nil {
				return err
			}
			dstCsHeight := int64(dstClientState.ClientState.GetLatestHeight())

			// Then we need to query the consensus state for src at that height on dst
			dstConsState, err := chains[dst].QueryClientConsensusState(hs[dst].Height-1, dstCsHeight)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].ConnAck(dstState, dstConsState, dstCsHeight),
				chains[src].UpdateClient(hs[dst]),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func connConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-confirm [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-confirm",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			// NOTE: We query connection at height - 1 because of the way tendermint returns
			// proofs the commit for height n is contained in the header of height n + 1
			dstState, err := chains[dst].QueryConnection(hs[dst].Height - 1)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].ConnConfirm(dstState),
				chains[src].UpdateClient(hs[dst]),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func chanInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [ordering]",
		Short: "chan-init",
		Args:  cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(args[0], args[1])
			if err != nil {
				return err
			}

			if err = chains[src].FullPath(args[2], args[4], args[6], args[8]); err != nil {
				return chains[src].ErrCantSetPath(relayer.FULLPATH, err)
			}

			if err = chains[dst].FullPath(args[3], args[5], args[7], args[9]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.FULLPATH, err)
			}

			var order chanState.Order
			if order = chanState.OrderFromString(args[6]); order == chanState.NONE {
				return fmt.Errorf("invalid order '%s' passed in, expected 'UNORDERED' or 'ORDERED'", args[6])
			}

			return SendAndPrint([]sdk.Msg{chains[src].ChanInit(chains[dst], order)}, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func chanTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-try [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-try",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannelClient(args[2], args[3], args[5]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[4], args[6]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].UpdateClient(dstHeader),
				chains[src].ChanTry(chains[dst], dstChanState),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func chanAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-ack [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-ack",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannelClient(args[2], args[3], args[5]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[4], args[6]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].UpdateClient(dstHeader),
				chains[src].ChanAck(dstChanState),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func chanConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-confirm [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-confirm",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannelClient(args[2], args[3], args[5]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[4], args[6]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].UpdateClient(dstHeader),
				chains[src].ChanConfirm(dstChanState),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func chanCloseInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-close-init [chain-id] [chan-id] [port-id]",
		Short: "chan-close-init",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err := src.PathChannel(args[1], args[2]); err != nil {
				return src.ErrCantSetPath(relayer.CHANPATH, err)
			}

			return SendAndPrint([]sdk.Msg{src.ChanCloseInit()}, src, cmd)
		},
	}
	return transactionFlags(cmd)
}

func chanCloseConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-close-confirm [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-close-confirm",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannelClient(args[2], args[3], args[5]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[4], args[6]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].UpdateClient(dstHeader),
				chains[src].ChanCloseConfirm(dstChanState),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}
