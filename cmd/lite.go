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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	lite "github.com/tendermint/tendermint/lite2"
)

var (
	lc *lite.Client

	trustedHash    []byte
	trustedHeight  int64
	trustingPeriod time.Duration
	updatePeriod   time.Duration
	url            string
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

		if trustingPeriod > 0 {
			chain.TrustOptions.Period = trustingPeriod.String()
		}

		if len(trustedHash) > 0 && trustedHeight > 0 {
			chain.TrustOptions = relayer.TrustOptions{
				Period: chain.TrustOptions.Period,
				Height: trustedHeight,
				Hash:   trustedHash,
			}
		}

		// If no trusted hash was given, fetch it using the given url
		res, err := http.Get(url)
		if err != nil {
			return err
		}
		var opts relayer.TrustOptions
		bz, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return err
		}
		err = json.Unmarshal(bz, &opts)
		if err != nil {
			return err
		}
		tp := chain.TrustOptions.Period
		chain.TrustOptions = opts
		chain.TrustOptions.Period = tp

		lc, err = chain.NewLiteClient(filepath.Join(liteDir, chainID))
		if err != nil {
			return err
		}

		return nil
	},
}

var liteDeleteCmd = &cobra.Command{
	Use:   "delete [chain-id]",
	Short: "Delete an existing lite client for a configured chain, this will force new initialization during the next usage of the lite client.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// if lc.IsRunning() {
		// 	return errors.New("client is running")
		// }
		lc.Cleanup()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(liteCmd)
	liteCmd.AddCommand(liteStartCmd)
	liteCmd.AddCommand(liteDeleteCmd)

	liteStartCmd.Flags().DurationVar(&trustingPeriod, "trusting-period", 168*time.Hour, "Trusting period. Should be significantly less than the unbonding period")
	liteStartCmd.Flags().Int64Var(&trustedHeight, "trusted-height", 1, "Trusted header's height")
	liteStartCmd.Flags().BytesHexVar(&trustedHash, "trusted-hash", []byte{}, "Trusted header's hash")
	liteStartCmd.Flags().DurationVar(&updatePeriod, "update-period", 5*time.Second, "Period for checking for new blocks")
	liteStartCmd.Flags().StringVar(&url, "url", "", "Optional URL to fetch trusted-hash and trusted-height")
}
