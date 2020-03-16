package cmd

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// NOTE: These commands are registered over in cmd/raw.go

func xfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer [src-chain-id] [dst-chain-id] [[path-name]]",
		Short: "xfer",
		Long:  "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(chains[src], chains[dst], args[2]); err != nil {
				return err
			}

			amount, err := sdk.ParseCoin("10stake")
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

			dstAddr := chains[dst].MustGetAddress()

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

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			seqRecv, err := chains[dst].QueryNextSeqRecv(hs[dst].Height - 1)
			if err != nil {
				return err
			}

			seqSend, err := chains[src].QueryNextSeqSend(hs[src].Height - 1)
			if err != nil {
				return err
			}

			srcCommitRes, err := chains[src].QueryPacketCommitment(hs[src].Height, int64(seqSend-1))
			if err != nil {
				return err
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
					chains[dst].PathEnd.MsgRecvPacket(
						chains[src].PathEnd,
						seqRecv.NextSequenceRecv,
						xferPacket,
						chanTypes.NewPacketResponse(
							chains[src].PathEnd.PortID,
							chains[src].PathEnd.ChannelID,
							seqSend,
							chains[dst].PathEnd.NewPacket(
								chains[src].PathEnd,
								seqSend,
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
	return cmd
}

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
