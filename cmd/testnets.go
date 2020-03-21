package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

func testnetsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "testnets",
		Aliases: []string{"tst"},
		Short:   "commands for managing and using relayer faucets",
	}
	cmd.AddCommand(
		faucetStartCmd(),
		faucetRequestCmd(),
		gaiaServiceCmd(),
		faucetService(),
		rlyService(),
	)
	return cmd
}

func gaiaServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gaia-service [user] [home]",
		Aliases: []string{"gaia-svc"},
		Short:   "gaia-service returns a sample gaiad service file",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`[Unit]
Description=gaiad
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/gaiad start --pruning=nothing
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], args[1], args[1])
		},
	}
	return cmd
}

func faucetService() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "faucet-service [user] [home] [chain-id] [key-name] [amount]",
		Aliases: []string{"faucet-svc"},
		Short:   "faucet-service returns a sample faucet service file",
		Args:    cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[2])
			if err != nil {
				return err
			}
			_, err = chain.Keybase.Get(args[3])
			if err != nil {
				return err
			}
			_, err = sdk.ParseCoin(args[4])
			if err != nil {
				return err
			}
			fmt.Printf(`[Unit]
Description=faucet
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/rly testnets faucet %s %s %s
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], args[1], args[1], args[2], args[3], args[4])
			return nil
		},
	}
	return cmd
}

func rlyService() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relayer-service [path-name]",
		Aliases: []string{"rly-svc"},
		Short:   "relayer-service returns a service file for the relayer to relay over an individual path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, home := os.Getenv("USER"), os.Getenv("HOME")
			if user == "" || home == "" {
				return fmt.Errorf("$USER(%s) or $HOME(%s) not set", user, home)
			}

			// ensure that path is configured
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}

			// ensure that chains are configured
			src, dst := path.Src.ChainID, path.Dst.ChainID
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			// set paths on chains
			if err = chains[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = chains[dst].SetPath(path.Dst); err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = chains[src].GetAddress(); err != nil {
				return err
			}
			if _, err = chains[src].GetAddress(); err != nil {
				return err
			}

			// ensure that balances aren't == nil
			var srcBal, dstBal sdk.Coins
			if srcBal, err = chains[src].QueryBalance(chains[src].Key); err != nil {
				return err
			} else if srcBal.AmountOf(chains[src].DefaultDenom).IsZero() {
				return fmt.Errorf("no balance on %s, ensure %s has a balance before continuing setup", src, chains[src].MustGetAddress())
			}
			if dstBal, err = chains[dst].QueryBalance(chains[dst].Key); err != nil {
				return err
			} else if dstBal.AmountOf(chains[dst].DefaultDenom).IsZero() {
				return fmt.Errorf("no balance on %s, ensure %s has a balance before continuing setup", dst, chains[dst].MustGetAddress())
			}

			// ensure lite clients are initialized
			if _, err = chains[src].GetLatestLiteHeight(); err != nil {
				return fmt.Errorf("no lite client on %s, ensure it is initalized before continuing: %w", src, err)
			}
			if _, err = chains[dst].GetLatestLiteHeight(); err != nil {
				return fmt.Errorf("no lite client on %s, ensure it is initalized before continuing: %w", dst, err)
			}

			fmt.Printf(`[Unit]
Description=%s
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/rly start %s %s %s -d
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], user, home, home, src, dst, args[0])
			return nil
		},
	}
	return cmd
}

func faucetRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "request [chain-id] [[key-name]]",
		Aliases: []string{"req"},
		Short:   "request tokens from a relayer faucet",
		Args:    cobra.RangeArgs(1, 2),
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

			var keyName string
			if len(args) == 2 {
				keyName = args[1]
			} else {
				keyName = chain.Key
			}

			info, err := chain.Keybase.Get(keyName)
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
		Use:   "faucet [chain-id] [key-name] [amount]",
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
