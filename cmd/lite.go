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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	lite "github.com/tendermint/tendermint/lite2"
)

// liteCmd represents the lite command
var liteCmd = &cobra.Command{
	Use:   "lite",
	Short: "Commands to manage lite clients created by this relayer",
}

// This command just primarily for testing but may be useful otherwise. Ideally this is implemented with the
var liteStartCmd = &cobra.Command{
	Use:   "start [chain-id]",
	Short: "This command starts the auto updating relayer and logs when new headers are recieved",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]

		if !relayer.Exists(chainID, config.c) {
			return fmt.Errorf("chain with ID %s is not configured", chainID)
		}

		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		autoLite, err := chain.NewAutoLiteClient(fmt.Sprintf("%s/%s", liteDir, chainID), homePath, config.Global.LiteCacheSize)
		if err != nil {
			return err
		}

		defer autoLite.Stop()

		select {
		case h := <-autoLite.TrustedHeaders():
			fmt.Println("got header", h.Height)
			// Output: got header 3
		case err := <-autoLite.Errs():
			switch errors.Cause(err).(type) {
			case lite.ErrOldHeaderExpired:
				// reobtain trust height and hash
				return err
			default:
				// try with another full node
				return err
			}
		}

		return nil
	},
}

// TODO: Figure out arguements for initializing a lite client with a root of trust
var liteInitCmd = &cobra.Command{
	Use:   "init [chain-id] [hash] [height] [trust-period]",
	Short: "Create a new lite client for a configured chain, requires passing in a root of trust",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// TODO: Figure out arguements for initializing a lite client with a root of trust
var liteUpdateCmd = &cobra.Command{
	Use:   "update [chain-id] [hash] [height] [trust-period]",
	Short: "Update an existing lite client for a configured chain, requres passing in a root of trust",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var liteDeleteCmd = &cobra.Command{
	Use:   "delete [chain-id]",
	Short: "Delete an existing lite client for a configured chain, this will force new initialization during the next usage of the lite clien.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	rootCmd.AddCommand(liteCmd)
	liteCmd.AddCommand(liteStartCmd)
	liteCmd.AddCommand(liteInitCmd)
	liteCmd.AddCommand(liteDeleteCmd)
	liteCmd.AddCommand(liteUpdateCmd)
}
