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
	"os"

	"github.com/spf13/cobra"
)

const (
	homeFlag = "home"
	cfgDir   = "config"
	keyDir   = "keys"
	liteDir  = "lite"
	cfgFile  = "config.yaml"
	homeDisc = "set home directory"
	cfgDisc  = "set config file"
)

var (
	cfgPath     string
	homePath    string
	config      *Config
	defaultHome = os.ExpandEnv("$HOME/.relayer")
)

func init() {
	rootCmd.PersistentFlags().StringVar(&homePath, homeFlag, defaultHome, homeDisc)
	rootCmd.PersistentFlags().StringVar(&cfgPath, cfgDir, cfgFile, cfgDisc)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "relayer",
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
