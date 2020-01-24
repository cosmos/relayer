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
	"path/filepath"
	"strconv"
	"time"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

var (
	//	lcMap map[string]*lite.Client // chainID => client
	//
	//	trustedHash    []byte
	//	trustedHeight  int64
	//	trustingPeriod time.Duration
	updatePeriod time.Duration

//	url            string
)

// liteCmd represents the lite command
var liteCmd = &cobra.Command{
	Use:   "lite",
	Short: "Commands to manage lite clients created by this relayer",
}

var liteStartCmd = &cobra.Command{
	Use:   "start [chain-id]",
	Short: "This command starts the auto updating relayer and logs when new headers are recieved",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		updatePeriod, _ = cmd.Flags().GetDuration("update-period")
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		return chain.StartLiteClient(filepath.Join(liteDir, chainID), updatePeriod)
	},
}

var liteDeleteCmd = &cobra.Command{
	Use:   "delete [chain-id]",
	Short: "Delete an existing lite client for a configured chain, this will force new initialization during the next usage of the lite client.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		return chain.StopLiteClient()
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
		if len(args) == 1 {
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
			if height == -1 {
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
	liteCmd.AddCommand(liteStartCmd)
	liteCmd.AddCommand(liteDeleteCmd)
	liteCmd.AddCommand(liteGetHeaderCmd)
	liteCmd.AddCommand(liteGetValidatorsCmd)

	//liteStartCmd.Flags().DurationVar(&trustingPeriod, "trusting-period", 168*time.Hour, "Trusting period. Should be significantly less than the unbonding period")
	//liteStartCmd.Flags().Int64Var(&trustedHeight, "trusted-height", 1, "Trusted header's height")
	//liteStartCmd.Flags().BytesHexVar(&trustedHash, "trusted-hash", []byte{}, "Trusted header's hash")
	liteStartCmd.Flags().DurationVar(&updatePeriod, "update-period", 5*time.Second, "Period for checking for new blocks")
	//liteStartCmd.Flags().StringVar(&url, "url", "", "Optional URL to fetch trusted-hash and trusted-height")
}
