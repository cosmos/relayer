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
	"encoding/json"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/ibc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgPath     string
	homePath    string
	config      *Config
	defaultHome = os.ExpandEnv("$HOME/.relayer")
	cdc         *codec.Codec
)

func init() {
	// Register top level flags --home and --config
	rootCmd.PersistentFlags().StringVar(&homePath, flags.FlagHome, defaultHome, "set home directory")
	rootCmd.PersistentFlags().StringVar(&cfgPath, flagConfig, "config.yaml", "set config file")
	viper.BindPFlag(flags.FlagHome, rootCmd.Flags().Lookup(flags.FlagHome))
	viper.BindPFlag(flagConfig, rootCmd.Flags().Lookup(flagConfig))

	// Register subcommands
	rootCmd.AddCommand(
		liteCmd,
		keysCmd,
		queryCmd,
		startCmd,
		transactionCmd,
		chainsCmd(),
	)

	// This is a bit of a cheat :shushing_face:
	cdc = codec.New()
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	codec.RegisterEvidences(cdc)
	authvesting.RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	keys.RegisterCodec(cdc)
	ibc.AppModuleBasic{}.RegisterCodec(cdc)
	cdc.Seal()

}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "relayer",
	Short: "This application relays data between configured IBC enabled chains",
}

func chainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "Returns chain configuration data",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := json.Marshal(config.Chains)
			if err != nil {
				return err
			}

			return PrintOutput(out, cmd)
		},
	}

	return outputFlags(cmd)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		// reads `homeDir/config/config.yaml` into `var config *Config` before each command
		return initConfig(rootCmd)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
