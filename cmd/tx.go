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
	"strconv"

	"github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

// transactionCmd represents the tx command
func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transactions",
		Aliases: []string{"tx"},
		Short:   "IBC Transaction Commands",
	}

	cmd.AddCommand(
		createClientsCmd(),
		createConnectionCmd(),
		createChannelCmd(),
		closeChannelCmd(),
		fullPathCmd(),
		rawTransactionCmd(),
		transferCmd(),
		relayMsgsCmd(),
	)

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients [path-name]",
		Aliases: []string{"clnts"},
		Short:   "create a clients between two configured chains with a configured path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			return c[src].CreateClients(c[dst])
		},
	}
	return cmd
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [path-name]",
		Aliases: []string{"conn"},
		Short:   "create a connection between two configured chains with a configured path",
		Long:    "This command is meant to be used to repair or create a connection between two chains with a configured path in the config file",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			return c[src].CreateConnection(c[dst], to)
		},
	}

	return timeoutFlag(cmd)
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel [path-name]",
		Aliases: []string{"chan"},
		Short:   "create a channel between two configured chains with a configured path",
		Long:    "This command is meant to be used to repair or create a channel between two chains with a configured path in the config file",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// TODO: read order out of path config
			return c[src].CreateChannel(c[dst], true, to)
		},
	}

	return timeoutFlag(cmd)
}

func closeChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel-close [src-chain-id] [dst-chain-id]",
		Short: "close a channel between two configured chains with a configured path",
		Long:  "This command is meant to close a channel",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			pth, err := cmd.Flags().GetString(flagPath)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(chains[src], chains[dst], pth); err != nil {
				return err
			}

			return chains[src].CloseChannel(chains[dst], to)
		},
	}

	return pathFlag(timeoutFlag(cmd))
}

func fullPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "full-path [path-name]",
		Aliases: []string{"link", "connect", "path", "pth"},
		Short:   "create clients, connection, and channel between two configured chains with a configured path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}

			src, dst := path.Src.ChainID, path.Dst.ChainID
			c, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = c[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = c[dst].SetPath(path.Dst); err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// Check if clients have been created, if not create them
			if err = c[src].CreateClients(c[dst]); err != nil {
				return err
			}

			// Check if connection has been created, if not create it
			if err = c[src].CreateConnection(c[dst], to); err != nil {
				return err
			}

			// Check if channel has been created, if not create it
			return c[src].CreateChannel(c[dst], true, to)
		},
	}

	return timeoutFlag(cmd)
}

func relayMsgsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay [path-name]",
		Aliases: []string{"rly"},
		Short:   "Queries for the packets that remain to be relayed on a given path, in both directions, and relays them",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = relayer.RelayUnRelayedPacketsOrderedChan(c[src], c[dst]); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func transferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transfer [src-chain-id] [dst-chain-id] [amount] [is-source] [dst-chain-addr]",
		Aliases: []string{"xfer"},
		Short:   "transfer",
		Long:    "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
		Args:    cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			c, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			pth, err := cmd.Flags().GetString(flagPath)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(c[src], c[dst], pth); err != nil {
				return err
			}

			amount, err := sdk.ParseCoin(args[2])
			if err != nil {
				return err
			}

			// If there is a path seperator in the denom of the coins being sent,
			// then src is not the source, otherwise it is
			// NOTE: this will not work in the case where tokens are sent from A -> B -> C
			// Need a function in the SDK to determine from a denom if the tokens are from this chain
			// TODO: Refactor this in the SDK.
			source, err := strconv.ParseBool(args[3])
			if err != nil {
				return err
			}

			// TODO: This needs to be changed to incorporate the upstream changes
			if source {
				amount.Denom = fmt.Sprintf("%s/%s/%s", c[dst].PathEnd.PortID, c[dst].PathEnd.ChannelID, amount.Denom)
			} else {
				amount.Denom = fmt.Sprintf("%s/%s/%s", c[src].PathEnd.PortID, c[src].PathEnd.ChannelID, amount.Denom)
			}

			dstAddr, err := sdk.AccAddressFromBech32(args[4])
			if err != nil {
				return err
			}

			dstHeader, err := c[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			// MsgTransfer will call SendPacket on src chain
			txs := relayer.RelayMsgs{
				Src: []sdk.Msg{c[src].PathEnd.MsgTransfer(c[dst].PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, c[src].MustGetAddress())},
				Dst: []sdk.Msg{},
			}

			if txs.Send(c[src], c[dst]); !txs.Success() {
				return fmt.Errorf("failed to send first transaction")
			}

			// Working on SRC chain :point_up:
			// Working on DST chain :point_down:

			var (
				hs           map[string]*tmclient.Header
				seqRecv      chanTypes.RecvResponse
				seqSend      uint64
				srcCommitRes relayer.CommitmentResponse
			)

			if err = retry.Do(func() error {
				hs, err = relayer.UpdatesWithHeaders(c[src], c[dst])
				if err != nil {
					return err
				}

				seqRecv, err = c[dst].QueryNextSeqRecv(hs[dst].Height)
				if err != nil {
					return err
				}

				seqSend, err = c[src].QueryNextSeqSend(hs[src].Height)
				if err != nil {
					return err
				}

				srcCommitRes, err = c[src].QueryPacketCommitment(hs[src].Height-1, int64(seqSend-1))
				if err != nil {
					return err
				}

				if srcCommitRes.Proof.Proof == nil {
					return fmt.Errorf("nil proof, retrying")
				}
				return nil
			}); err != nil {
				return err
			}

			// reconstructing packet data here instead of retrieving from an indexed node
			xferPacket := c[src].PathEnd.XferPacket(
				sdk.NewCoins(amount),
				c[src].MustGetAddress(),
				dstAddr,
			)

			// Debugging by simply passing in the packet information that we know was sent earlier in the SendPacket
			// part of the command. In a real relayer, this would be a separate command that retrieved the packet
			// information from an indexing node
			txs = relayer.RelayMsgs{
				Dst: []sdk.Msg{
					c[dst].PathEnd.UpdateClient(hs[src], c[dst].MustGetAddress()),
					c[dst].PathEnd.MsgRecvPacket(
						c[src].PathEnd,
						seqRecv.NextSequenceRecv,
						uint64(hs[dst].Height+1000),
						xferPacket,
						chanTypes.NewPacketResponse(
							c[src].PathEnd.PortID,
							c[src].PathEnd.ChannelID,
							seqSend-1,
							c[src].PathEnd.NewPacket(
								c[dst].PathEnd,
								seqSend-1,
								xferPacket,
								uint64(hs[dst].Height+1000),
							),
							srcCommitRes.Proof.Proof,
							int64(srcCommitRes.ProofHeight),
						),
						c[dst].MustGetAddress(),
					),
				},
				Src: []sdk.Msg{},
			}

			txs.Send(c[src], c[dst])
			return nil
		},
	}
	return pathFlag(cmd)
}
