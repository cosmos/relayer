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
	"path/filepath"
	"time"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
// NOTE: This is basically psuedocode
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "starts the relayer using the configured chains and strategy",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := time.ParseDuration(config.Global.Timeout)
		if err != nil {
			return err
		}

		// initiate a lite client for each chain declared in the config file
		for _, chain := range config.c {
			err := chain.StartLiteClient(filepath.Join(liteDir, chain.ChainID))
			if err != nil {
				return err
			}
		}

		// The relayer will continuously run the strategy declared in the config file
		for {
			err = relayer.Relay(config.Global.Strategy, config.c)
			time.Sleep(d)
			if err != nil {
				return err
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
