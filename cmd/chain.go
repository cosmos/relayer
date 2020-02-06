package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	chainCmd.AddCommand(chainLatestHeaderCmd)
	chainCmd.AddCommand(chainLatestHeightCmd)
	chainCmd.AddCommand(chainHeaderCmd)
	chainCmd.AddCommand(chainNodeStateCmd)
}

// chainCmd represents the chain command
var chainCmd = &cobra.Command{
	Use:   "chain",
	Short: "query functionality for configured chains",
}

var chainLatestHeaderCmd = &cobra.Command{
	Use:   "latest-header [chain-id]",
	Short: "Use configured RPC client to fetch the latest header from a configured chain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}

		h, err := chain.QueryLatestHeader()
		if err != nil {
			return err
		}

		fmt.Println(h)
		return nil
	},
}

var chainLatestHeightCmd = &cobra.Command{
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

var chainHeaderCmd = &cobra.Command{
	Use:   "header [chain-id] [height]",
	Short: "Use configured RPC client to fetch a header at a given height from a configured chain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}

		height, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}

		h, err := chain.QueryHeaderAtHeight(height)
		if err != nil {
			return err
		}

		fmt.Println(h)
		return nil
	},
}

// GetCmdQueryConsensusState defines the command to query the consensus state of
// the chain as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#query
var chainNodeStateCmd = &cobra.Command{
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
