package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/auth"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tmtypes "github.com/tendermint/tendermint/types"
)

func init() {
	queryCmd.AddCommand(queryAccountCmd())
	queryCmd.AddCommand(queryHeaderCmd())
	queryCmd.AddCommand(queryNodeStateCmd())
	queryCmd.AddCommand(queryClientCmd())
	queryCmd.AddCommand(queryClientsCmd())
	queryCmd.AddCommand(queryConnection())
	queryCmd.AddCommand(queryConnections())
	queryCmd.AddCommand(queryConnectionsUsingClient())
	queryCmd.AddCommand(queryChannel())
	queryCmd.AddCommand(queryChannels())
	queryCmd.AddCommand(queryNextSeqRecv())
	queryCmd.AddCommand(queryPacketCommitment())
	queryCmd.AddCommand(queryPacketAck())
	queryCmd.AddCommand(queryTxs())
}

var eventFormat = "{eventType}.{eventAttribute}={value}"

// queryCmd represents the chain command
var queryCmd = &cobra.Command{
	Use:     "query",
	Aliases: []string{"q"},
	Short:   "query functionality for configured chains",
}

func queryTxs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "txs [chain-id] [events]",
		Short: "query transactions by the events they produce",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			events, err := parseEvents(args[1])
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

			return queryOutput(txs, chain, cmd)
		},
	}
	return outputFlags(paginationFlags(cmd))
}

func parseEvents(e string) ([]string, error) {
	eventsStr := strings.Trim(e, "'")
	var events []string
	if strings.Contains(eventsStr, "&") {
		events = strings.Split(eventsStr, "&")
	} else {
		events = append(events, eventsStr)
	}

	var tmEvents []string

	for _, event := range events {
		if !strings.Contains(event, "=") {
			return []string{}, fmt.Errorf("invalid event; event %s should be of the format: %s", event, eventFormat)
		} else if strings.Count(event, "=") > 1 {
			return []string{}, fmt.Errorf("invalid event; event %s should be of the format: %s", event, eventFormat)
		}

		tokens := strings.Split(event, "=")
		if tokens[0] == tmtypes.TxHeightKey {
			event = fmt.Sprintf("%s=%s", tokens[0], tokens[1])
		} else {
			event = fmt.Sprintf("%s='%s'", tokens[0], tokens[1])
		}

		tmEvents = append(tmEvents, event)
	}
	return tmEvents, nil
}

func queryAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account [chain-id]",
		Short: "Use configured RPC client to fetch the account balance of the relayer account",
		Args:  cobra.ExactArgs(1),
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

			return queryOutput(acc, chain, cmd)
		},
	}
	return outputFlags(cmd)
}

func queryHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "header [chain-id] [height]",
		Short: "Use configured RPC client to fetch a header at a given height from a configured chain",
		Args:  cobra.RangeArgs(1, 2),
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

			return queryOutput(header, chain, cmd)
		},
	}

	cmd.Flags().BoolP(flagFlags, "f", false, "pass flag to output the flags for lite init/update")
	viper.BindPFlag(flagFlags, cmd.Flags().Lookup(flagFlags))

	return outputFlags(cmd)
}

// GetCmdQueryConsensusState defines the command to query the consensus state of
// the chain as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#query
func queryNodeStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-state [chain-id] [height]",
		Short: "Query the consensus state of a client at a given height, or at latest height if height is not passed",
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

			return queryOutput(csRes, chain, cmd)
		},
	}

	return outputFlags(cmd)
}

func queryClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client [chain-id] [client-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(args[1], dcon, dcha, dpor); err != nil {
				return err
			}

			res, err := chain.QueryClientState()
			if err != nil {
				return err
			}

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(cmd)
}

func queryClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [chain-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryClients(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryConnections() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections [chain-id]",
		Short: "Query for all connections on a given chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryConnections(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryConnectionsUsingClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client-connections [chain-id] [client-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(args[1], dcon, dcha, dpor); err != nil {
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

			return queryOutput(res.ConnectionPaths, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryConnection() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection [chain-id] [connection-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(dcli, args[1], dcon, dpor); err != nil {
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

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryChannel() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [chain-id] [channel-id] [port-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2]); err != nil {
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

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryChannels() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels [chain-id]",
		Short: "Query for all channels on a given chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryChannels(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
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

			if err = chain.AddPath(dcli, dcon, args[1], args[2]); err != nil {
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

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryPacketCommitment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-commit [chain-id] [channel-id] [port-id] [seq]",
		Short: "Query the commitment for a given packet",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2]); err != nil {
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

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryPacketAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-ack [chain-id] [channel-id] [port-id] [seq]",
		Short: "Query the commitment for a given packet",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2]); err != nil {
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

			return queryOutput(res, chain, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryOutput(res interface{}, chain *relayer.Chain, cmd *cobra.Command) error {
	text, err := cmd.Flags().GetBool("text")
	if err != nil {
		panic(err)
	}

	indent, err := cmd.Flags().GetBool("indent")
	if err != nil {
		panic(err)
	}
	return chain.Print(res, text, indent)
}
