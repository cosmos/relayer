package cmd

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// NOTE: These commands are registered over in cmd/raw.go

func xfersend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer-send [src-chain-id] [dst-chain-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [amount] [dst-addr]",
		Short: "xfer-send",
		Long:  "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
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

			amount, err := sdk.ParseCoin(args[6])
			if err != nil {
				return err
			}

			// If there is a path seperator in the denom of the coins being sent,
			// then src is not the source, otherwise it is
			// NOTE: this will not work in the case where tokens are sent from A -> B -> C
			// Need a function in the SDK to determine from a denom if the tokens are from this chain
			var source bool
			if strings.Contains(amount.GetDenom(), "/") {
				source = false
			} else {
				source = true
			}

			dstAddr, err := sdk.AccAddressFromBech32(args[7])
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].PathEnd.MsgTransfer(chains[dst].PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, source, chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
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
