/*
Package cmd includes relayer commands
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
	"net/http"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/helpers"
	"github.com/gorilla/mux"
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

			out, err := helpers.InitLight(chain, force, height, hash)
			if err != nil {
				return err
			}

			if out != "" {
				fmt.Println(out)
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

			out, err := helpers.UpdateLight(chain)
			if err != nil {
				return err
			}

			fmt.Println(out)
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
				header, err = helpers.GetLightHeader(chain, args[1])
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

// API Handlers

// GetLightHeader handles the route
func GetLightHeader(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	var header *tmclient.Header
	height := strings.TrimSpace(r.URL.Query().Get("height"))

	if len(height) == 0 {
		header, err = helpers.GetLightHeader(chain)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
	} else {
		header, err = helpers.GetLightHeader(chain, height)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, header, w)
}

// GetLightHeight handles the route
func GetLightHeight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := chain.GetLatestLightHeight()
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, height, w)
}

type postLightRequest struct {
	Force  bool   `json:"force"`
	Height int64  `json:"height"`
	Hash   string `json:"hash"`
}

// PostLight handles the route
func PostLight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	var request postLightRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	out, err := helpers.InitLight(chain, request.Force, request.Height, []byte(request.Hash))
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if out == "" {
		out = fmt.Sprintf("successfully created light client for %s", vars["chain-id"])
	}
	helpers.SuccessJSONResponse(http.StatusCreated, out, w)
}

// PutLight handles the route
func PutLight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	out, err := helpers.UpdateLight(chain)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	helpers.SuccessJSONResponse(http.StatusOK, out, w)
}

// DeleteLight handles the route
func DeleteLight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	err = chain.DeleteLightDB()
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	helpers.SuccessJSONResponse(http.StatusOK, fmt.Sprintf("Removed Light DB for %s", vars["chain-id"]), w)
}
