package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/auth"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// queryCmd represents the chain command
func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query",
		Aliases: []string{"q"},
		Short:   "IBC Query Commands",
		Long:    "Commands to query IBC primatives, and other useful data on configured chains.",
	}

	cmd.AddCommand(
		queryFullPathCmd(),
		queryUnrelayed(),
		flags.LineBreak,
		queryAccountCmd(),
		queryBalanceCmd(),
		queryHeaderCmd(),
		queryNodeStateCmd(),
		queryTxs(),
		queryTx(),
		flags.LineBreak,
		queryClientCmd(),
		queryClientsCmd(),
		queryConnection(),
		queryConnections(),
		queryConnectionsUsingClient(),
		queryChannel(),
		queryChannels(),
		queryConnectionChannels(),
		queryNextSeqRecv(),
		queryPacketCommitment(),
		queryPacketAck(),
	)

	return cmd
}

func queryTx() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [chain-id] [tx-hash]",
		Short: "Query transaction by transaction hash",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			txs, err := chain.QueryTx(args[1])
			if err != nil {
				return err
			}

			return chain.Print(txs, false, false)
		},
	}
	return cmd
}

func queryTxs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "txs [chain-id] [events]",
		Short: "Query transactions by the events they produce",
		Long: strings.TrimSpace(`Search for transactions that match the exact given events where results are paginated. Each event 
takes the form of '{eventType}.{eventAttribute}={value}' with multiple events seperated by '&'. 
Please refer to each module's documentation for the full set of events to query for. Each module
documents its respective events under 'cosmos-sdk/x/{module}/spec/xx_events.md'.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			events, err := relayer.ParseEvents(args[1])
			if err != nil {
				return err
			}

			h, err := chain.UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			txs, err := chain.QueryTxs(h.GetHeight(), viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit), events)
			if err != nil {
				return err
			}

			return chain.Print(txs, false, false)
		},
	}
	return paginationFlags(cmd)
}

func queryAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account [chain-id]",
		Aliases: []string{"acc"},
		Short:   "Query the account data",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			addr, err := chain.GetAddress()
			if err != nil {
				return err
			}

			acc, err := auth.NewAccountRetriever(chain.Cdc, chain).GetAccount(addr)
			if err != nil {
				return err
			}

			return chain.Print(acc, false, false)
		},
	}
	return cmd
}

func queryBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "balance [chain-id] [[key-name]]",
		Aliases: []string{"bal"},
		Short:   "Query the account balances",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			var keyName string
			if len(args) == 2 {
				keyName = args[1]
			}

			coins, err := chain.QueryBalance(keyName)
			if err != nil {
				return err
			}

			var out string
			if jsn {
				byt, err := json.Marshal(coins)
				if err != nil {
					return err
				}
				out = string(byt)
			} else {
				out = coins.String()
			}

			fmt.Println(out)
			return nil
		},
	}
	return jsonFlag(cmd)
}

func queryHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "header [chain-id] [height]",
		Aliases: []string{"hdr"},
		Short:   "Query the header of a chain at a given height",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			var header *tmclient.Header

			switch len(args) {
			case 1:
				header, err = chain.QueryLatestHeader()
				if err != nil {
					return err
				}
			case 2:
				var height int64
				height, err = strconv.ParseInt(args[1], 10, 64) //convert to int64
				if err != nil {
					return err
				}

				if height == 0 {
					height, err = chain.QueryLatestHeight()
					if err != nil {
						return err
					}

					if height == -1 {
						return relayer.ErrLiteNotInitialized
					}
				}

				header, err = chain.QueryHeaderAtHeight(height)
				if err != nil {
					return err
				}

			}

			if viper.GetBool(flagFlags) {
				fmt.Printf("-x %x --height %d", header.SignedHeader.Hash(), header.Header.Height)
				return nil
			}

			return chain.Print(header, false, false)
		},
	}

	return flagsFlag(cmd)
}

// GetCmdQueryConsensusState defines the command to query the consensus state of
// the chain as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#query
func queryNodeStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-state [chain-id] [height]",
		Short: "Query the consensus state of a client at a given height",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			var height int64
			switch len(args) {
			case 1:
				height, err = chain.QueryLatestHeight()
				if err != nil {
					return err
				}
			case 2:
				height, err = strconv.ParseInt(args[1], 10, 64)
				if err != nil {
					fmt.Println("invalid height, defaulting to latest:", args[1])
					height = 0
				}
			}

			csRes, err := chain.QueryConsensusState(height)
			if err != nil {
				return err
			}

			return chain.Print(csRes, false, false)
		},
	}

	return cmd
}

func queryClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client [chain-id] [client-id]",
		Aliases: []string{"clnt"},
		Short:   "Query the state of a client given it's client-id",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(args[1], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			res, err := chain.QueryClientState()
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return cmd
}

func queryClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients [chain-id]",
		Aliases: []string{"clnts"},
		Short:   "Query for all client states on a chain",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryClients(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryConnections() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections [chain-id]",
		Aliases: []string{"conns"},
		Short:   "Query for all connections on a chain",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryConnections(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryConnectionsUsingClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client-connections [chain-id] [client-id]",
		Aliases: []string{"clnt-conns"},
		Short:   "Query for all connections on a given client",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(args[1], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnectionsUsingClient(height)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryConnection() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [chain-id] [connection-id]",
		Aliases: []string{"conn"},
		Short:   "Query the connection state for the given connection id",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(dcli, args[1], dcon, dpor, dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnection(height)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return cmd
}

func queryConnectionChannels() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection-channels [chain-id] [connection-id]",
		Aliases: []string{"conn-chans"},
		Short:   "Query any channels associated with a given connection",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}
			if err = chain.AddPath(dcli, args[1], dcha, dpor, dord); err != nil {
				return err
			}

			chans, err := chain.QueryConnectionChannels(args[1], viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.Print(chans, false, false)
		},
	}
	return paginationFlags(cmd)
}

func queryChannel() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel [chain-id] [channel-id] [port-id]",
		Aliases: []string{"chan"},
		Short:   "Query a channel given it's channel and port ids",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryChannel(height)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryChannels() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channels [chain-id]",
		Aliases: []string{"chans"},
		Short:   "Query for all channels on a chain",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryChannels(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryNextSeqRecv() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seq-send [chain-id] [channel-id] [port-id]",
		Short: "Query the next sequence send for a given channel",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryNextSeqRecv(height)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryPacketCommitment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-commit [chain-id] [channel-id] [port-id] [seq]",
		Short: "Query for the packet commitment given it's sequence and channel ids",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			seq, err := strconv.ParseInt(args[3], 10, 64)
			if err != nil {
				return err
			}

			res, err := chain.QueryPacketCommitment(height, seq)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryPacketAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-ack [chain-id] [channel-id] [port-id] [seq]",
		Short: "Query for the packet acknoledgement given it's sequence and channel ids",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			seq, err := strconv.ParseInt(args[3], 10, 64)
			if err != nil {
				return err
			}

			res, err := chain.QueryPacketAck(height, seq)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryUnrelayed() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unrelayed [path]",
		Aliases: []string{"queue"},
		Short:   "Query for the packet sequence numbers that remain to be relayed on a given path",
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

			sh, err := relayer.NewSyncHeaders(c[src], c[dst])
			if err != nil {
				return err
			}

			sp, err := relayer.UnrelayedSequences(c[src], c[dst], sh)
			if err != nil {
				return err
			}

			return c[src].Print(sp, false, false)
		},
	}

	return cmd
}

func queryFullPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "full-path [path-name]",
		Aliases: []string{"link", "connect", "path", "pth"},
		Short:   "Query for the status of clients, connections, channels and packets on a path",
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

			stat, err := relayer.QueryPathStatus(c[src], c[dst], path)
			if err != nil {
				return err
			}

			return c[src].Print(stat, false, false)
		},
	}

	return cmd
}
