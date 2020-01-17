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

// liteCmd represents the lite command
var liteCreateCmd = &cobra.Command{
	Use:   "start [chain-id]",
	Short: "Commands to manage lite clients created by this relayer",
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

// liteCmd represents the lite command
var liteShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Commands to manage lite clients created by this relayer",
}

// liteCmd represents the lite command
var liteDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Commands to manage lite clients created by this relayer",
}

// liteCmd represents the lite command
var liteUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Commands to manage lite clients created by this relayer",
}

func init() {
	rootCmd.AddCommand(liteCmd)
	liteCmd.AddCommand(liteCreateCmd)
	liteCmd.AddCommand(liteShowCmd)
	liteCmd.AddCommand(liteDeleteCmd)
	liteCmd.AddCommand(liteUpdateCmd)
}
