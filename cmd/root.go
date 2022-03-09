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
	"bufio"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// MB is a megabyte
const (
	MB = 1048576 // in bytes
)

var (
	homePath    string
	debug       bool
	config      *Config
	defaultHome = os.ExpandEnv("$HOME/.relayer")
	appName     = "rly"

	// Default identifiers for dummy usage
	dcli = "defaultclientid"
	dcon = "defaultconnectionid"
	dcha = "defaultchannelid"
	dpor = "defaultportid"
	dord = "ordered"
)

// NewRootCmd returns the root command for relayer.
func NewRootCmd() *cobra.Command {
	// RootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   appName,
		Short: "This application makes data relay between IBC enabled chains easy!",
		Long: strings.TrimSpace(`rly has:
   1. Configuration management for Chains and Paths
   2. Key management for managing multiple keys for multiple chains
   3. Query and transaction functionality for IBC

   NOTE: Most of the commands have aliases that make typing them much quicker 
         (i.e. 'rly tx', 'rly q', etc...)`),
	}

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// reads `homeDir/config/config.yaml` into `var config *Config`
		return initConfig(rootCmd)
	}

	// Register --home flag
	rootCmd.PersistentFlags().StringVar(&homePath, flags.FlagHome, defaultHome, "set home directory")
	if err := viper.BindPFlag(flags.FlagHome, rootCmd.PersistentFlags().Lookup(flags.FlagHome)); err != nil {
		panic(err)
	}

	// Register --debug flag
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "debug output")
	if err := viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		panic(err)
	}

	// Register subcommands
	rootCmd.AddCommand(
		configCmd(),
		chainsCmd(),
		pathsCmd(),
		keysCmd(),
		flags.LineBreak,
		transactionCmd(),
		queryCmd(),
		startCmd(),
		flags.LineBreak,
		getVersionCmd(),
	)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false

	rootCmd := NewRootCmd()
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// readLineFromBuf reads one line from stdin.
func readStdin() (string, error) {
	str, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(str), err
}
