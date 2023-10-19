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
	"io"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	flagCoinType           = "coin-type"
	flagAlgo               = "signing-algorithm"
	flagRestoreAll         = "restore-all"
	defaultCoinType uint32 = sdk.CoinType
)

// keysCmd represents the keys command
func keysCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keys",
		Aliases: []string{"k"},
		Short:   "Manage keys held by the relayer for each chain",
	}

	cmd.AddCommand(
		keysAddCmd(a),
		keysUseCmd(a),
		keysRestoreCmd(a),
		keysDeleteCmd(a),
		keysListCmd(a),
		keysExportCmd(a),
		keysShowCmd(a),
	)

	return cmd
}

func keysUseCmd(a *appState) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "use chain_name key_name",
		Aliases: []string{"u"},
		Short:   "Use a key from the keychain associated with a particular chain. Run 'rly keys list ibc-0' to view available keys",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys use ibc-0 key_name`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.useKey(args[0], args[1])
		},
	}
	return cmd
}

// keysAddCmd respresents the `keys add` command
func keysAddCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add chain_name key_name",
		Aliases: []string{"a"},
		Short:   "Adds a key to the keychain associated with a particular chain",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys add ibc-0
$ %s keys add ibc-1 key2
$ %s k a cosmoshub testkey`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			keyName := args[1]
			if chain.ChainProvider.KeyExists(keyName) {
				return errKeyExists(keyName)
			}

			cmdFlags := cmd.Flags()

			coinType, err := cmdFlags.GetInt32(flagCoinType)
			if err != nil {
				return err
			}

			if coinType < 0 {
				if ccp, ok := chain.ChainProvider.(*cosmos.CosmosProvider); ok && ccp.PCfg.Slip44 != nil {
					coinType = int32(*ccp.PCfg.Slip44)
				} else {
					coinType = int32(defaultCoinType)
				}
			}

			algo, err := cmdFlags.GetString(flagAlgo)
			if err != nil {
				return err
			}

			if algo == "" {
				if ccp, ok := chain.ChainProvider.(*cosmos.CosmosProvider); ok {
					algo = ccp.PCfg.SigningAlgorithm
				} else {
					algo = string(hd.Secp256k1Type)
				}
			}

			ko, err := chain.ChainProvider.AddKey(keyName, uint32(coinType), algo)
			if err != nil {
				return fmt.Errorf("failed to add key: %w", err)
			}

			out, err := json.Marshal(&ko)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().Int32(flagCoinType, -1, "coin type number for HD derivation")
	cmd.Flags().String(flagAlgo, "", "signing algorithm for key (secp256k1, sr25519)")

	return cmd
}

// keysRestoreCmd respresents the `keys add` command
func keysRestoreCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "restore chain_name key_name mnemonic",
		Aliases: []string{"r"},
		Short:   "Restores a mnemonic to the keychain associated with a particular chain",
		Args:    withUsage(cobra.RangeArgs(2, 3)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys restore  ibc-0 testkey "[mnemonic-words]"
$ %s k r  cosmoshub faucet-key "[mnemonic-words]"
$ %s k r demo-key "[mnemonic-words]" --restore-all`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {

			cmdFlags := cmd.Flags()

			restoreAll, err := cmdFlags.GetBool(flagRestoreAll)
			if err != nil {
				return err
			}

			if !restoreAll {

				keyName := args[1]
				chain, ok := a.config.Chains[args[0]]
				if !ok {
					return errChainNotFound(args[0])
				}

				if chain.ChainProvider.KeyExists(keyName) {
					return errKeyExists(keyName)
				}

				coinType, err := cmdFlags.GetInt32(flagCoinType)
				if err != nil {
					return err
				}

				if coinType < 0 {
					if ccp, ok := chain.ChainProvider.(*cosmos.CosmosProvider); ok && ccp.PCfg.Slip44 != nil {
						coinType = int32(*ccp.PCfg.Slip44)
					} else {
						coinType = int32(defaultCoinType)
					}
				}

				algo, err := cmdFlags.GetString(flagAlgo)
				if err != nil {
					return err
				}

				if algo == "" {
					if ccp, ok := chain.ChainProvider.(*cosmos.CosmosProvider); ok {
						algo = ccp.PCfg.SigningAlgorithm
					} else {
						algo = string(hd.Secp256k1Type)
					}
				}

				address, err := chain.ChainProvider.RestoreKey(keyName, args[2], uint32(coinType), algo)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), address)
				return nil

			}

			chains := a.config.Chains
			keyName := args[0]

			for i, c := range chains {

				chain := a.config.Chains[i]

				if chain.ChainProvider.KeyExists(keyName) {
					return errKeyExists(keyName)
				}

				coinType, err := cmdFlags.GetInt32(flagCoinType)
				if err != nil {
					return err
				}

				if coinType < 0 {
					if ccp, ok := c.ChainProvider.(*cosmos.CosmosProvider); ok && ccp.PCfg.Slip44 != nil {
						coinType = int32(*ccp.PCfg.Slip44)
					} else {
						coinType = int32(defaultCoinType)
					}
				}

				algo, err := cmdFlags.GetString(flagAlgo)
				if err != nil {
					return err
				}

				if algo == "" {
					if ccp, ok := c.ChainProvider.(*cosmos.CosmosProvider); ok {
						algo = ccp.PCfg.SigningAlgorithm
					} else {
						algo = string(hd.Secp256k1Type)
					}
				}

				addresses, err := c.ChainProvider.RestoreKey(keyName, args[1], uint32(coinType), algo)
				if err != nil {
					return err
				}

				fmt.Fprintln(cmd.OutOrStdout(), addresses)
			}

			return nil
		},
	}
	cmd.Flags().Int32(flagCoinType, -1, "coin type number for HD derivation")
	cmd.Flags().String(flagAlgo, "", "signing algorithm for key (secp256k1, sr25519)")
	cmd.Flags().Bool(flagRestoreAll, false, "restores keys for all configured chains with a single mnemonic")

	return cmd
}

// keysDeleteCmd respresents the `keys delete` command
func keysDeleteCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete chain_name key_name",
		Aliases: []string{"d"},
		Short:   "Deletes a key from the keychain associated with a particular chain",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys delete ibc-0 -y
$ %s keys delete ibc-1 key2 -y
$ %s k d cosmoshub default`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			keyName := args[1]
			if !chain.ChainProvider.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			if skip, _ := cmd.Flags().GetBool(flagSkip); !skip {
				fmt.Fprintf(cmd.ErrOrStderr(), "Are you sure you want to delete key(%s) from chain(%s)? (Y/n)\n", keyName, args[0])
				if !askForConfirmation(a, cmd.InOrStdin(), cmd.ErrOrStderr()) {
					return nil
				}
			}

			err := chain.ChainProvider.DeleteKey(keyName)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "key %s deleted\n", keyName)
			return nil
		},
	}

	return skipConfirm(a.viper, cmd)
}

func askForConfirmation(a *appState, stdin io.Reader, stderr io.Writer) bool {
	var response string

	_, err := fmt.Fscanln(stdin, &response)
	if err != nil {
		a.log.Fatal("Failed to read input", zap.Error(err))
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		fmt.Fprintln(stderr, "please type (y)es or (n)o and then press enter")
		return askForConfirmation(a, stdin, stderr)
	}
}

// keysListCmd respresents the `keys list` command
func keysListCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list chain_name",
		Aliases: []string{"l"},
		Short:   "Lists keys from the keychain associated with a particular chain",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys list ibc-0
$ %s k l ibc-1`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainName := args[0]

			chain, ok := a.config.Chains[chainName]
			if !ok {
				return errChainNotFound(chainName)
			}

			info, err := chain.ChainProvider.ListAddresses()
			if err != nil {
				return err
			}

			if len(info) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: no keys found for chain %s (do you need to run 'rly keys add %s'?)\n", chainName, chainName)
			}

			for key, val := range info {
				fmt.Fprintf(cmd.OutOrStdout(), "key(%s) -> %s\n", key, val)
			}

			return nil
		},
	}

	return cmd
}

// keysExportCmd respresents the `keys export` command
func keysExportCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "export chain_name key_name",
		Aliases: []string{"e"},
		Short:   "Exports a privkey from the keychain associated with a particular chain",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys export ibc-0 testkey
$ %s k e cosmoshub testkey`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[1]
			chain, ok := a.config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			if !chain.ChainProvider.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			info, err := chain.ChainProvider.ExportPrivKeyArmor(keyName)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), info)
			return nil
		},
	}

	return cmd
}

// ShowAddressByChainAndKey represents the logic for showing relayer address by chain_name and key_name
func (a *appState) showAddressByChainAndKey(cmd *cobra.Command, args []string) error {
	chain, ok := a.config.Chains[args[0]]
	if !ok {
		return errChainNotFound(args[0])
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

	fmt.Fprintln(cmd.OutOrStdout(), address)
	return nil
}

// keysShowCmd respresents the `keys show` command
func keysShowCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show chain_name [key_name]",
		Aliases: []string{"s"},
		Short:   "Shows a key from the keychain associated with a particular chain",
		Args:    withUsage(cobra.RangeArgs(1, 2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s keys show ibc-0
$ %s keys show ibc-1 key2
$ %s k s ibc-2 testkey`, appName, appName, appName)),
		RunE: a.showAddressByChainAndKey,
	}

	return cmd
}

// addressCmd represents the address of a relayer
func addressCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "address chain_name [key_name]",
		Aliases: []string{"a"},
		Short:   "Shows the address of a relayer",
		Args:    withUsage(cobra.RangeArgs(1, 2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s address ibc-0
$ %s address ibc-1 key2
$ %s a ibc-2 testkey`, appName, appName, appName)),
		RunE: a.showAddressByChainAndKey,
	}

	return cmd
}
