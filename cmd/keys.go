/*
Package cmd includes relayer commands
Copyright © 2020 Jack Zampolin jack.zampolin@gmail.com

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
	"log"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
)

const (
	flagCoinType           = "coin-type"
	defaultCoinType uint32 = sdk.CoinType
)

// keysCmd represents the keys command
func keysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keys",
		Aliases: []string{"k"},
		Short:   "Manage keys held by the relayer for each chain",
	}

	cmd.AddCommand(keysAddCmd())
	cmd.AddCommand(keysRestoreCmd())
	cmd.AddCommand(keysDeleteCmd())
	cmd.AddCommand(keysListCmd())
	cmd.AddCommand(keysShowCmd())
	cmd.AddCommand(keysExportCmd())

	return cmd
}

// keysAddCmd respresents the `keys add` command
func keysAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [chain-id] [name]",
		Aliases: []string{"a"},
		Short:   "Adds a key to the keychain associated with a particular chain",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys add ibc-0
$ %s keys add ibc-1 key2
$ %s k a ibc-2 testkey`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			keyName := args[1]
			if chain.ChainProvider.KeyExists(keyName) {
				return errKeyExists(keyName)
			}

			ko, err := chain.ChainProvider.AddKey(keyName)
			if err != nil {
				return err
			}

			out, err := json.Marshal(&ko)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}
	cmd.Flags().Uint32(flagCoinType, defaultCoinType, "coin type number for HD derivation")

	return cmd
}

// keysRestoreCmd respresents the `keys add` command
func keysRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "restore [chain-id] [name] [mnemonic]",
		Aliases: []string{"r"},
		Short:   "Restores a mnemonic to the keychain associated with a particular chain",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys restore ibc-0 testkey "[mnemonic-words]"
$ %s k r ibc-1 faucet-key "[mnemonic-words]"`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if chain.ChainProvider.KeyExists(keyName) {
				return errKeyExists(keyName)
			}

			address, err := chain.ChainProvider.RestoreKey(keyName, args[2])
			if err != nil {
				return err
			}

			fmt.Println(address)
			return nil
		},
	}
	cmd.Flags().Uint32(flagCoinType, defaultCoinType, "coin type number for HD derivation")

	return cmd
}

// keysDeleteCmd respresents the `keys delete` command
func keysDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [chain-id] [name]",
		Aliases: []string{"d"},
		Short:   "Deletes a key from the keychain associated with a particular chain",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys delete ibc-0 -y
$ %s keys delete ibc-1 key2 -y
$ %s k d ibc-2 testkey`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			keyName := args[1]
			if !chain.ChainProvider.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			if skip, _ := cmd.Flags().GetBool(flagSkip); !skip {
				fmt.Printf("Are you sure you want to delete key(%s) from chain(%s)? (Y/n)\n", keyName, args[0])
				if !askForConfirmation() {
					return nil
				}
			}

			err = chain.ChainProvider.DeleteKey(keyName)
			if err != nil {
				panic(err)
			}

			fmt.Printf("key %s deleted\n", keyName)
			return nil
		},
	}

	return skipConfirm(cmd)
}

func askForConfirmation() bool {
	var response string

	_, err := fmt.Scanln(&response)
	if err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		fmt.Println("please type (y)es or (n)o and then press enter")
		return askForConfirmation()
	}
}

// keysListCmd respresents the `keys list` command
func keysListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [chain-id]",
		Aliases: []string{"l"},
		Short:   "Lists keys from the keychain associated with a particular chain",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys list ibc-0
$ %s k l ibc-1`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			info, err := chain.ChainProvider.ListAddresses()
			if err != nil {
				return err
			}

			for key, val := range info {
				fmt.Printf("key(%s) -> %s\n", key, val)
			}

			return nil
		},
	}

	return cmd
}

// keysShowCmd respresents the `keys show` command
func keysShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show [chain-id] [[name]]",
		Aliases: []string{"s"},
		Short:   "Shows a key from the keychain associated with a particular chain",
		Args:    cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys show ibc-0
$ %s keys show ibc-1 key2
$ %s k s ibc-2 testkey`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			var keyName string
			if len(args) == 2 {
				keyName = args[1]
			} else {
				keyName = chain.ChainProvider.Key()
			}

			if !chain.ChainProvider.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			address, err := chain.ChainProvider.ShowAddress(keyName)
			if err != nil {
				return err
			}

			fmt.Println(address)
			return nil
		},
	}

	return cmd
}

// keysExportCmd respresents the `keys export` command
func keysExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "export [chain-id] [name]",
		Aliases: []string{"e"},
		Short:   "Exports a privkey from the keychain associated with a particular chain",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys export ibc-0 testkey
$ %s k e ibc-2 testkey`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if !chain.ChainProvider.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			info, err := chain.ChainProvider.ExportPrivKeyArmor(keyName)
			if err != nil {
				return err
			}

			fmt.Println(info)
			return nil
		},
	}

	return cmd
}
