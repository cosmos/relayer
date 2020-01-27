/*
Copyright © 2020 Jack Zampolin <jack.zampolin@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// liteCmd represents the lite command
var liteCmd = &cobra.Command{
	Use:   "lite",
	Short: "Commands to manage lite clients created by this relayer",
}

var liteResetCmd = &cobra.Command{
	Use:   "reset [chain-id]",
	Short: "This command resets the auto updating client. Use a trusted has and header or a trusted url",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var liteDeleteCmd = &cobra.Command{
	Use:   "delete [chain-id]",
	Short: "Delete an existing lite client for a configured chain, this will force new initialization during the next usage of the lite client.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// Only checks headers in the trusted store - doesn't validate headers from the trusted node that don't yet exist in the lite client itself
var liteGetHeaderCmd = &cobra.Command{
	Use: "header [chain-id] [height]",
	Short: "Get header from lite client. height at -1 returns first trusted header, 0 returns last trusted header and " +
		"all others return the header at that height if stored in the lite client",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}
		if chain.LiteClient != nil {
			return fmt.Errorf("lite client is not running on this chain")
		}
		if len(args) == 1 { //get the latest trusted header
			header, err := chain.LiteClient.TrustedHeader(0, time.Now())
			if err != nil {
				return err
			}
			fmt.Print(header) // output
		} else {
			height, err := strconv.ParseInt(args[1], 10, 64) //convert to int64
			if err != nil {
				return err
			}
			if height == -1 { // get the first trusted header
				height, err = chain.LiteClient.FirstTrustedHeight()
				if err != nil {
					return err
				}
			}
			header, err := chain.LiteClient.TrustedHeader(height, time.Now())
			if err != nil {
				return err
			}
			fmt.Print(header) // output
		}
		return nil
	},
}

var liteGetValidatorsCmd = &cobra.Command{
	Use: "validators [chain-id] [height]",
	Short: "Get the validator set from the lite client of the specified height. No height specified takes the last " +
		"trusted validator set. -1 as height takes the first and 0 also takes the last",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}
		if chain.LiteClient != nil {
			return fmt.Errorf("lite client is not running on this chain")
		}
		lc := chain.LiteClient
		if len(args) == 1 { // return latest validator set
			lastTrustedHeight, err := lc.LastTrustedHeight()
			if err != nil {
				return err
			}
			vals, err := lc.TrustedValidatorSet(lastTrustedHeight, time.Now())
			if err != nil {
				return err
			}
			fmt.Print(vals)
		} else {
			height, err := strconv.ParseInt(args[1], 10, 64) //convert to int64
			if err != nil {
				return err
			}
			if height == -1 {
				height, err = lc.FirstTrustedHeight()
				if err != nil {
					return err
				}
			}
			vals, err := lc.TrustedValidatorSet(height, time.Now())
			if err != nil {
				return err
			}
			fmt.Print(vals) //output
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(liteCmd)
	liteCmd.AddCommand(liteResetCmd)
	liteCmd.AddCommand(liteDeleteCmd)
	liteCmd.AddCommand(liteGetHeaderCmd)
	liteCmd.AddCommand(liteGetValidatorsCmd)
}
