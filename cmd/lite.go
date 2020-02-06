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
	"github.com/spf13/cobra"
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
	liteCmd.AddCommand(headerCmd)
	liteCmd.AddCommand(latestHeightCmd)
	liteCmd.AddCommand(initLiteCmd)
	liteCmd.AddCommand(initLiteForceCmd)
}

var initLiteForceCmd = &cobra.Command{
	Use:   "init-force [chain-id]",
	Short: "Initalize the lite client by querying root of trust from configured node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain, err := config.c.GetChain(args[0])
		if err != nil {
			return err
		}

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
	},
}

var initLiteCmd = &cobra.Command{
	Use:   "init [chain-id] [hash] [height]",
	Short: "Initiate the lite client by passing it a root of trust as a hash and height",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain, err := config.c.GetChain(args[0])
		if err != nil {
			return err
		}

		height, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return err
		}

		var trustOptions = chain.TrustOptions(height, []byte(args[1]))

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
	_ = res.Body.Close()
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
