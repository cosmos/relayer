package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

func faucetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "faucet",
		Short: "commands for managing and using relayer faucets",
	}
	cmd.AddCommand(
		faucetStartCmd(),
		faucetRequestCmd(),
	)
	return cmd
}

func faucetRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "request [chain-id] [key-name]",
		Short: "request tokens from a relayer faucet",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			urlString, err := cmd.Flags().GetString(flagURL)
			if err != nil {
				return err
			}

			if urlString == "" {
				u, err := url.Parse(chain.RPCAddr)
				if err != nil {
					return err
				}

				host, _, err := net.SplitHostPort(u.Host)
				if err != nil {
					return err
				}

				urlString = fmt.Sprintf("%s://%s:%d", u.Scheme, host, 8000)
			}

			info, err := chain.Keybase.Get(args[1])
			if err != nil {
				return err
			}

			body, err := json.Marshal(relayer.FaucetRequest{Address: info.GetAddress().String(), ChainID: chain.ChainID})
			if err != nil {
				return err
			}

			resp, err := http.Post(urlString, "application/json", bytes.NewBuffer(body))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			fmt.Println(string(respBody))
			return nil
		},
	}
	return urlFlag(cmd)
}

func faucetStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [chain-id] [key-name] [amount]",
		Short: "listens on a port for requests for tokens",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}
			info, err := chain.Keybase.Get(args[1])
			if err != nil {
				return err
			}
			amount, err := sdk.ParseCoin(args[2])
			if err != nil {
				return err
			}
			listenAddr, err := cmd.Flags().GetString(flagListenAddr)
			if err != nil {
				return err
			}
			r := mux.NewRouter()
			r.HandleFunc("/", chain.FaucetHandler(info.GetAddress(), amount)).Methods("POST")
			srv := &http.Server{
				Handler:      r,
				Addr:         listenAddr,
				WriteTimeout: 15 * time.Second,
				ReadTimeout:  15 * time.Second,
			}
			chain.Log(fmt.Sprintf("Listening on %s for faucet requests...", listenAddr))
			return srv.ListenAndServe()
		},
	}
	return listenFlag(cmd)
}
