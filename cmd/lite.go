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
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/types"
	"strconv"
)

var headerCmd = &cobra.Command{
	Use: "header [chain-id] [height]",
	Short: "Get header from relayer. 0 returns last trusted header and " +
		"all others return the header at that height if stored",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}
		var header *types.SignedHeader
		if len(args) == 1 {
			header, err = chain.LatestHeader()
			if err != nil {
				return err
			}
			fmt.Println(header)
		}
		height, err := strconv.ParseInt(args[1], 10, 64) //convert to int64
		if err != nil {
			return err
		}
		header, err = chain.SignedHeaderAtHeight(height)
		if err != nil {
			return err
		}
		fmt.Println(header)
		return nil
	},
}

var latestHeightCmd = &cobra.Command{
	Use: "latestHeight [chain-id]",
	Short: "Get header from relayer. 0 returns last trusted header and " +
		"all others return the header at that height if stored",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}
		height, err := chain.LatestHeight()
		if err != nil {
			return err
		}
		fmt.Println(height)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(headerCmd)
	rootCmd.AddCommand(latestHeightCmd)
}
