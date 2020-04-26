package cmd

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

////////////////////////////////////////
////  RAW IBC TRANSACTION COMMANDS  ////
////////////////////////////////////////

func rawTransactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raw",
		Short: "raw IBC transaction commands",
	}

	cmd.AddCommand(
		updateClientCmd(),
		createClientCmd(),
		connInit(),
		connTry(),
		connAck(),
		connConfirm(),
		createConnectionStepCmd(),
		chanInit(),
		chanTry(),
		chanAck(),
		chanConfirm(),
		createChannelStepCmd(),
		chanCloseInit(),
		chanCloseConfirm(),
		closeChannelStepCmd(),
		xfersend(),
	)

	return cmd
}
func updateClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update-client [src-chain-id] [dst-chain-id] [client-id]",
		Aliases: []string{"uc"},
		Short:   "update client for dst-chain on src-chain",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, dcha, dpor, dord); err != nil {
				return err
			}
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{chains[src].PathEnd.UpdateClient(dstHeader, chains[src].MustGetAddress())}, chains[src], cmd)
		},
	}
	return cmd
}

func createClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client [src-chain-id] [dst-chain-id] [client-id]",
		Aliases: []string{"clnt"},
		Short:   "create a client for dst-chain on src-chain",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{chains[src].PathEnd.CreateClient(dstHeader, chains[src].GetTrustingPeriod(), chains[src].MustGetAddress())}, chains[src], cmd)
		},
	}
	return cmd
}

func connInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-init",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], dcha, dpor, dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], dcha, dpor, dord); err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{chains[src].PathEnd.ConnInit(chains[dst].PathEnd, chains[src].MustGetAddress())}, chains[src], cmd)
		},
	}
	return cmd
}

func connTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-try [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-try",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], dcha, dpor, dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], dcha, dpor, dord); err != nil {
				return err
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
				chains[src].PathEnd.UpdateClient(hs[dst], chains[src].MustGetAddress()),
				chains[src].PathEnd.ConnTry(chains[dst].PathEnd, dstConnState, dstConsState, dstCsHeight, chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func connAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-ack [src-chain-id] [dst-chain-id] [dst-client-id] [src-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-ack",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], dcha, dpor, dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], dcha, dpor, dord); err != nil {
				return err
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
				chains[src].PathEnd.ConnAck(dstState, dstConsState, dstCsHeight, chains[src].MustGetAddress()),
				chains[src].PathEnd.UpdateClient(hs[dst], chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func connConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-confirm [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-confirm",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], dcha, dpor, dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], dcha, dpor, dord); err != nil {
				return err
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
				chains[src].PathEnd.ConnConfirm(dstState, chains[src].MustGetAddress()),
				chains[src].PathEnd.UpdateClient(hs[dst], chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func createConnectionStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]",
		Aliases: []string{"conn-step"},
		Short:   "create a connection between chains, passing in identifiers",
		Long:    "This command creates the next handshake message given a specifc set of identifiers. If the command fails, you can safely run it again to repair an unfinished connection",
		Args:    cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], dcha, dpor, dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], dcha, dpor, dord); err != nil {
				return err
			}

			msgs, err := chains[src].CreateConnectionStep(chains[dst])
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = sendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = sendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}
	return cmd
}

func chanInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [ordering]",
		Short: "chan-init",
		Args:  cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(args[0], args[1])
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], args[6], args[8], args[10]); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], args[7], args[9], args[10]); err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{chains[src].PathEnd.ChanInit(chains[dst].PathEnd, chains[src].MustGetAddress())}, chains[src], cmd)
		},
	}
	return cmd
}

func chanTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-try [src-chain-id] [dst-chain-id] [src-client-id] [src-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-try",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[3], args[4], args[6], dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(dcli, dcon, args[5], args[7], dord); err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height - 1)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].PathEnd.UpdateClient(dstHeader, chains[src].MustGetAddress()),
				chains[src].PathEnd.ChanTry(chains[dst].PathEnd, dstChanState, chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func chanAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-ack [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-ack",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, args[3], args[5], dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(dcli, dcon, args[4], args[6], dord); err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height - 1)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].PathEnd.UpdateClient(dstHeader, chains[src].MustGetAddress()),
				chains[src].PathEnd.ChanAck(dstChanState, chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func chanConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-confirm [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-confirm",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, args[3], args[5], dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(dcli, dcon, args[4], args[6], dord); err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height - 1)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].PathEnd.UpdateClient(dstHeader, chains[src].MustGetAddress()),
				chains[src].PathEnd.ChanConfirm(dstChanState, chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func createChannelStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [ordering]",
		Aliases: []string{"chan-step"},
		Short:   "create the next step in creating a channel between chains with the passed identifiers",
		Args:    cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			ordering := chanState.OrderFromString(args[10])
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], args[6], args[8], dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], args[7], args[9], dord); err != nil {
				return err
			}

			msgs, err := chains[src].CreateChannelStep(chains[dst], ordering)
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = sendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = sendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}
	return cmd
}

func chanCloseInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-close-init [chain-id] [channel-id] [port-id]",
		Short: "chan-close-init",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := src.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{src.PathEnd.ChanCloseInit(src.MustGetAddress())}, src, cmd)
		},
	}
	return cmd
}

func chanCloseConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-close-confirm [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]",
		Short: "chan-close-confirm",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, args[3], args[5], dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(dcli, dcon, args[4], args[6], dord); err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(dstHeader.Height - 1)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].PathEnd.UpdateClient(dstHeader, chains[src].MustGetAddress()),
				chains[src].PathEnd.ChanCloseConfirm(dstChanState, chains[src].MustGetAddress()),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func closeChannelStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close-channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]",
		Short: "create the next step in closing a channel between chains with the passed identifiers",
		Args:  cobra.ExactArgs(10),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], args[4], args[6], args[8], dord); err != nil {
				return err
			}

			if err = chains[dst].AddPath(args[3], args[5], args[7], args[9], dord); err != nil {
				return err
			}

			msgs, err := chains[src].CloseChannelStep(chains[dst])
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = sendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = sendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}
	return cmd
}

func sendAndPrint(txs []sdk.Msg, c *relayer.Chain, cmd *cobra.Command) error {
	return c.SendAndPrint(txs, false, false)
}
