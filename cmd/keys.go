/*
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
	"fmt"

	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/go-bip39"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	keysCmd.AddCommand(keysAddCmd())
	keysCmd.AddCommand(keysRestoreCmd())
	keysCmd.AddCommand(keysDeleteCmd())
	keysCmd.AddCommand(keysListCmd())
	keysCmd.AddCommand(keysShowCmd())
	keysCmd.AddCommand(keysExportCmd())
}

// keysCmd represents the keys command
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "helps users manage keys for multiple chains",
}

// keysAddCmd respresents the `keys add` command
func keysAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [chain-id] [name]",
		Short: "adds a key to the keychain associated with a particular chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			mnemonic, err := createMnemonic()
			if err != nil {
				return err
			}

			if keyExists(chain.Keybase, keyName) {
				return errKeyExists(keyName)
			}

			info, err := chain.Keybase.CreateAccount(keyName, mnemonic, "", ckeys.DefaultKeyPass, keys.CreateHDPath(0, 0).String(), keys.Secp256k1)
			if err != nil {
				return err
			}

			return PrintOutput(info, cmd)
		},
	}

	return outputFlags(cmd)
}

// keysRestoreCmd respresents the `keys add` command
func keysRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore [chain-id] [name] [mnemonic]",
		Short: "restores a mnemonic to the keychain associated with a particular chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if keyExists(chain.Keybase, keyName) {
				return errKeyExists(keyName)
			}

			info, err := chain.Keybase.CreateAccount(keyName, args[2], "", ckeys.DefaultKeyPass, keys.CreateHDPath(0, 0).String(), keys.Secp256k1)
			if err != nil {
				return err
			}

			printAddress, _ := cmd.Flags().GetBool(flagAddress)
			if printAddress {
				fmt.Println(info.GetAddress().String())
				return nil
			}

			return PrintOutput(info, cmd)
		},
	}

	return addressFlag(outputFlags(cmd))
}

// keysDeleteCmd respresents the `keys delete` command
func keysDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [chain-id] [name]",
		Short: "deletes a key from the keychain associated with a particular chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if !keyExists(chain.Keybase, keyName) {
				return errKeyDoesntExist(keyName)
			}

			err = chain.Keybase.Delete(keyName, ckeys.DefaultKeyPass, true)
			if err != nil {
				panic(err)
			}

			fmt.Printf("key %s deleted\n", keyName)
			return nil
		},
	}

	return cmd
}

// keysListCmd respresents the `keys list` command
func keysListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [chain-id]",
		Short: "lists keys from the keychain associated with a particular chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			info, err := chain.Keybase.List()
			if err != nil {
				return err
			}

			return PrintOutput(info, cmd)
		},
	}

	return outputFlags(cmd)
}

// keysShowCmd respresents the `keys show` command
func keysShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [chain-id] [name]",
		Short: "shows a key from the keychain associated with a particular chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if !keyExists(chain.Keybase, keyName) {
				return errKeyDoesntExist(keyName)
			}

			info, err := chain.Keybase.Get(keyName)
			if err != nil {
				return err
			}

			if viper.GetBool(flagAddress) {
				fmt.Println(info.GetAddress().String())
				return nil
			}

			return PrintOutput(info, cmd)
		},
	}

	return addressFlag(outputFlags(cmd))
}

// keysExportCmd respresents the `keys export` command
func keysExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [chain-id] [name]",
		Short: "exports a privkey from the keychain associated with a particular chain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			if !keyExists(chain.Keybase, keyName) {
				return errKeyDoesntExist(keyName)
			}

			info, err := chain.Keybase.ExportPrivKey(keyName, ckeys.DefaultKeyPass, ckeys.DefaultKeyPass)
			if err != nil {
				return err
			}

			return PrintOutput(info, cmd)
		},
	}

	return outputFlags(cmd)
}

// returns true if there is a specified key in the keybase
func keyExists(kb keys.Keybase, name string) bool {
	keyInfos, err := kb.List()
	if err != nil {
		return false
	}

	for _, k := range keyInfos {
		if k.GetName() == name {
			return true
		}
	}
	return false
}

// Returns a new mnemonic
func createMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}
