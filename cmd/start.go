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
	"github.com/spf13/cobra"
)

// startCmd represents the start command
// NOTE: This is basically psuedocode
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start [src-chain-id] [dst-chain-id] ",
		Aliases: []string{"st"},
		Short:   "Start runs the relayer strategy associated with a path between the two chains",
		Args:    cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			pth, err := cmd.Flags().GetString(flagPath)
			if err != nil {
				return err
			}

			path, err := setPathsFromArgs(chains[src], chains[dst], pth)
			if err != nil {
				return err
			}

			strategy, err := path.GetStrategy()
			if err != nil {
				return nil
			}

			return strategy.Run(chains[src], chains[dst])
		},
	}
	return pathFlag(cmd)
}
