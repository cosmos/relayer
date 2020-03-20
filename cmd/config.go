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
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "commands to manage the config file",
	}

	cmd.AddCommand(
		configShowCmd(),
		configInitCmd(),
	)

	return cmd
}

// Command for printing current configuration
func configShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Prints current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			cfgPath := path.Join(home, "config", "config.yaml")
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if _, err := os.Stat(home); os.IsNotExist(err) {
					return fmt.Errorf("Home path does not exist: %s", home)
				}
				return fmt.Errorf("Config does not exist: %s", cfgPath)
			}

			out, err := yaml.Marshal(config)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}

	return cmd
}

// Command for inititalizing an empty config at the --home location
func configInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a default home directory at path defined by --home",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			cfgDir := path.Join(home, "config")
			cfgPath := path.Join(cfgDir, "config.yaml")

			// If the config doesn't exist...
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				// And the config folder doesn't exist...
				if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
					// And the home folder doesn't exist
					if _, err := os.Stat(home); os.IsNotExist(err) {
						// Create the home folder
						if err = os.Mkdir(home, os.ModePerm); err != nil {
							return err
						}
					}
					// Create the home config folder
					if err = os.Mkdir(cfgDir, os.ModePerm); err != nil {
						return err
					}
				}

				// Then create the file...
				f, err := os.Create(cfgPath)
				if err != nil {
					return err
				}
				defer f.Close()

				// And write the default config to that location...
				if _, err = f.Write(defaultConfig()); err != nil {
					return err
				}

				// And return no error...
				return nil
			}

			// Otherwise, the config file exists, and an error is returned...
			return fmt.Errorf("Config already exists: %s", cfgPath)
		},
	}
	return cmd
}

// Config represents the config file for the relayer
type Config struct {
	Global GlobalConfig   `yaml:"global" json:"global"`
	Chains relayer.Chains `yaml:"chains" json:"chains"`
	Paths  relayer.Paths  `yaml:"paths" json:"paths"`
}

// MustYAML returns the yaml string representation of the Paths
func (c Config) MustYAML() []byte {
	out, err := yaml.Marshal(c)
	if err != nil {
		panic(err)
	}
	return out
}

func defaultConfig() []byte {
	return Config{
		Global: newDefaultGlobalConfig(),
		Chains: relayer.Chains{},
		Paths:  relayer.Paths{},
	}.MustYAML()
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	Timeout       string `yaml:"timeout" json:"timeout"`
	LiteCacheSize int    `yaml:"lite-cache-size" json:"lite-cache-size"`
}

// newDefaultGlobalConfig returns a global config with defaults set
func newDefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Timeout:       "10s",
		LiteCacheSize: 20,
	}
}

// AddChain adds an additional chain to the config
func (c *Config) AddChain(chain *relayer.Chain) (cfg *Config, err error) {
	if _, err = c.Chains.Get(chain.ChainID); err == nil {
		return cfg, fmt.Errorf("chain with ID %s already exists in config", chain.ChainID)
	}
	c.Chains = append(c.Chains, chain)
	return c, nil
}

// DeleteChain removes a chain from the config
func (c *Config) DeleteChain(chain string) *Config {
	var set relayer.Chains
	for _, ch := range c.Chains {
		if ch.ChainID != chain {
			set = append(set, ch)
		}
	}
	c.Chains = set
	return c
}

// Called to initialize the relayer.Chain types on Config
func validateConfig(c *Config) error {
	var new = &Config{Global: c.Global, Chains: relayer.Chains{}, Paths: c.Paths}
	to, err := time.ParseDuration(new.Global.Timeout)
	if err != nil {
		return err
	}

	for _, i := range c.Chains {
		chain, err := i.Init(homePath, appCodec, cdc, to, debug)
		if err != nil {
			return err
		}
		new.Chains = append(new.Chains, chain)
	}
	// Reset the config var
	config = new
	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command) error {
	home, err := cmd.PersistentFlags().GetString(flags.FlagHome)
	if err != nil {
		return err
	}

	config = &Config{}
	cfgPath := path.Join(home, "config", "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		viper.SetConfigFile(cfgPath)
		if err := viper.ReadInConfig(); err == nil {
			// read the config file bytes
			file, err := ioutil.ReadFile(viper.ConfigFileUsed())
			if err != nil {
				fmt.Println("Error reading file:", err)
				os.Exit(1)
			}

			// unmarshall them into the struct
			err = yaml.Unmarshal(file, config)
			if err != nil {
				fmt.Println("Error unmarshalling config:", err)
				os.Exit(1)
			}

			// ensure config has []*relayer.Chain used for all chain operations
			err = validateConfig(config)
			if err != nil {
				fmt.Println("Error parsing chain config:", err)
				os.Exit(1)
			}
		}
	}
	return nil
}

func overWriteConfig(cmd *cobra.Command, cfg *Config) error {
	home, err := cmd.Flags().GetString(flags.FlagHome)
	if err != nil {
		return err
	}

	cfgPath := path.Join(home, "config", "config.yaml")
	if _, err = os.Stat(cfgPath); err == nil {
		viper.SetConfigFile(cfgPath)
		if err = viper.ReadInConfig(); err == nil {
			// ensure validateConfig runs properly
			err = validateConfig(config)
			if err != nil {
				return err
			}

			// marshal the new config
			out, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}

			// overwrite the config file
			err = ioutil.WriteFile(viper.ConfigFileUsed(), out, 0666)
			if err != nil {
				return err
			}

			// set the global variable
			config = cfg
		}
	}
	return err
}
