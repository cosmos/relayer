package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/helpers"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/spf13/cobra"
)

// queryCmd represents the chain command
func queryCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query",
		Aliases: []string{"q"},
		Short:   "IBC query commands",
		Long:    "Commands to query IBC primitives and other useful data on configured chains.",
	}

	cmd.AddCommand(
		queryUnrelayedPackets(a),
		queryUnrelayedAcknowledgements(a),
		lineBreakCommand(),
		//queryAccountCmd(),
		queryBalanceCmd(a),
		queryHeaderCmd(a),
		queryNodeStateCmd(a),
		//queryValSetAtHeightCmd(),
		queryTxs(a),
		queryTx(a),
		lineBreakCommand(),
		queryClientCmd(a),
		queryClientsCmd(a),
		queryConnection(a),
		queryConnections(a),
		queryConnectionsUsingClient(a),
		queryChannel(a),
		queryChannels(a),
		queryConnectionChannels(a),
		queryPacketCommitment(a),
		lineBreakCommand(),
		queryIBCDenoms(a),
		queryBaseDenomFromIBCDenom(a),
	)

	return cmd
}

func queryIBCDenoms(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ibc-denoms [chain-id]",
		Short: "query denomination traces for a given network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query ibc-denoms ibc-0
$ %s q ibc-denoms ibc-0`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			h, err := chain.ChainProvider.QueryLatestHeight(cmd.Context())
			if err != nil {
				return err
			}

			res, err := chain.ChainProvider.QueryDenomTraces(cmd.Context(), 0, 100, h)
			if err != nil {
				return err
			}

			for _, d := range res {
				fmt.Fprintln(cmd.OutOrStdout(), d)
			}
			return nil
		},
	}

	return cmd
}

func queryBaseDenomFromIBCDenom(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "denom-trace [chain-id] [denom-hash]",
		Short: "query that retrieves the base denom from the IBC denomination trace",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query denom-trace osmosis-1 9BBA9A1C257E971E38C1422780CE6F0B0686F0A3085E2D61118D904BFE0F5F5E
$ %s q denom-trace osmosis-1 9BBA9A1C257E971E38C1422780CE6F0B0686F0A3085E2D61118D904BFE0F5F5E`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}
			res, err := c.ChainProvider.QueryDenomTrace(cmd.Context(), args[1])
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), res)
			return nil
		},
	}

	return cmd
}

func queryTx(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [chain-id] [tx-hash]",
		Short: "query for a transaction on a given network by transaction hash and chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query tx ibc-0 [tx-hash]
$ %s q tx ibc-0 A5DF8D272F1C451CFF92BA6C41942C4D29B5CF180279439ED6AB038282F956BE`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			txs, err := chain.ChainProvider.QueryTx(cmd.Context(), args[1])
			if err != nil {
				return err
			}

			out, err := json.Marshal(txs)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	return cmd
}

func queryTxs(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "txs [chain-id] [events]",
		Short: "query for transactions on a given network by chain ID and a set of transaction events",
		Long: strings.TrimSpace(`Search for a paginated list of transactions that match the given set of
events. Each event takes the form of '{eventType}.{eventAttribute}={value}' with multiple events
separated by '&'.

Please refer to each module's documentation for the full set of events to query for. Each module
documents its respective events under 'cosmos-sdk/x/{module}/spec/xx_events.md'.`,
		),
		Args: cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query txs ibc-0 "message.action=transfer" --offset 1 --limit 10
$ %s q txs ibc-0 "message.action=transfer"`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			offset, err := cmd.Flags().GetUint64(flags.FlagOffset)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flags.FlagLimit)
			if err != nil {
				return err
			}

			txs, err := chain.ChainProvider.QueryTxs(cmd.Context(), int(offset), int(limit), []string{args[1]})
			if err != nil {
				return err
			}

			out, err := json.Marshal(txs)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	return paginationFlags(a.Viper, cmd)
}

//func queryAccountCmd() *cobra.Command {
//	cmd := &cobra.Command{
//		Use:     "account [chain-id]",
//		Aliases: []string{"acc"},
//		Short:   "query the relayer's account on a given network by chain ID",
//		Args:    cobra.ExactArgs(1),
//		Example: strings.TrimSpace(fmt.Sprintf(`
//$ %s query account ibc-0
//$ %s q acc ibc-1`,
//			appName, appName,
//		)),
//		RunE: func(cmd *cobra.Command, args []string) error {
//			chain, err := config.Chains.Get(args[0])
//			if err != nil {
//				return err
//			}
//
//			addr := chain.ChainProvider.Address()
//			if addr == "" || len(addr) == 0 {
//				return fmt.Errorf("failed to retrieve address or address is invalid on chain %s \n", chain.ChainID())
//			}
//
//			// TODO circle back to this after clearing up errors
//			//{
//			//	"account":
//			//		{
//			//		"@type":"/cosmos.auth.v1beta1.BaseAccount",
//			//		"address":"cosmos1kn4tkqezr3c7zc43lsu5r4p2l2qqf4mp3hnjax",
//			//		"pub_key":{"@type":"/cosmos.crypto.secp256k1.PubKey",
//			//			"key":"A/YSSeVUdSJjogfcuhaR0rCUCMREOEdFTZR2cTTC7TPC"
//			//		},
//			//		"account_number":"380320",
//			//		"sequence":"2336"
//			//	}
//			//}
//
//			res, err := types.NewQueryClient(chain.CLIContext(0)).Account(
//				context.Background(),
//				&types.QueryAccountRequest{
//					Address: addr,
//				},
//			)
//			if err != nil {
//				return err
//			}
//
//			return chain.Print(res, false, false)
//		},
//	}
//
//	return cmd
//}

func queryBalanceCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "balance [chain-id] [[key-name]]",
		Aliases: []string{"bal"},
		Short:   "query the relayer's account balance on a given network by chain-ID",
		Args:    cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query balance ibc-0
$ %s query balance ibc-0 testkey`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			showDenoms, err := cmd.Flags().GetBool(flagIBCDenoms)
			if err != nil {
				return err
			}

			keyName := chain.ChainProvider.Key()
			if len(args) == 2 {
				keyName = args[1]
			}

			if !chain.ChainProvider.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			addr, err := chain.ChainProvider.ShowAddress(keyName)
			if err != nil {
				return err
			}

			coins, err := helpers.QueryBalance(cmd.Context(), chain, addr, showDenoms)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "address {%s} balance {%s} \n", addr, coins)
			return nil
		},
	}

	return ibcDenomFlags(a.Viper, cmd)
}

func queryHeaderCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "header [chain-id] [[height]]",
		Short: "query the header of a network by chain ID at a given height or the latest height",
		Args:  cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query header ibc-0
$ %s query header ibc-0 1400`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			var header ibcexported.Header
			switch len(args) {
			case 1:
				header, err = chain.ChainProvider.GetLightSignedHeaderAtHeight(cmd.Context(), 0)
				if err != nil {
					return err
				}

			case 2:
				header, err = helpers.QueryHeader(cmd.Context(), chain, args[1])
				if err != nil {
					return err
				}
			}

			s, err := chain.Sprint(header, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal header: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return cmd
}

// GetCmdQueryConsensusState defines the command to query the consensus state of
// the chain as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#query
func queryNodeStateCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-state [chain-id]",
		Short: "query the consensus state of a network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query node-state ibc-0
$ %s q node-state ibc-1`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			csRes, _, err := chain.ChainProvider.QueryConsensusState(cmd.Context(), 0)
			if err != nil {
				return err
			}

			s, err := chain.Sprint(csRes, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal consensus state: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return cmd
}

func queryClientCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client [chain-id] [client-id]",
		Short: "query the state of a light client on a network by chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client ibc-0 ibczeroclient
$ %s query client ibc-0 ibczeroclient --height 1205`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			if height == 0 {
				height, err = chain.ChainProvider.QueryLatestHeight(cmd.Context())
				if err != nil {
					return err
				}
			}

			if err = chain.AddPath(args[1], dcon); err != nil {
				return err
			}

			res, err := chain.ChainProvider.QueryClientStateResponse(cmd.Context(), height, chain.ClientID())
			if err != nil {
				return err
			}

			s, err := chain.Sprint(res, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal state: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return heightFlag(a.Viper, cmd)
}

func queryClientsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients [chain-id]",
		Aliases: []string{"clnts"},
		Short:   "query for all light client states on a network by chain ID",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query clients ibc-0
$ %s query clients ibc-2 --offset 2 --limit 30`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			// TODO fix pagination
			//pagereq, err := client.ReadPageRequest(cmd.Flags())
			//if err != nil {
			//	return err
			//}

			res, err := chain.ChainProvider.QueryClients(cmd.Context())
			if err != nil {
				return err
			}

			for _, client := range res {
				s, err := chain.Sprint(&client, false, false)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal state: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	flags.AddPaginationFlagsToCmd(cmd, "client states")
	return cmd
}

//func queryValSetAtHeightCmd() *cobra.Command {
//	cmd := &cobra.Command{
//		Use:   "valset [chain-id]",
//		Short: "query the validator set at particular height for a network by chain ID",
//		Args:  cobra.ExactArgs(1),
//		Example: strings.TrimSpace(fmt.Sprintf(`
//$ %s query valset ibc-0
//$ %s q valset ibc-1`,
//			appName, appName,
//		)),
//		RunE: func(cmd *cobra.Command, args []string) error {
//			chain, err := config.Chains.Get(args[0])
//			if err != nil {
//				return err
//			}
//
//			h, err := chain.ChainProvider.QueryLatestHeight()
//			if err != nil {
//				return err
//			}
//
//			version := clienttypes.ParseChainID(args[0])
//
//			res, err := chain.QueryValsetAtHeight(clienttypes.NewHeight(version, uint64(h)))
//			if err != nil {
//				return err
//			}
//
//			return chain.Print(res, false, false)
//		},
//	}
//
//	return cmd
//}

func queryConnections(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections [chain-id]",
		Aliases: []string{"conns"},
		Short:   "query for all connections on a network by chain ID",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connections ibc-0
$ %s query connections ibc-2 --offset 2 --limit 30
$ %s q conns ibc-1`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			// TODO fix pagination
			//pagereq, err := client.ReadPageRequest(cmd.Flags())
			//if err != nil {
			//	return err
			//}

			res, err := chain.ChainProvider.QueryConnections(cmd.Context())
			if err != nil {
				return err
			}

			for _, connection := range res {
				s, err := chain.Sprint(connection, false, false)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal connection: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	flags.AddPaginationFlagsToCmd(cmd, "connections on a network")
	return cmd
}

func queryConnectionsUsingClient(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client-connections [chain-id] [client-id]",
		Short: "query for all connections for a given client on a network by chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client-connections ibc-0 ibczeroclient
$ %s query client-connections ibc-0 ibczeroclient --height 1205`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(args[1], dcon); err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			if height == 0 {
				height, err = chain.ChainProvider.QueryLatestHeight(cmd.Context())
				if err != nil {
					return err
				}
			}

			res, err := chain.ChainProvider.QueryConnectionsUsingClient(cmd.Context(), height, chain.ClientID())
			if err != nil {
				return err
			}

			s, err := chain.Sprint(res, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal client connection state: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return heightFlag(a.Viper, cmd)
}

func queryConnection(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [chain-id] [connection-id]",
		Aliases: []string{"conn"},
		Short:   "query the connection state for a given connection id on a network by chain ID",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection ibc-0 ibconnection0
$ %s q conn ibc-1 ibconeconn`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(dcli, args[1]); err != nil {
				return err
			}

			height, err := chain.ChainProvider.QueryLatestHeight(cmd.Context())
			if err != nil {
				return err
			}

			res, err := chain.ChainProvider.QueryConnection(cmd.Context(), height, chain.ConnectionID())
			if err != nil {
				return err
			}

			s, err := chain.Sprint(res, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal connection state: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return cmd
}

func queryConnectionChannels(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection-channels [chain-id] [connection-id]",
		Short: "query all channels associated with a given connection on a network by chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection-channels ibc-0 ibcconnection1
$ %s query connection-channels ibc-2 ibcconnection2 --offset 2 --limit 30`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, args[1]); err != nil {
				return err
			}

			// TODO fix pagination
			//pagereq, err := client.ReadPageRequest(cmd.Flags())
			//if err != nil {
			//	return err
			//}

			chans, err := chain.ChainProvider.QueryConnectionChannels(cmd.Context(), 0, args[1])
			if err != nil {
				return err
			}

			for _, channel := range chans {
				s, err := chain.Sprint(channel, false, false)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal channel: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	flags.AddPaginationFlagsToCmd(cmd, "channels associated with a connection")
	return cmd
}

func queryChannel(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [chain-id] [channel-id] [port-id]",
		Short: "query a channel by channel and port ID on a network by chain ID",
		Args:  cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channel ibc-0 ibczerochannel transfer
$ %s query channel ibc-2 ibctwochannel transfer --height 1205`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			channelID := args[1]
			portID := args[2]
			if err = chain.AddPath(dcli, dcon); err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			if height == 0 {
				height, err = chain.ChainProvider.QueryLatestHeight(cmd.Context())
				if err != nil {
					return err
				}
			}

			res, err := chain.ChainProvider.QueryChannel(cmd.Context(), height, channelID, portID)
			if err != nil {
				return err
			}

			s, err := chain.Sprint(res, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal channel state: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return heightFlag(a.Viper, cmd)
}

func queryChannels(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels [chain-id]",
		Short: "query for all channels on a network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channels ibc-0
$ %s query channels ibc-2 --offset 2 --limit 30`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			// TODO fix pagination
			//pagereq, err := client.ReadPageRequest(cmd.Flags())
			//if err != nil {
			//	return err
			//}

			res, err := chain.ChainProvider.QueryChannels(cmd.Context())
			if err != nil {
				return err
			}

			for _, channel := range res {
				s, err := chain.Sprint(channel, false, false)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal channel: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	flags.AddPaginationFlagsToCmd(cmd, "channels on a network")
	return cmd
}

func queryPacketCommitment(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-commit [chain-id] [channel-id] [port-id] [seq]",
		Short: "query for the packet commitment given a sequence and channel ID on a network by chain ID",
		Args:  cobra.ExactArgs(4),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query packet-commit ibc-0 ibczerochannel transfer 32
$ %s q packet-commit ibc-1 ibconechannel transfer 31`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := a.Config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			channelID := args[1]
			portID := args[2]

			seq, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return err
			}

			res, err := chain.ChainProvider.QueryPacketCommitment(cmd.Context(), 0, channelID, portID, seq)
			if err != nil {
				return err
			}

			s, err := chain.Sprint(res, false, false)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal packet-commit state: %v\n", err)
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), s)
			return nil
		},
	}

	return cmd
}

func queryUnrelayedPackets(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unrelayed-packets [path] [src-channel-id]",
		Aliases: []string{"unrelayed-pkts"},
		Short:   "query for the packet sequence numbers that remain to be relayed on a given path",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s q unrelayed-packets demo-path channel-0
$ %s query unrelayed-packets demo-path channel-0
$ %s query unrelayed-pkts demo-path channel-0`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := a.Config.Paths.Get(args[0])
			if err != nil {
				return err
			}

			src, dst := path.Src.ChainID, path.Dst.ChainID

			c, err := a.Config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = c[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = c[dst].SetPath(path.Dst); err != nil {
				return err
			}

			channelID := args[1]
			channel, err := relayer.QueryChannel(cmd.Context(), c[src], channelID)
			if err != nil {
				return err
			}

			sp, err := relayer.UnrelayedSequences(cmd.Context(), c[src], c[dst], channel)
			if err != nil {
				return err
			}

			out, err := json.Marshal(sp)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	return cmd
}

func queryUnrelayedAcknowledgements(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unrelayed-acknowledgements [path] [src-channel-id]",
		Aliases: []string{"unrelayed-acks"},
		Short:   "query for unrelayed acknowledgement sequence numbers that remain to be relayed on a given path",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s q unrelayed-acknowledgements demo-path channel-0
$ %s query unrelayed-acknowledgements demo-path channel-0
$ %s query unrelayed-acks demo-path channel-0`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := a.Config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			src, dst := path.Src.ChainID, path.Dst.ChainID

			c, err := a.Config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = c[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = c[dst].SetPath(path.Dst); err != nil {
				return err
			}

			channelID := args[1]
			channel, err := relayer.QueryChannel(cmd.Context(), c[src], channelID)
			if err != nil {
				return err
			}

			sp, err := relayer.UnrelayedAcknowledgements(cmd.Context(), c[src], c[dst], channel)
			if err != nil {
				return err
			}

			out, err := json.Marshal(sp)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	return cmd
}
