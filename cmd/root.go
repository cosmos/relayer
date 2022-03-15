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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	MB      = 1024 * 1024 // in bytes
	appName = "rly"
)

var defaultHome = filepath.Join(os.Getenv("HOME"), ".relayer")

const (
	// Default identifiers for dummy usage
	dcli = "defaultclientid"
	dcon = "defaultconnectionid"
	dcha = "defaultchannelid"
	dpor = "defaultportid"
	dord = "ordered"
)

// NewRootCmd returns the root command for relayer.
func NewRootCmd() *cobra.Command {
	// Use a local app state instance scoped to the new root command,
	// so that tests don't concurrently access the state.
	a := &appState{
		Viper: viper.New(),
	}

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
		// reads `homeDir/config/config.yaml` into `a.Config`
		return initConfig(rootCmd, a)
	}

	// Register --home flag
	rootCmd.PersistentFlags().StringVar(&a.HomePath, flags.FlagHome, defaultHome, "set home directory")
	if err := a.Viper.BindPFlag(flags.FlagHome, rootCmd.PersistentFlags().Lookup(flags.FlagHome)); err != nil {
		panic(err)
	}

	// Register --debug flag
	rootCmd.PersistentFlags().BoolVarP(&a.Debug, "debug", "d", false, "debug output")
	if err := a.Viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		panic(err)
	}

	// Register subcommands
	rootCmd.AddCommand(
		configCmd(a),
		chainsCmd(a),
		pathsCmd(a),
		keysCmd(a),
		lineBreakCommand(),
		transactionCmd(a),
		queryCmd(a),
		startCmd(a),
		lineBreakCommand(),
		getVersionCmd(a),
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

// readLine reads one line from the given reader.
func readLine(in io.Reader) (string, error) {
	str, err := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(str), err
}

// lineBreakCommand returns a new instance of a command to be used as a line break
// in a command's help output.
//
// This is not a plain reference to flags.LineBreak,
// because that is a global value that will be modified by concurrent tests,
// causing a data race.
func lineBreakCommand() *cobra.Command {
	var cmd cobra.Command = *flags.LineBreak
	return &cmd
}
