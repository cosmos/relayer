/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

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
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// transactionCmd represents the tx command
func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transact",
		Aliases: []string{"tx"},
		Short:   "IBC Transaction Commands",
		Long: strings.TrimSpace(`Commands to create IBC transactions on configured chains. 
		Most of these commands take a '[path]' argument. Make sure:
	1. Chains are properly configured to relay over by using the 'rly chains list' command
	2. Path is properly configured to relay over by using the 'rly paths list' command`),
	}

	cmd.AddCommand(
		linkCmd(),
		linkThenStartCmd(),
		relayMsgsCmd(),
		relayAcksCmd(),
		xfersend(),
		flags.LineBreak,
		createClientsCmd(),
		updateClientsCmd(),
		upgradeClientsCmd(),
		upgradeChainCmd(),
		createConnectionCmd(),
		createChannelCmd(),
		closeChannelCmd(),
		flags.LineBreak,
		rawTransactionCmd(),
	)

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients [path-name]",
		Aliases: []string{"clnts"},
		Short:   "create a clients between two configured chains with a configured path",
		Long: "Creates a working ibc client for chain configured on each end of the" +
			" path by querying headers from each chain and then sending the corresponding create-client messages",
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact clients demo-path
$ %s tx clnts demo-path`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			modified, err := c[src].CreateClients(c[dst])
			if modified {
				if err := overWriteConfig(cmd, config); err != nil {
					return err
				}
			}

			return err
		},
	}
	return cmd
}

func updateClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update-clients [path-name]",
		Aliases: []string{"update", "uc"},
		Short:   "update a clients between two configured chains with a configured path",
		Long: "Updates a working ibc client for chain configured on each end of the " +
			"path by querying headers from each chain and then sending the corresponding update-client messages",
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact update-clients demo-path
$ %s tx update demo-path
$ %s tx uc demo-path`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			return c[src].UpdateClients(c[dst])
		},
	}
	return cmd
}

func upgradeClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upgrade-clients [path-name] [chain-id]",
		Aliases: []string{"upgrade"},
		Short:   "upgrade a client on the provided chain-id",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			targetChainID := args[1]
			// send the upgrade message on the targetChainID
			if src == targetChainID {
				return c[src].UpgradeClients(c[dst])
			}

			return c[dst].UpgradeClients(c[src])
		},
	}
	return cmd
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [path-name]",
		Aliases: []string{"conn", "con"},
		Short:   "create a connection between two configured chains with a configured path",
		Long: strings.TrimSpace(`This command is meant to be used to repair or create 
		a connection between two chains with a configured path in the config file`),
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact connection demo-path
$ %s tx conn demo-path --timeout 5s
$ %s tx con demo-path -o 3s`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// TODO: make '3' be a flag, maximum retries after failed message send
			modified, err := c[src].CreateOpenConnections(c[dst], 3, to)
			if modified {
				if err := overWriteConfig(cmd, config); err != nil {
					return err
				}
			}

			return err
		},
	}

	return timeoutFlag(cmd)
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel [path-name]",
		Aliases: []string{"chan", "ch"},
		Short:   "create a channel between two configured chains with a configured path",
		Long: strings.TrimSpace(`This command is meant to be used to repair or 
		create a channel between two chains with a configured path in the config file`),
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact channel demo-path
$ %s tx chan demo-path --timeout 5s
$ %s tx ch demo-path -o 3s`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// TODO: make '3' a flag, max retries after failed message send
			modified, err := c[src].CreateOpenChannels(c[dst], 3, to)
			if modified {
				if err := overWriteConfig(cmd, config); err != nil {
					return err
				}
			}

			return err

		},
	}

	return timeoutFlag(cmd)
}

func closeChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel-close [path-name]",
		Aliases: []string{"chan-cl", "close", "cl"},
		Short:   "close a channel between two configured chains with a configured path",
		Long:    "This command is meant to close a channel",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact channel-close demo-path
$ %s tx chan-cl demo-path --timeout 5s
$ %s tx cl demo-path
$ %s tx close demo-path -o 3s`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			return c[src].CloseChannel(c[dst], to)
		},
	}

	return timeoutFlag(cmd)
}

func linkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "link [path-name]",
		Aliases: []string{"full-path", "connect", "path", "pth"},
		Short:   "create clients, connection, and channel between two configured chains with a configured path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact link demo-path
$ %s tx full-path demo-path --timeout 5s
$ %s tx connect demo-path
$ %s tx path demo-path -o 3s
$ %s tx pth demo-path`, appName, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// create clients if they aren't already created
			modified, err := c[src].CreateClients(c[dst])
			if modified {
				overWriteConfig(cmd, config)
			}

			if err != nil {
				return err
			}

			// TODO: make '3' a flag, maximum retries after failed message send
			// create connection if it isn't already created
			modified, err = c[src].CreateOpenConnections(c[dst], 3, to)
			if modified {
				if err := overWriteConfig(cmd, config); err != nil {
					return err
				}
			}
			if err != nil {
				return err
			}

			// create channel if it isn't already created
			modified, err = c[src].CreateOpenChannels(c[dst], 3, to)
			if modified {
				if err := overWriteConfig(cmd, config); err != nil {
					return err
				}
			}
			return err
		},
	}

	return timeoutFlag(cmd)
}

func linkThenStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link-then-start [path-name]",
		Short: "wait for a link to come up, then start relaying packets",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact link-then-start demo-path
$ %s tx link-then-start demo-path --timeout 5s`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			lCmd := linkCmd()
			for err := lCmd.RunE(cmd, args); err != nil; err = lCmd.RunE(cmd, args) {
				fmt.Printf("retrying link: %s\n", err)
				time.Sleep(1 * time.Second)
			}
			sCmd := startCmd()
			return sCmd.RunE(cmd, args)
		},
	}
	return strategyFlag(timeoutFlag(cmd))
}

func relayMsgsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay-packets [path-name]",
		Aliases: []string{"rly", "pkts", "relay"},
		Short:   "relay any packets that remain to be relayed on a given path, in both directions",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact relay-packets demo-path
$ %s tx rly demo-path -l 3
$ %s tx pkts demo-path -s 5
$ %s tx relay demo-path`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = ensureKeysExist(c); err != nil {
				return err
			}

			sh, err := relayer.NewSyncHeaders(c[src], c[dst])
			if err != nil {
				return err
			}

			strategy, err := GetStrategyWithOptions(cmd, config.Paths.MustGet(args[0]).MustGetStrategy())
			if err != nil {
				return err
			}

			sp, err := strategy.UnrelayedSequences(c[src], c[dst], sh)
			if err != nil {
				return err
			}

			if err = strategy.RelayPackets(c[src], c[dst], sp, sh); err != nil {
				return err
			}

			return nil
		},
	}

	return strategyFlag(cmd)
}

func relayAcksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay-acknowledgements [path-name]",
		Aliases: []string{"acks"},
		Short:   "relay any acknowledgements that remain to be relayed on a given path, in both directions",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact relay-acknowledgements demo-path
$ %s tx acks demo-path -l 3 -s 6`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = ensureKeysExist(c); err != nil {
				return err
			}

			sh, err := relayer.NewSyncHeaders(c[src], c[dst])
			if err != nil {
				return err
			}

			strategy, err := GetStrategyWithOptions(cmd, config.Paths.MustGet(args[0]).MustGetStrategy())
			if err != nil {
				return err
			}

			// sp.Src contains all sequences acked on SRC but acknowledgement not processed on DST
			// sp.Dst contains all sequences acked on DST but acknowledgement not processed on SRC
			sp, err := strategy.UnrelayedAcknowledgements(c[src], c[dst], sh)
			if err != nil {
				return err
			}

			if err = strategy.RelayAcknowledgements(c[src], c[dst], sp, sh); err != nil {
				return err
			}

			return nil
		},
	}

	return strategyFlag(cmd)
}

func upgradeChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-chain [path-name] [chain-id] [new-unbonding-period] [deposit] [path/to/upgradePlan.json]",
		Short: "upgrade a chain by providing the chain-id of the chain being upgraded, the new unbonding period, the proposal deposit and the json file of the upgrade plan without the upgrade client state ",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			targetChainID := args[1]

			unbondingPeriod, err := time.ParseDuration(args[2])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// parse deposit
			deposit, err := sdk.ParseCoinNormalized(args[3])
			if err != nil {
				return err
			}

			// parse plan
			plan := &upgradetypes.Plan{}
			path := args[4]
			if _, err := os.Stat(path); err != nil {
				return err
			}

			byt, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			if err = json.Unmarshal(byt, plan); err != nil {
				return err
			}

			// send the upgrade message on the targetChainID
			if src == targetChainID {
				return c[src].UpgradeChain(c[dst], plan, deposit, unbondingPeriod)
			}

			return c[dst].UpgradeChain(c[src], plan, deposit, unbondingPeriod)
		},
	}
	return cmd
}

// Returns an error if a configured key for a given chain doesn't exist
func ensureKeysExist(chains map[string]*relayer.Chain) (err error) {
	for _, v := range chains {
		if _, err = v.GetAddress(); err != nil {
			return
		}
	}
	return nil
}
