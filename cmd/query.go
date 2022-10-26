package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
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
		queryBalanceCmd(a),
		queryHeaderCmd(a),
		queryNodeStateCmd(a),
		queryTxs(a),
		queryTx(a),
		lineBreakCommand(),
		queryClientCmd(a),
		queryClientsCmd(a),
		queryClientsExpiration(a),
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
		Use:   "ibc-denoms chain_name",
		Short: "query denomination traces for a given network by chain ID",
		Args:  withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query ibc-denoms ibc-0
$ %s q ibc-denoms ibc-0`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
		Use:   "denom-trace chain_id denom_hash",
		Short: "query that retrieves the base denom from the IBC denomination trace",
		Args:  withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query denom-trace osmosis 9BBA9A1C257E971E38C1422780CE6F0B0686F0A3085E2D61118D904BFE0F5F5E
$ %s q denom-trace osmosis 9BBA9A1C257E971E38C1422780CE6F0B0686F0A3085E2D61118D904BFE0F5F5E`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
		Use:   "tx chain_name tx_hash",
		Short: "query for a transaction on a given network by transaction hash and chain ID",
		Args:  withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query tx ibc-0 [tx-hash]
$ %s q tx ibc-0 A5DF8D272F1C451CFF92BA6C41942C4D29B5CF180279439ED6AB038282F956BE`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
		Use:   "txs chain_name events",
		Short: "query for transactions on a given network by chain ID and a set of transaction events",
		Long: strings.TrimSpace(`Search for a paginated list of transactions that match the given set of
events. Each event takes the form of '{eventType}.{eventAttribute}={value}' with multiple events
separated by '&'.

Please refer to each module's documentation for the full set of events to query for. Each module
documents its respective events under 'cosmos-sdk/x/{module}/spec/xx_events.md'.`,
		),
		Args: withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query txs ibc-0 "message.action=transfer" --page 1 --limit 10
$ %s q txs ibc-0 "message.action=transfer"`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			page, err := cmd.Flags().GetUint64(flagPage)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flagLimit)
			if err != nil {
				return err
			}

			txs, err := chain.ChainProvider.QueryTxs(cmd.Context(), int(page), int(limit), []string{args[1]})
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

	return paginationFlags(a.Viper, cmd, "txs")
}

func queryBalanceCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "balance chain_name [key_name]",
		Aliases: []string{"bal"},
		Short:   "query the relayer's account balance on a given network by chain-ID",
		Args:    withUsage(cobra.RangeArgs(1, 2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query balance ibc-0
$ %s query balance ibc-0 testkey`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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

			coins, err := relayer.QueryBalance(cmd.Context(), chain, addr, showDenoms)
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
		Use:   "header chain_name [height]",
		Short: "query the header of a network by chain ID at a given height or the latest height",
		Args:  withUsage(cobra.RangeArgs(1, 2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query header ibc-0
$ %s query header ibc-0 1400`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			var height int64
			switch len(args) {
			case 1:
				var err error
				height, err = chain.ChainProvider.QueryLatestHeight(cmd.Context())
				if err != nil {
					return err
				}

			case 2:
				var err error
				height, err = strconv.ParseInt(args[1], 10, 64)
				if err != nil {
					return err
				}
			}

			header, err := chain.ChainProvider.QueryIBCHeader(cmd.Context(), height)
			if err != nil {
				return err
			}

			s, err := json.Marshal(header)
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
		Use:   "node-state chain_name",
		Short: "query the consensus state of a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query node-state ibc-0
$ %s q node-state ibc-1`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			height, err := chain.ChainProvider.QueryLatestHeight(cmd.Context())
			if err != nil {
				return err
			}

			csRes, _, err := chain.ChainProvider.QueryConsensusState(cmd.Context(), height)
			if err != nil {
				return err
			}

			s, err := chain.ChainProvider.Sprint(csRes)
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
		Use:   "client chain_name client_id",
		Short: "query the state of a light client on a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client osmosis 07-tendermint-259
$ %s query client ibc-0 ibczeroclient --height 1205`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			height, err := cmd.Flags().GetInt64(flagHeight)
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

			s, err := chain.ChainProvider.Sprint(res)
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
		Use:     "clients chain_name",
		Aliases: []string{"clnts"},
		Short:   "query for all light client states on a network by chain ID",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query clients osmosis
$ %s query clients ibc-2 --offset 2 --limit 30`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
				s, err := chain.ChainProvider.Sprint(&client)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal state: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	return paginationFlags(a.Viper, cmd, "client states")
}

func queryConnections(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections chain_id",
		Aliases: []string{"conns"},
		Short:   "query for all connections on a network by chain ID",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connections ibc-0
$ %s query connections ibc-2 --offset 2 --limit 30
$ %s q conns ibc-1`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
				s, err := chain.ChainProvider.Sprint(connection)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal connection: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	return paginationFlags(a.Viper, cmd, "connections on a network")
}

func queryConnectionsUsingClient(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client-connections chain_name client_id",
		Short: "query for all connections for a given client on a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client-connections ibc-0 ibczeroclient
$ %s query client-connections ibc-0 ibczeroclient --height 1205`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			//TODO - Add pagination

			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			if err := chain.AddPath(args[1], dcon); err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flagHeight)
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

			s, err := chain.ChainProvider.Sprint(res)
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
		Use:     "connection chain_name connection_id",
		Aliases: []string{"conn"},
		Short:   "query the connection state for a given connection id on a network by chain ID",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection ibc-0 ibconnection0
$ %s q conn ibc-1 ibconeconn`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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

			s, err := chain.ChainProvider.Sprint(res)
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
		Use:   "connection-channels chain_name connection_id",
		Short: "query all channels associated with a given connection on a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection-channels ibc-0 ibcconnection1
$ %s query connection-channels ibc-2 ibcconnection2 --offset 2 --limit 30`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			if err := chain.AddPath(dcli, args[1]); err != nil {
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
				s, err := chain.ChainProvider.Sprint(channel)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal channel: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	return paginationFlags(a.Viper, cmd, "channels associated with a connection")
}

func queryChannel(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel chain_name channel_id port_id",
		Short: "query a channel by channel and port ID on a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channel ibc-0 ibczerochannel transfer
$ %s query channel ibc-2 ibctwochannel transfer --height 1205`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			channelID := args[1]
			portID := args[2]
			if err := chain.AddPath(dcli, dcon); err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flagHeight)
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

			s, err := chain.ChainProvider.Sprint(res)
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
		Use:   "channels chain_name",
		Short: "query for all channels on a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channels ibc-0
$ %s query channels ibc-2 --offset 2 --limit 30`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
				s, err := chain.ChainProvider.Sprint(channel)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to marshal channel: %v\n", err)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), s)
			}

			return nil
		},
	}

	return paginationFlags(a.Viper, cmd, "channels on a network")
}

func queryPacketCommitment(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-commit chain_name channel_id port_id seq",
		Short: "query for the packet commitment given a sequence and channel ID on a network by chain ID",
		Args:  withUsage(cobra.ExactArgs(4)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query packet-commit ibc-0 ibczerochannel transfer 32
$ %s q packet-commit ibc-1 ibconechannel transfer 31`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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

			s, err := chain.ChainProvider.Sprint(res)
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
		Use:     "unrelayed-packets path src_channel_id",
		Aliases: []string{"unrelayed-pkts"},
		Short:   "query for the packet sequence numbers that remain to be relayed on a given path",
		Args:    withUsage(cobra.ExactArgs(2)),
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

			sp := relayer.UnrelayedSequences(cmd.Context(), c[src], c[dst], channel)

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
		Use:     "unrelayed-acknowledgements path src_channel_id",
		Aliases: []string{"unrelayed-acks"},
		Short:   "query for unrelayed acknowledgement sequence numbers that remain to be relayed on a given path",
		Args:    withUsage(cobra.ExactArgs(2)),
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

			sp := relayer.UnrelayedAcknowledgements(cmd.Context(), c[src], c[dst], channel)

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

func queryClientsExpiration(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients-expiration path",
		Short: "query for light clients expiration date",
		Args:  withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query clients-expiration demo-path`,
			appName,
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

			srcExpiration, err := relayer.QueryClientExpiration(cmd.Context(), c[src], c[dst])
			if err != nil {
				return err
			}
			dstExpiration, err := relayer.QueryClientExpiration(cmd.Context(), c[dst], c[src])
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), relayer.SPrintClientExpiration(c[src], srcExpiration))
			fmt.Fprintf(cmd.OutOrStdout(), relayer.SPrintClientExpiration(c[dst], dstExpiration))

			return nil
		},
	}

	return cmd
}
