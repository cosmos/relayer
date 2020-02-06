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
	"strconv"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	lite "github.com/tendermint/tendermint/lite2"
)

var (
	flagHeight = "height"
	flagHash   = "hash"
	flagURL    = "url"
	flagForce  = "force"
)

// chainCmd represents the keys command
var liteCmd = &cobra.Command{
	Use:   "lite",
	Short: "basic functionality for managing the lite clients",
}

func init() {
	initLiteCmd.Flags().Int64P(flagHeight, "h", -1, "Trusted header's height")
	initLiteCmd.Flags().BytesHexP(flagHash, "ha", []byte{}, "Trusted header's hash")
	initLiteCmd.Flags().StringP(flagURL, "u", "", "Optional URL to fetch trusted-hash and trusted-height")
	initLiteCmd.Flags().BoolP(flagForce, "f", false, "Option to force pulling root of trust from configured url")
	liteCmd.AddCommand(headerCmd)
	liteCmd.AddCommand(latestHeightCmd)
	liteCmd.AddCommand(initLiteCmd)
}

var initLiteCmd = &cobra.Command{
	Use:   "init [chain-id]",
	Short: "Initiate the lite client by passing it a root of trust as a hash and height flag or as a url",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain, err := config.c.GetChain(args[0])
		if err != nil {
			return err
		}

		// if we are forcing trust from the configured node, init the client
		if viper.GetBool(flagForce) {
			// establish db connection
			db, df, err := chain.NewLiteDB()
			if err != nil {
				return err
			}
			defer df()

			// initialize the lite client database by querying the configured node
			_, err = chain.TrustNodeInitClient(db)
			if err != nil {
				return err
			}

			return nil
		}

		trustedHeight, _ := cmd.Flags().GetInt64("height")
		trustedHash, _ := cmd.Flags().GetBytesHex("hash")
		url, _ := cmd.Flags().GetString("url")

		var trustOptions lite.TrustOptions
		if len(trustedHash) > 0 && trustedHeight > 0 {
			trustOptions = chain.TrustOptions(trustedHeight, trustedHash)
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

		db, df, err := chain.NewLiteDB()
		if err != nil {
			return err
		}
		defer df()

		_, err = chain.InitLiteClient(db, trustOptions)
		if err != nil {
			return err
		}
		return nil

	},
}

var headerCmd = &cobra.Command{
	Use: "header [chain-id] [height]",
	Short: "Get header from the database. 0 returns last trusted header and " +
		"all others return the header at that height if stored",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}
		var header *tmclient.Header
		if len(args) == 1 {
			header, err = chain.GetLatestLiteHeader()
			if err != nil {
				return err
			}
			fmt.Println(header)
		}
		height, err := strconv.ParseInt(args[1], 10, 64) //convert to int64
		if err != nil {
			return err
		}
		header, err = chain.GetLiteSignedHeaderAtHeight(height)
		if err != nil {
			return err
		}
		fmt.Println(header)
		return nil
	},
}

var latestHeightCmd = &cobra.Command{
	Use: "latest-height [chain-id]",
	Short: "Get header from relayer database. 0 returns last trusted header and " +
		"all others return the header at that height if stored",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		chain, err := config.c.GetChain(chainID)
		if err != nil {
			return err
		}

		// Get stored height
		height, err := chain.GetLatestLiteHeader()
		if err != nil {
			return err
		}

		fmt.Println(height)
		return nil
	},
}

func queryTrustOptions(url string) (out lite.TrustOptions, err error) {
	// fetch from URL
	res, err := http.Get(url)
	if err != nil {
		return
	}

	// read in the res body
	bz, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	// close the response body
	err = res.Body.Close()
	if err != nil {
		return
	}

	// unmarshal the data into the trust options hash
	err = json.Unmarshal(bz, &out)
	if err != nil {
		return
	}

	return
}
