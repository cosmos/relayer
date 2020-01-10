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

	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/go-bip39"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keysAddCmd)
	keysCmd.AddCommand(keysDeleteCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysShowCmd)
	keysCmd.AddCommand(keysExportCmd)
}

// keysCmd represents the keys command
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "helps users manage keys for multiple chains",
}

var keysAddCmd = &cobra.Command{
	Use:   "add [chain-id] [name]",
	Short: "adds a key to the keychain associated with a particular chain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(config)
		fmt.Println(config.Chains)
		fmt.Println(config.c)
		chainID := args[0]
		keyName := args[1]

		if !relayer.Exists(chainID, config.c) {
			return fmt.Errorf("chain with ID %s is not configured", chainID)
		}

		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		mnemonic, err := createMnemonic()
		if err != nil {
			return err
		}

		if keyExists(chain.Keybase, keyName) {
			return fmt.Errorf("a key with name %s already exists", keyName)
		}

		info, err := chain.Keybase.CreateAccount(keyName, mnemonic, "", "", 0, 0)
		if err != nil {
			return err
		}

		fmt.Println("seed", mnemonic)
		fmt.Println("address", info.GetAddress())
		return nil
	},
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete [chain-id] [name]",
	Short: "deletes a key from the keychain associated with a particular chain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		keyName := args[1]

		if !relayer.Exists(chainID, config.c) {
			return fmt.Errorf("chain with ID %s is not configured", chainID)
		}

		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		if !keyExists(chain.Keybase, keyName) {
			return fmt.Errorf("a key with name %s doesn't exist", keyName)
		}

		err = chain.Keybase.Delete(keyName, "", true)
		if err != nil {
			panic(err)
		}

		fmt.Printf("key %s deleted\n", keyName)
		return nil
	},
}

var keysListCmd = &cobra.Command{
	Use:   "list [chain-id]",
	Short: "lists keys from the keychain associated with a particular chain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]

		if !relayer.Exists(chainID, config.c) {
			return fmt.Errorf("chain with ID %s is not configured", chainID)
		}

		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		info, err := chain.Keybase.List()
		if err != nil {
			return err
		}

		fmt.Println(info)
		return nil
	},
}

var keysShowCmd = &cobra.Command{
	Use:   "show [chain-id] [name]",
	Short: "shows a key from the keychain associated with a particular chain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		keyName := args[1]

		if !relayer.Exists(chainID, config.c) {
			return fmt.Errorf("chain with ID %s is not configured", chainID)
		}

		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		if !keyExists(chain.Keybase, keyName) {
			return fmt.Errorf("a key with name %s doesn't exist", keyName)
		}

		info, err := chain.Keybase.Get(keyName)
		if err != nil {
			return err
		}

		fmt.Println(info)
		return nil
	},
}

var keysExportCmd = &cobra.Command{
	Use:   "export [chain-id] [name]",
	Short: "exports a privkey from the keychain associated with a particular chain",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		chainID := args[0]
		keyName := args[1]

		if !relayer.Exists(chainID, config.c) {
			return fmt.Errorf("chain with ID %s is not configured", chainID)
		}

		chain, err := relayer.GetChain(chainID, config.c)
		if err != nil {
			return err
		}

		if !keyExists(chain.Keybase, keyName) {
			return fmt.Errorf("a key with name %s doesn't exist", keyName)
		}

		info, err := chain.Keybase.ExportPrivateKeyObject(keyName, "")
		if err != nil {
			return err
		}

		fmt.Println(info)
		return nil
	},
}

func keyExists(kb keys.Keybase, name string) bool {
	keyInfos, _ := kb.List()
	for _, k := range keyInfos {
		if k.GetName() == name {
			return true
		}
	}
	return false
}

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
