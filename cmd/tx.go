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
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	transactionCmd.AddCommand(createClientCmd())
	transactionCmd.AddCommand(createClientsCmd())
	transactionCmd.AddCommand(createConnectionCmd())
	transactionCmd.AddCommand(createChannelCmd())
}

// transactionCmd represents the tx command
var transactionCmd = &cobra.Command{
	Use:     "transactions",
	Aliases: []string{"tx"},
	Short:   "IBC Transaction Commands, UNDER CONSTRUCTION",
}

func createClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client [src-chain-id] [dst-chain-id] [client-id]",
		Short: "create a client for dst-chain on src-chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcChain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			dstChain, err := config.c.GetChain(args[1])
			if err != nil {
				return err
			}

			err = srcChain.UpdateLiteDBToLatestHeader()
			if err != nil {
				return err
			}

			err = dstChain.UpdateLiteDBToLatestHeader()
			if err != nil {
				return err
			}

			dstHeight, err := dstChain.QueryLatestHeight()
			if err != nil {
				return err
			}

			srcAddr, err := srcChain.GetAddress()
			if err != nil {
				return err
			}

			msgCreateClient, err := srcChain.CreateClient(dstChain, dstHeight, srcAddr)
			if err != nil {
				return err
			}

			res, err := srcChain.SendMsg(msgCreateClient)
			if err != nil {
				return err
			}

			fmt.Println(res)
			return nil
		},
	}

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id]",
		Short: "create a clients for dst-chain on src-chain and src-chain on dst-chain",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcChain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			dstChain, err := config.c.GetChain(args[1])
			if err != nil {
				return err
			}

			dstHeight, err := dstChain.QueryLatestHeight()
			if err != nil {
				return err
			}

			srcHeight, err := srcChain.QueryLatestHeight()
			if err != nil {
				return err
			}

			srcAddr, err := srcChain.GetAddress()
			if err != nil {
				return err
			}

			dstAddr, err := dstChain.GetAddress()
			if err != nil {
				return err
			}

			srcCreateClient, err := srcChain.CreateClient(dstChain, dstHeight, srcAddr)
			if err != nil {
				return err
			}

			res, err := srcChain.SendMsg(srcCreateClient)
			if err != nil {
				return err
			}
			fmt.Println(res)

			dstCreateClient, err := dstChain.CreateClient(srcChain, srcHeight, dstAddr)
			if err != nil {
				return err
			}

			res, err = dstChain.SendMsg(dstCreateClient)
			if err != nil {
				return err
			}

			fmt.Println(res)
			return nil
		},
	}
	return cmd
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id], [dst-connection-id]",
		Short: "create a connection between chains, passing in identifiers",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout := 5 * time.Second

			srcChain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			dstChain, err := config.c.GetChain(args[1])
			if err != nil {
				return err
			}

			// TODO: validate identifiers ICS24

			err = srcChain.CreateConnection(dstChain, args[2], args[3], args[4], args[5], timeout)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [src-chain-id] [dst-chain-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]",
		Short: "",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout := 5 * time.Second

			srcChain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			dstChain, err := config.c.GetChain(args[1])
			if err != nil {
				return err
			}

			// TODO: validate identifiers ICS24

			err = srcChain.CreateChannel(dstChain, args[2], args[3], args[4], args[5], args[6], args[7], timeout)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
