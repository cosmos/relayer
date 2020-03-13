package cmd

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
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
}

// queryCmd represents the chain command
var queryCmd = &cobra.Command{
	Use:     "query",
	Aliases: []string{"q"},
	Short:   "query functionality for configured chains",
}

func queryAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account [chain-id]",
		Short: "Use configured RPC client to fetch the account balance of the relayer account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.c.GetChain(args[0])
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

			return chain.PrintOutput(acc, cmd)
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
			chain, err := config.c.GetChain(args[0])
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

			return chain.PrintOutput(header, cmd)
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
			chain, err := config.c.GetChain(args[0])
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

			return chain.PrintOutput(csRes, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err = chain.PathClient(args[1]); err != nil {
				return chain.ErrCantSetPath(relayer.CLNTPATH, err)
			}

			res, err := chain.QueryClientState()
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryClients(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryConnections(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err := chain.PathConnection(args[1], "passesvalidation"); err != nil {
				return chain.ErrCantSetPath(relayer.CONNPATH, err)
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnectionsUsingClient(height)
			if err != nil {
				return err
			}

			return chain.PrintOutput(res.ConnectionPaths, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err := chain.PathConnection("passesvalidation", args[1]); err != nil {
				return chain.ErrCantSetPath(relayer.CONNPATH, err)
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnection(height)
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err = chain.PathChannel(args[1], args[2]); err != nil {
				return chain.ErrCantSetPath(relayer.CHANPATH, err)
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryChannel(height)
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			res, err := chain.QueryChannels(viper.GetInt(flags.FlagPage), viper.GetInt(flags.FlagLimit))
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err = chain.PathChannel(args[1], args[2]); err != nil {
				return chain.ErrCantSetPath(relayer.CHANPATH, err)
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryNextSeqRecv(height)
			if err != nil {
				return err
			}

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err = chain.PathChannel(args[1], args[2]); err != nil {
				return chain.ErrCantSetPath(relayer.CHANPATH, err)
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

			return chain.PrintOutput(res, cmd)
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
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if err = chain.PathChannel(args[1], args[2]); err != nil {
				return chain.ErrCantSetPath(relayer.CHANPATH, err)
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

			return chain.PrintOutput(res, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}
