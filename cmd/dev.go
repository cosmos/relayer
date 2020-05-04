package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"github.com/spf13/cobra"
)

func devCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "development",
		Aliases: []string{"dev"},
		Short:   "commands for developers either deploying or hacking on the relayer",
	}
	cmd.AddCommand(
		gaiaServiceCmd(),
		faucetService(),
		rlyService(),
		listenCmd(),
		genesisCmd(),
		gozDataCmd(),
	)
	return cmd
}

func gozDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goz-dump [chain-id]",
		Aliases: []string{"dump", "goz"},
		Short:   "fetch the list of chains connected as a CSV dump",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			clients, err := c.QueryClients(1, 1000)
			if err != nil {
				return err
			}

			header, err := c.UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			var clientDatas = []*clientData{}
			for _, cl := range clients {
				cd := &clientData{
					ClientID:            cl.GetID(),
					ChainID:             cl.GetChainID(),
					TimeSinceLastUpdate: time.Since(cl.(tmclient.ClientState).LastHeader.Time).String(),
					ChannelIDs:          []string{},
				}

				if err := c.AddPath(cd.ClientID, dcon, dcha, dpor, dord); err != nil {
					return err
				}

				conns, err := c.QueryConnectionsUsingClient(header.Height)
				if err != nil {
					return err
				}

				cd.ConnectionIDs = conns.ConnectionPaths
				for _, conn := range conns.ConnectionPaths {
					if err := c.AddPath(cl.GetID(), conn, dcha, dpor, dord); err != nil {
						return err
					}

					chans, err := c.QueryConnectionChannels(conn, 1, 1000)
					if err != nil {
						return err
					}
					for _, cha := range chans {
						if cha.State.String() == "OPEN" {
							cd.ChannelIDs = append(cd.ChannelIDs, cha.ID)
						}
					}

				}
				clientDatas = append(clientDatas, cd)

			}

			out, _ := json.Marshal(clientDatas)
			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

type clientData struct {
	ClientID            string   `json:"client-id"`
	ConnectionIDs       []string `json:"connection-ids"`
	ChannelIDs          []string `json:"channel-ids"`
	ChainID             string   `json:"chain-id"`
	TimeSinceLastUpdate string   `json:"since-last-update"`
}

func genesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "genesis [chain-id]",
		Aliases: []string{"gen"},
		Short:   "fetch the genesis file for a configured chain",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			gen, err := c.Client.Genesis()
			if err != nil {
				return err
			}

			return c.Print(gen, false, false)
		},
	}
	return cmd
}

// listenCmd represents the listen command
func listenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listen [chain-id]",
		Aliases: []string{"l"},
		Short:   "listen to all transaction and block events from a given chain and output them to stdout",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			tx, err := cmd.Flags().GetBool(flagTx)
			if err != nil {
				return err
			}
			block, err := cmd.Flags().GetBool(flagBlock)
			if err != nil {
				return err
			}
			data, err := cmd.Flags().GetBool(flagData)
			if err != nil {
				return err
			}

			if block && tx {
				return fmt.Errorf("Must output block and/or tx")
			}

			done := c.ListenRPCEmitJSON(tx, block, data)

			trapSignal(done)

			return nil
		},
	}
	return listenFlags(cmd)
}

func gaiaServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gaia [user] [home]",
		Short: "gaia returns a sample gaiad service file",
		Args:  cobra.ExactArgs(2),
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
		Use:   "faucet [user] [home] [chain-id] [key-name] [amount]",
		Short: "faucet returns a sample faucet service file",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[2])
			if err != nil {
				return err
			}
			_, err = chain.Keybase.Key(args[3])
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
		Use:     "relayer [path-name]",
		Aliases: []string{"rly"},
		Short:   "relayer returns a service file for the relayer to relay over an individual path",
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
