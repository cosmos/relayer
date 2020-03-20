package cmd

import (
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

// NOTE: These commands are registered over in cmd/raw.go

func xfersend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer-send [src-chain-id] [dst-chain-id] [amount] [source] [dst-addr]",
		Short: "xfer-send",
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

			return nil
		},
	}
	return pathFlag(cmd)
}

// UNTESTED: Currently filled with incorrect logic to make code compile
func xferrecv() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer-recv [src-chain-id] [dst-chain-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [amount] [dst-addr]",
		Short: "xfer-recv",
		Long:  "recives tokens sent from dst to src",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(dcli, dcon, args[2], args[4]); err != nil {
				return err
			}

			if err = chains[dst].AddPath(dcli, dcon, args[3], args[5]); err != nil {
				return err
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			seqRecv, err := chains[src].QueryNextSeqRecv(hs[src].Height - 1)
			if err != nil {
				return err
			}

			// seqSend, err := chains[dst].QueryNextSeqSend(hs[dst].Height - 1)
			// if err != nil {
			// 	return err
			// }

			// dstCommitRes, err := chains[dst].QueryPacketCommitment(hs[dst].Height, int64(seqSend))
			// if err != nil {
			// 	return err
			// }

			txs := []sdk.Msg{
				chains[src].PathEnd.UpdateClient(hs[dst], chains[src].MustGetAddress()),
				chains[src].PathEnd.MsgRecvPacket(
					chains[dst].PathEnd,
					seqRecv.NextSequenceRecv,
					chains[src].PathEnd.XferPacket(
						sdk.NewCoins(),
						chains[src].MustGetAddress(),
						chains[src].MustGetAddress(),
						false,
						19291024),
					chanTypes.PacketResponse{},
					chains[src].MustGetAddress(),
				),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}
