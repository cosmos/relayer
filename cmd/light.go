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
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// chainCmd represents the keys command
func lightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "light",
		Aliases: []string{"l"},
		Short:   "manage light clients held by the relayer for each chain",
	}

	cmd.AddCommand(lightHeaderCmd())
	cmd.AddCommand(initLightCmd())
	cmd.AddCommand(updateLightCmd())
	cmd.AddCommand(deleteLightCmd())

	return cmd
}

func initLightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init [chain-id]",
		Aliases: []string{"i"},
		Short:   "Initiate the light client",
		Long: `Initiate the light client by:
	1. passing it a root of trust as a --hash/-x and --height
	2. Use --force/-f to initialize from the configured node`,
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s light init ibc-0 --force
$ %s light init ibc-1 --height 1406 --hash <hash>
$ %s l i ibc-2 --force`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			db, df, err := chain.NewLightDB()
			if err != nil {
				return err
			}
			defer df()

			force, err := cmd.Flags().GetBool(flagForce)
			if err != nil {
				return err
			}
			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}
			hash, err := cmd.Flags().GetBytesHex(flagHash)
			if err != nil {
				return err
			}

			switch {
			case force: // force initialization from trusted node
				_, err := chain.LightClientWithoutTrust(db)
				if err != nil {
					return err
				}
				fmt.Printf("successfully created light client for %s by trusting endpoint %s...\n", chain.ChainID, chain.RPCAddr)
			case height > 0 && len(hash) > 0: // height and hash are given
				_, err = chain.LightClientWithTrust(db, chain.TrustOptions(height, hash))
				if err != nil {
					return wrapInitFailed(err)
				}
			default: // return error
				return errInitWrongFlags
			}

			return nil
		},
	}

	return forceFlag(lightFlags(cmd))
}

func updateLightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update [chain-id]",
		Aliases: []string{"u"},
		Short:   "Update the light client to latest header from configured node",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s light update ibc-0
$ %s l u ibc-1`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			bh, err := chain.GetLatestLightHeader()
			if err != nil {
				return err
			}

			ah, err := chain.UpdateLightWithHeader()
			if err != nil {
				return err
			}

			fmt.Printf("Updated light client for %s from height %d -> height %d\n", args[0], bh.Header.Height, ah.Header.Height)
			return nil
		},
	}

	return cmd
}

func lightHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "header [chain-id] [[height]]",
		Aliases: []string{"hdr"},
		Short:   "Get a header from the light client database",
		Long: "Get a header from the light client database. 0 returns last" +
			"trusted header and all others return the header at that height if stored",
		Args: cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s light header ibc-0
$ %s light header ibc-1 1400
$ %s l hdr ibc-2`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.Chains.Get(chainID)
			if err != nil {
				return err
			}

			var header *tmclient.Header

			switch len(args) {
			case 1:
				header, err = chain.GetLatestLightHeader()
				if err != nil {
					return err
				}
			case 2:
				var height int64
				height, err = strconv.ParseInt(args[1], 10, 64) //convert to int64
				if err != nil {
					return err
				}

				if height == 0 {
					height, err = chain.GetLatestLightHeight()
					if err != nil {
						return err
					}

					if height == -1 {
						return relayer.ErrLightNotInitialized
					}
				}

				header, err = chain.GetLightSignedHeaderAtHeight(height)
				if err != nil {
					return err
				}

			}

			out, err := chain.Encoding.Marshaler.MarshalJSON(header)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func deleteLightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [chain-id]",
		Aliases: []string{"d"},
		Short:   "wipe the light client database, forcing re-initialzation on the next run",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s light delete ibc-0
$ %s l d ibc-2`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.Chains.Get(chainID)
			if err != nil {
				return err
			}

			err = chain.DeleteLightDB()
			if err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}
