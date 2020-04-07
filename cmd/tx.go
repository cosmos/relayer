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
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

// transactionCmd represents the tx command
func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transactions",
		Aliases: []string{"tx"},
		Short:   "IBC Transaction Commands",
	}

	cmd.AddCommand(
		createClientsCmd(),
		createConnectionCmd(),
		createChannelCmd(),
		closeChannelCmd(),
		fullPathCmd(),
		rawTransactionCmd(),
		transferCmd(),
		relayMsgsCmd(),
	)

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients [path-name]",
		Aliases: []string{"clnts"},
		Short:   "create a clients between two configured chains with a configured path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			return c[src].CreateClients(c[dst])
		},
	}
	return cmd
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [path-name]",
		Aliases: []string{"conn"},
		Short:   "create a connection between two configured chains with a configured path",
		Long:    "This command is meant to be used to repair or create a connection between two chains with a configured path in the config file",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			return c[src].CreateConnection(c[dst], to)
		},
	}

	return timeoutFlag(cmd)
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel [path-name]",
		Aliases: []string{"chan"},
		Short:   "create a channel between two configured chains with a configured path",
		Long:    "This command is meant to be used to repair or create a channel between two chains with a configured path in the config file",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// TODO: read order out of path config
			return c[src].CreateChannel(c[dst], true, to)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			return c[src].CloseChannel(c[dst], to)
		},
	}

	return timeoutFlag(cmd)
}

func fullPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "full-path [path-name]",
		Aliases: []string{"link", "connect", "path", "pth"},
		Short:   "create clients, connection, and channel between two configured chains with a configured path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if err = c[src].CreateClients(c[dst]); err != nil {
				return err
			}

			if err = c[src].CreateConnection(c[dst], to); err != nil {
				return err
			}

			return c[src].CreateChannel(c[dst], true, to)
		},
	}

	return timeoutFlag(cmd)
}

func relayMsgsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay [path-name]",
		Aliases: []string{"rly", "queue"},
		Short:   "Queries for the packets that remain to be relayed on a given path, in both directions, and relays them",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = relayer.RelayUnRelayedPacketsOrderedChan(c[src], c[dst]); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
