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

	"github.com/cosmos/cosmos-sdk/client/flags"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"github.com/ovrclk/relayer/relayer"
	"github.com/spf13/cobra"
)

// chainCmd represents the keys command
func liteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lite",
		Aliases: []string{"l"},
		Short:   "manage lite clients held by the relayer for each chain",
	}

	cmd.AddCommand(liteHeaderCmd())
	cmd.AddCommand(initLiteCmd())
	cmd.AddCommand(updateLiteCmd())
	cmd.AddCommand(deleteLiteCmd())

	return cmd
}

func initLiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init [chain-id]",
		Aliases: []string{"i"},
		Short:   "Initiate the light client",
		Long: `Initiate the light client by:
	1. passing it a root of trust as a --hash/-x and --height
	2. Use --force/-f to initialize from the configured node`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			db, df, err := chain.NewLiteDB()
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
				_, err = chain.LiteClientWithoutTrust(db)
				if err != nil {
					return err
				}
			case height > 0 && len(hash) > 0: // height and hash are given
				_, err = chain.LiteClientWithTrust(db, chain.TrustOptions(height, hash))
				if err != nil {
					return wrapInitFailed(err)
				}
			default: // return error
				return errInitWrongFlags
			}

			return nil
		},
	}

	return forceFlag(liteFlags(cmd))
}

func updateLiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update [chain-id]",
		Aliases: []string{"u"},
		Short:   "Update the light client by providing a new root of trust",
		Long: `Update the light client by
	1. providing a new root of trust as a --hash/-x and --height
	2. via --url/-u where trust options can be found
	3. updating from the configured node by passing no flags`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
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
			case height > 0 && len(hash) > 0: // height and hash are given
				db, df, err := chain.NewLiteDB()
				if err != nil {
					return err
				}
				defer df()

				_, err = chain.LiteClientWithTrust(db, chain.TrustOptions(height, hash))
				if err != nil {
					return wrapInitFailed(err)
				}
			default: // nothing is given => update existing client
				// NOTE: "Update the light client by providing a new root of trust"
				// does not mention this at all. I mean that we can update existing
				// client by calling "update [chain-id]".
				//
				// Since first two conditions essentially repeat initLiteCmd above, I
				// think we should remove first two conditions here and just make
				// updateLiteCmd only about updating the light client to latest header
				// (i.e. not mix responsibilities).
				_, err = chain.UpdateLiteWithHeader()
				if err != nil {
					return wrapIncorrectHeader(err)
				}
			}

			return nil
		},
	}

	return liteFlags(cmd)
}

func liteHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "header [chain-id] [height]",
		Aliases: []string{"hdr"},
		Short: "Get header from the database. 0 returns last trusted header and " +
			"all others return the header at that height if stored",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.Chains.Get(chainID)
			if err != nil {
				return err
			}

			var header *tmclient.Header

			switch len(args) {
			case 1:
				header, err = chain.GetLatestLiteHeader()
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
					height, err = chain.GetLatestLiteHeight()
					if err != nil {
						return err
					}

					if height == -1 {
						return relayer.ErrLiteNotInitialized
					}
				}

				header, err = chain.GetLiteSignedHeaderAtHeight(height)
				if err != nil {
					return err
				}

			}

			out, err := chain.Cdc.MarshalJSON(header)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func deleteLiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [chain-id]",
		Aliases: []string{"d"},
		Short:   "wipe the lite client database, forcing re-initialzation on the next run",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.Chains.Get(chainID)
			if err != nil {
				return err
			}

			err = chain.DeleteLiteDB()
			if err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}
