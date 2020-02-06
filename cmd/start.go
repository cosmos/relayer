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

		for _, chain := range config.c {
			// NOTE: this is now hardcoded to a once every 5 seconds update,
			// this should be made configurable
			go chain.StartUpdatingLiteClient(time.Duration(5 * time.Second))

			// TODO: Figure out how/when to stop
		}

		// The relayer will continuously run the strategy declared in the config file
		ticker := time.NewTicker(d)
		for ; true; <-ticker.C {
			err = relayer.Relay(config.Global.Strategy, config.c)
			if err != nil {
				// TODO: This should have a better error handling strategy
				// Ideally some errors are just logged while others halt the process
				fmt.Println(err)
			}
		}

		return nil
	},
}
