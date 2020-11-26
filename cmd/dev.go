package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	)
	return cmd
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

			gen, err := c.Client.Genesis(context.Background())
			if err != nil {
				return err
			}

			out, err := json.Marshal(gen)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
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
				return fmt.Errorf("must output block and/or tx")
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
			_, err = sdk.ParseCoinNormalized(args[4])
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
			if _, err = chains[dst].GetAddress(); err != nil {
				return err
			}

			// ensure that balances aren't == nil
			var srcBal, dstBal sdk.Coins
			if srcBal, err = chains[src].QueryBalance(chains[src].Key); err != nil {
				return err
			} else if srcBal.Empty() {
				return fmt.Errorf("no balance on %s, ensure %s has a balance before continuing setup",
					src, chains[src].MustGetAddress())
			}
			if dstBal, err = chains[dst].QueryBalance(chains[dst].Key); err != nil {
				return err
			} else if dstBal.Empty() {
				return fmt.Errorf("no balance on %s, ensure %s has a balance before continuing setup",
					dst, chains[dst].MustGetAddress())
			}

			// ensure light clients are initialized
			if _, err = chains[src].GetLatestLightHeight(); err != nil {
				return fmt.Errorf("no light client on %s, ensure it is initialized before continuing: %w", src, err)
			}
			if _, err = chains[dst].GetLatestLightHeight(); err != nil {
				return fmt.Errorf("no light client on %s, ensure it is initialized before continuing: %w", dst, err)
			}

			fmt.Printf(`[Unit]
Description=%s
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/rly start %s -d
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], user, home, home, args[0])
			return nil
		},
	}
	return cmd
}
