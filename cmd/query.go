package cmd

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/auth"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	queryCmd.AddCommand(queryLatestHeightCmd)
	queryCmd.AddCommand(queryHeaderCmd)
	queryCmd.AddCommand(queryNodeStateCmd)
	queryCmd.AddCommand(queryClientCmd)
	queryCmd.AddCommand(queryClientsCmd())
	queryCmd.AddCommand(queryAccountCmd())
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

			fmt.Println(addr.String())

			acc, err := auth.NewAccountRetriever(chain).GetAccount(addr)
			if err != nil {
				return err
			}

			fmt.Println(acc.String())

			return nil
		},
	}
	return cmd
}

var queryLatestHeightCmd = &cobra.Command{
	Use:   "latest-height [chain-id]",
	Short: "Use configured RPC client to fetch the latest height from a configured chain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}

		h, err := chain.QueryLatestHeight()
		if err != nil {
			return err
		}

		fmt.Println(h)
		return nil
	},
}

var queryHeaderCmd = &cobra.Command{
	Use:   "header [chain-id] [height]",
	Short: "Use configured RPC client to fetch a header at a given height from a configured chain",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
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

		out, err := chain.Cdc.MarshalJSON(header)
		if err != nil {
			return err
		}

		fmt.Println(string(out))
		return nil
	},
}

// GetCmdQueryConsensusState defines the command to query the consensus state of
// the chain as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#query
var queryNodeStateCmd = &cobra.Command{
	Use:   "node-state [chain-id] [height]",
	Short: "Query the consensus state of a client at a given height, or at latest height if height is not passed",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}

		var height int64
		if len(args) == 2 {
			height, err = strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Println("invalid height, defaulting to latest:", args[1])
				height = 0
			}
		} else if len(args) == 1 {
			height, err = chain.QueryLatestHeight()
			if err != nil {
				return err
			}
		}

		csRes, err := chain.QueryConsensusState(height)
		if err != nil {
			return err
		}

		out, err := chain.Cdc.MarshalJSON(csRes)
		if err != nil {
			return err
		}

		fmt.Println(string(out))
		return nil
	},
}

var queryClientCmd = &cobra.Command{
	Use:   "client [chain-id] [client-id]",
	Short: "Query the client for a counterparty chain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}

		res, err := chain.QueryClientState(args[1])
		if err != nil {
			return err
		}

		out, err := chain.Cdc.MarshalJSON(res)
		if err != nil {
			return err
		}

		fmt.Println(string(out))
		return nil
	},
}

func queryClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [chain-id]",
		Short: "Query the client for a counterparty chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.c.GetChain(chainID)
			if err != nil {
				return err
			}

			page := viper.GetInt(flags.FlagPage)
			if page == 0 {
				page = 1
			}
			limit := viper.GetInt(flags.FlagLimit)
			if limit == 0 {
				limit = 100
			}

			res, err := chain.QueryClients(page, limit)
			if err != nil {
				return err
			}

			out, err := chain.Cdc.MarshalJSON(res)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().Int(flags.FlagPage, 1, "pagination page of light clients to to query for")
	cmd.Flags().Int(flags.FlagLimit, 100, "pagination limit of light clients to query for")
	return cmd
}
