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
	"github.com/cosmos/relayer/relayer"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
)

var initLiteCmd = &cobra.Command{
	Use:   "lite [chain-id]",
	Short: "Initiate the lite client by passing it a root of trust as a hash and height flag or as a url",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		trustedHeight, _ := cmd.Flags().GetInt64("height")
		trustedHash, _ := cmd.Flags().GetBytesHex("hash")
		url, _ := cmd.Flags().GetString("url")

		var trustOptions lite.TrustOptions
		if len(trustedHash) > 0 && trustedHeight > 0 {
			trustOptions = chain.GetTrustOptions(trustedHeight, trustedHash)
		} else if url != "" {
			res, err := http.Get(url)
			if err != nil {
				return err
			}
			bz, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return err
			}
			err = res.Body.Close()
			if err != nil {
				return err
			}

			err = json.Unmarshal(bz, &trustOptions)
			if err != nil {
				return err
			}
		} else {
			return errors.New("must provide either a height (--height) and a hash (--hash) or a url (--url)")
		}

		db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-%s", chain.ChainID), path.Join(chain.ChainDir, "db"))
		if err != nil {
			return err
		}
		defer func() {
			err := db.Close()
			if err != nil {
				panic(err)
			}
		}()

		_, err = chain.InitLiteClient(db, trustOptions)
		if err != nil {
			return err
		}
		return nil

	},
}

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
	initLiteCmd.Flags().Int64P("height", "h", -1, "Trusted header's height")
	initLiteCmd.Flags().BytesHexP("hash", "ha", []byte{}, "Trusted header's hash")
	initLiteCmd.Flags().StringP("url", "u", "", "Optional URL to fetch trusted-hash and trusted-height")
	rootCmd.AddCommand(initLiteCmd)
	rootCmd.AddCommand(headerCmd)
	rootCmd.AddCommand(latestHeightCmd)
}
