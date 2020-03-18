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
	)

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [src-chain-id] [dst-chain-id]",
		Short: "create a clients between two configured chains with a configured path",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
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

			return chains[src].CreateClients(chains[dst])
		},
	}
	return pathFlag(cmd)
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection [src-chain-id] [dst-chain-id] [[path-name]]",
		Short: "create a connection between two configured chains with a configured path",
		Long:  "This command is meant to be used to repair or create a connection between two chains with a configured path in the config file",
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

			return chains[src].CreateConnection(chains[dst], to)
		},
	}

	return pathFlag(timeoutFlag(cmd))
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [src-chain-id] [dst-chain-id]",
		Short: "create a channel between two configured chains with a configured path",
		Long:  "This command is meant to be used to repair or create a channel between two chains with a configured path in the config file",
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

			return chains[src].CreateChannel(chains[dst], true, to)
		},
	}

	return pathFlag(timeoutFlag(cmd))
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
		Use:   "full-path [src-chain-id] [dst-chain-id]",
		Short: "create clients, connection, and channel between two configured chains with a configured path",
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

			// Check if clients have been created, if not create them
			if err = chains[src].CreateClients(chains[dst]); err != nil {
				return err
			}

			// Check if connection has been created, if not create it
			if err = chains[src].CreateConnection(chains[dst], to); err != nil {
				return err
			}

			// NOTE: this is hardcoded to create ordered channels right now. Add a flag here to toggle
			// Check if channel has been created, if not create it
			return chains[src].CreateChannel(chains[dst], true, to)

			// Check if channel has been created, if not create it
			return chains[src].CreateChannel(chains[dst], true, to)
		},
	}

	return pathFlag(timeoutFlag(cmd))
}

func transferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [src-chain-id] [dst-chain-id] [amount] [is-source] [dst-chain-addr]",
		Short: "transfer",
		Long:  "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
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

			if source {
				amount.Denom = fmt.Sprintf("%s/%s/%s", chains[dst].PathEnd.PortID, chains[dst].PathEnd.ChannelID, amount.Denom)
			} else {
				amount.Denom = fmt.Sprintf("%s/%s/%s", chains[src].PathEnd.PortID, chains[src].PathEnd.ChannelID, amount.Denom)
			}

			dstAddr, err := sdk.AccAddressFromBech32(args[4])
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			// MsgTransfer will call SendPacket on src chain
			txs := relayer.RelayMsgs{
				Src: []sdk.Msg{chains[src].PathEnd.MsgTransfer(chains[dst].PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, source, chains[src].MustGetAddress())},
				Dst: []sdk.Msg{},
			}

			if txs.Send(chains[src], chains[dst]); !txs.Success() {
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

			for {
				hs, err = relayer.UpdatesWithHeaders(chains[src], chains[dst])
				if err != nil {
					return err
				}

				seqRecv, err = chains[dst].QueryNextSeqRecv(hs[dst].Height)
				if err != nil {
					return err
				}

				seqSend, err = chains[src].QueryNextSeqSend(hs[src].Height)
				if err != nil {
					return err
				}

				srcCommitRes, err = chains[src].QueryPacketCommitment(hs[src].Height-1, int64(seqSend-1))
				if err != nil {
					return err
				}

				if srcCommitRes.Proof.Proof == nil {
					continue
				} else {
					break
				}
			}

			// reconstructing packet data here instead of retrieving from an indexed node
			xferPacket := chains[src].PathEnd.XferPacket(
				sdk.NewCoins(amount),
				chains[src].MustGetAddress(),
				dstAddr,
				source,
				dstHeader.GetHeight()+1000,
			)

			// Debugging by simply passing in the packet information that we know was sent earlier in the SendPacket
			// part of the command. In a real relayer, this would be a separate command that retrieved the packet
			// information from an indexing node
			txs = relayer.RelayMsgs{
				Dst: []sdk.Msg{
					chains[dst].PathEnd.UpdateClient(hs[src], chains[dst].MustGetAddress()),
					chains[src].PathEnd.MsgRecvPacket(
						chains[dst].PathEnd,
						seqRecv.NextSequenceRecv,
						xferPacket,
						chanTypes.NewPacketResponse(
							chains[src].PathEnd.PortID,
							chains[src].PathEnd.ChannelID,
							seqSend-1,
							chains[src].PathEnd.NewPacket(
								chains[src].PathEnd,
								seqSend-1,
								xferPacket,
							),
							srcCommitRes.Proof.Proof,
							int64(srcCommitRes.ProofHeight),
						),
						chains[dst].MustGetAddress(),
					),
				},
				Src: []sdk.Msg{},
			}

			txs.Send(chains[src], chains[dst])
			return nil
		},
	}
	return pathFlag(cmd)
}

func setPathsFromArgs(src, dst *relayer.Chain, name string) (*relayer.Path, error) {
	// Find any configured paths between the chains
	paths, err := config.Paths.PathsFromChains(src.ChainID, dst.ChainID)
	if err != nil {
		return nil, err
	}

	// Given the number of args and the number of paths,
	// work on the appropriate path
	var path *relayer.Path
	switch {
	case name != "" && len(paths) > 1:
		if path, err = paths.Get(name); err != nil {
			return path, err
		}
	case name != "" && len(paths) == 1:
		if path, err = paths.Get(name); err != nil {
			return path, err
		}
	case name == "" && len(paths) > 1:
		return nil, fmt.Errorf("more than one path between %s and %s exists, pass in path name", src, dst)
	case name == "" && len(paths) == 1:
		for _, v := range paths {
			path = v
		}
	}

	if err = src.SetPath(path.End(src.ChainID)); err != nil {
		return nil, err
	}

	if err = dst.SetPath(path.End(dst.ChainID)); err != nil {
		return nil, err
	}

	return path, nil
}
