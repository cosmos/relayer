package cmd

import (
	"fmt"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/23-commitment/types"
	ibctmtypes "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/light"
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
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw update-client ibc-0 ibc-1 ibczeroclient
$ %s tx raw uc ibc-0 ibc-1 ibconeclient`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			updateMsg, err := chains[src].UpdateClient(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{updateMsg},
				chains[src], cmd)
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
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw client ibc-0 ibc-1 ibczeroclient
$ %s tx raw clnt ibc-1 ibc-0 ibconeclient`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			dstHeader, err := chains[src].GetIBCCreateClientHeader()
			if err != nil {
				return err
			}

			if err = chains[src].AddPath(args[2], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			ubdPeriod, err := chains[dst].QueryUnbondingPeriod()
			if err != nil {
				return err
			}

			clientState := ibctmtypes.NewClientState(
				dstHeader.GetHeader().GetChainID(),
				ibctmtypes.NewFractionFromTm(light.DefaultTrustLevel),
				chains[dst].GetTrustingPeriod(),
				ubdPeriod,
				time.Minute*10,
				dstHeader.GetHeight().(clienttypes.Height),
				commitmenttypes.GetSDKSpecs(),
				relayer.DefaultUpgradePath,
				relayer.AllowUpdateAfterExpiry,
				relayer.AllowUpdateAfterMisbehaviour,
			)

			return sendAndPrint([]sdk.Msg{chains[src].CreateClient(
				clientState, dstHeader)},
				chains[src], cmd)
		},
	}
	return cmd
}

func connInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-init",
		Args:  cobra.ExactArgs(6),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw conn-init ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2
$ %s tx raw conn-init ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2`, appName, appName)),
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

			msgs, err := chains[src].ConnInit(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs,
				chains[src], cmd)
		},
	}
	return cmd
}

func connTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-try [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-try",
		Args:  cobra.ExactArgs(6),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw conn-try ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2
$ %s tx raw conn-try ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2`, appName, appName)),
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

			msgs, err := chains[src].ConnTry(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs, chains[src], cmd)
		},
	}
	return cmd
}

func connAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-ack [src-chain-id] [dst-chain-id] [dst-client-id] [src-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-ack",
		Args:  cobra.ExactArgs(6),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw conn-ack ibc-0 ibc-1 ibconeclient ibczeroclient ibcconn1 ibcconn2
$ %s tx raw conn-ack ibc-0 ibc-1 ibconeclient ibczeroclient ibcconn1 ibcconn2`, appName, appName)),
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

			msgs, err := chains[src].ConnAck(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs, chains[src], cmd)
		},
	}
	return cmd
}

func connConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-confirm [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-confirm",
		Args:  cobra.ExactArgs(6),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw conn-confirm ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2
$ %s tx raw conn-confirm ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2`, appName, appName)),
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

			msgs, err := chains[src].ConnConfirm(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs, chains[src], cmd)
		},
	}
	return cmd
}

func createConnectionStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] 
		[src-connection-id] [dst-connection-id]`),
		Aliases: []string{"conn-step"},
		Short:   "create a connection between chains, passing in identifiers",
		Long: strings.TrimSpace(`This command creates the next handshake message given a specifc set of identifiers. 
		If the command fails, you can safely run it again to repair an unfinished connection`),
		Args: cobra.ExactArgs(6),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw connection-step ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2
$ %s tx raw conn-step ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 ibcconn2`, appName, appName)),
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

			_, _, modified, err := relayer.ExecuteConnectionStep(chains[src], chains[dst])
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}
			if err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}

func chanInit() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`chan-init [src-chain-id] [dst-chain-id] [src-client-id] 
		[dst-client-id] [src-conn-id] [dst-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [ordering]`),
		Short: "chan-init",
		Args:  cobra.ExactArgs(11),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx raw chan-init ibc-0 ibc-1 ibczeroclient ibconeclient 
ibcconn1 ibcconn2 ibcchan1 ibcchan2 transfer transfer ordered`, appName)),
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

			msgs, err := chains[src].ChanInit(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs,
				chains[src], cmd)
		},
	}
	return cmd
}

func chanTry() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`chan-try [src-chain-id] [dst-chain-id] 
		[src-client-id] [src-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]`),
		Short: "chan-try",
		Args:  cobra.ExactArgs(8),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw chan-try ibc-0 ibc-1 ibczeroclient ibcconn0 ibcchan1 ibcchan2 transfer transfer
$ %s tx raw chan-try ibc-0 ibc-1 ibczeroclient ibcconn0 ibcchan1 ibcchan2 transfer transfer`, appName, appName)),
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

			msgs, err := chains[src].ChanTry(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs, chains[src], cmd)
		},
	}
	return cmd
}

func chanAck() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`chan-ack [src-chain-id] [dst-chain-id] [src-client-id] 
		[src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]`),
		Short: "chan-ack",
		Args:  cobra.ExactArgs(7),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw chan-ack ibc-0 ibc-1 ibczeroclient ibcchan1 ibcchan2 transfer transfer
$ %s tx raw chan-ack ibc-0 ibc-1 ibczeroclient ibcchan1 ibcchan2 transfer transfer
`, appName, appName)),
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

			msgs, err := chains[src].ChanAck(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs, chains[src], cmd)
		},
	}
	return cmd
}

func chanConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`chan-confirm [src-chain-id] [dst-chain-id] [src-client-id] 
		[src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]`),
		Short: "chan-confirm",
		Args:  cobra.ExactArgs(7),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw chan-confirm ibc-0 ibc-1 ibczeroclient ibcchan1 ibcchan2 transfer transfer
$ %s tx raw chan-confirm ibc-0 ibc-1 ibczeroclient ibcchan1 ibcchan2 transfer transfer`, appName, appName)),
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

			msgs, err := chains[src].ChanConfirm(chains[dst])
			if err != nil {
				return err
			}

			return sendAndPrint(msgs, chains[src], cmd)
		},
	}
	return cmd
}

func createChannelStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] 
		[src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [ordering]`),
		Aliases: []string{"chan-step"},
		Short:   "create the next step in creating a channel between chains with the passed identifiers",
		Args:    cobra.ExactArgs(11),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw chan-step ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 
ibcconn2 ibcchan1 ibcchan2 transfer transfer ordered
$ %s tx raw channel-step ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1
 ibcconn2 ibcchan1 ibcchan2 transfer transfer ordered`, appName, appName)),
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

			_, _, modified, err := relayer.ExecuteChannelStep(chains[src], chains[dst])
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}
			if err != nil {
				return err
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
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact raw chan-close-init ibc-0 ibcchan1 transfer
$ %s tx raw chan-close-init ibc-0 ibcchan1 transfer`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := src.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			return sendAndPrint([]sdk.Msg{src.ChanCloseInit()}, src, cmd)
		},
	}
	return cmd
}

func chanCloseConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`chan-close-confirm [src-chain-id] [dst-chain-id] 
		[src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id]`),
		Short: "chan-close-confirm",
		Args:  cobra.ExactArgs(7),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx raw chan-close-confirm ibc-0 ibc-1 ibczeroclient ibcchan1 ibcchan2 transfer transfer`, appName)),
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

			updateMsg, err := chains[src].UpdateClient(chains[dst])
			if err != nil {
				return err
			}

			dstChanState, err := chains[dst].QueryChannel(int64(chains[dst].MustGetLatestLightHeight()) - 1)
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				updateMsg,
				chains[src].ChanCloseConfirm(dstChanState),
			}

			return sendAndPrint(txs, chains[src], cmd)
		},
	}
	return cmd
}

func closeChannelStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: strings.TrimSpace(`close-channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] 
		[src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]`),
		Short: "create the next step in closing a channel between chains with the passed identifiers",
		Args:  cobra.ExactArgs(10),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx raw close-channel-step ibc-0 ibc-1 ibczeroclient ibconeclient ibcconn1 
ibcconn2 ibcchan1 ibcchan2 transfer transfer`, appName)),
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
