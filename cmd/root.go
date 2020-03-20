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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgPath     string
	homePath    string
	debug       bool
	config      *Config
	defaultHome = os.ExpandEnv("$HOME/.relayer")
	cdc         *codec.Codec
	appCodec    *codecstd.Codec

	// Default identifiers for dummy usage
	dcli = "defaultclientid"
	dcon = "defaultconnectionid"
	dcha = "defaultchannelid"
	dpor = "defaultportid"
)

func init() {
	// Register top level flags --home and --config
	// TODO: just rely on homePath and remove the config path arg?
	rootCmd.PersistentFlags().StringVar(&homePath, flags.FlagHome, defaultHome, "set home directory")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "debug output")
	rootCmd.PersistentFlags().StringVar(&cfgPath, flagConfig, "config.yaml", "set config file")
	if err := viper.BindPFlag(flags.FlagHome, rootCmd.Flags().Lookup(flags.FlagHome)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag(flagConfig, rootCmd.Flags().Lookup(flagConfig)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug")); err != nil {
		panic(err)
	}

	// Register subcommands
	rootCmd.AddCommand(
		liteCmd,
		keysCmd,
		queryCmd,
		startCmd(),
		transactionCmd(),
		chainsCmd(),
		pathsCmd(),
		configCmd(),
		getVersionCmd(),
		testnetsCmd(),
	)

	// This is a bit of a cheat :shushing_face:
	cdc = codecstd.MakeCodec(simapp.ModuleBasics)
	appCodec = codecstd.NewAppCodec(cdc)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rly",
	Short: "This application relays data between configured IBC enabled chains",
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

// readLineFromBuf reads one line from stdin.
func readStdin() (string, error) {
	str, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(str), err
}
