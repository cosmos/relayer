package cmd

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
)

func init() {
	queryCmd.AddCommand(queryHeaderCmd())
	queryCmd.AddCommand(queryNodeStateCmd())
	queryCmd.AddCommand(queryClientCmd())
	queryCmd.AddCommand(queryClientsCmd())
	queryCmd.AddCommand(queryAccountCmd())
	queryCmd.AddCommand(queryConnection())
	queryCmd.AddCommand(queryConnectionsUsingClient())
	queryCmd.AddCommand(queryChannel())
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

			acc, err := auth.NewAccountRetriever(chain).GetAccount(addr)
			if err != nil {
				return err
			}

			return PrintOutput(acc, cmd)
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

			return PrintOutput(header, cmd)
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

			return PrintOutput(csRes, cmd)
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

			res, err := chain.QueryClientState(args[1])
			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
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

			return PrintOutput(res, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}

func queryConnectionsUsingClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections [chain-id] [client-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnectionsUsingClient(args[1], height)
			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
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

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnection(args[1], height)
			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
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

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryChannel(args[1], args[2], height)
			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
		},
	}

	return outputFlags(paginationFlags(cmd))
}
