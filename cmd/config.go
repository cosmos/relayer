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
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"gopkg.in/yaml.v2"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "commands to manage the config file",
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
	Global GlobalConfig  `yaml:"global" json:"global"`
	Chains ChainConfigs  `yaml:"chains" json:"chains"`
	Paths  relayer.Paths `yaml:"paths" json:"paths"`

	c relayer.Chains
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
		Chains: ChainConfigs{},
		Paths:  relayer.Paths{},
	}.MustYAML()
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	Strategy      string `yaml:"strategy" json:"strategy"`
	Timeout       string `yaml:"timeout" json:"timeout"`
	LiteCacheSize int    `yaml:"lite-cache-size" json:"lite-cache-size"`
}

// newDefaultGlobalConfig returns a global config with defaults set
func newDefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Strategy:      "naieve",
		Timeout:       "10s",
		LiteCacheSize: 20,
	}
}

// ChainConfigs is a collection of ChainConfig
type ChainConfigs []ChainConfig

// Get returns a chain config with a given ID
// TODO: Add error handling here
func (c ChainConfigs) Get(cid string) *ChainConfig {
	for _, chain := range c {
		if chain.ChainID == cid {
			return &chain
		}
	}
	return nil
}

// AddChain adds an additional chain to the config
func (c *Config) AddChain(chain ChainConfig) (*Config, error) {
	if c.Chains.Get(chain.ChainID) != nil {
		return nil, fmt.Errorf("chain with ID %s already exists in config", chain.ChainID)
	}
	c.Chains = append(c.Chains, chain)
	return c, nil
}

// DeleteChain removes a chain from the config
func (c *Config) DeleteChain(chain string) *Config {
	var set ChainConfigs
	for _, ch := range c.Chains {
		if ch.ChainID != chain {
			set = append(set, ch)
		}
	}
	c.Chains = set
	return c
}

// AddPath adds a path to the config file
func (c *Config) AddPath(path relayer.Path) (*Config, error) {
	if c.Paths.Duplicate(path) {
		return nil, fmt.Errorf("an equivelent path exists in the config")
	}
	c.Paths = append(c.Paths, path)
	return c, nil
}

// DeletePath removes a path at index i
func (c *Config) DeletePath(i int) *Config {
	c.Paths = append(c.Paths[:i], c.Paths[i+1:]...)
	return c
}

// ChainConfig describes the config necessary for an individual chain
type ChainConfig struct {
	Key            string  `yaml:"key" json:"key"`
	ChainID        string  `yaml:"chain-id" json:"chain-id"`
	RPCAddr        string  `yaml:"rpc-addr" json:"rpc-addr"`
	AccountPrefix  string  `yaml:"account-prefix" json:"account-prefix"`
	Gas            uint64  `yaml:"gas,omitempty" json:"gas,omitempty"`
	GasAdjustment  float64 `yaml:"gas-adjustment,omitempty" json:"gas-adjustment,omitempty"`
	GasPrices      string  `yaml:"gas-prices,omitempty" json:"gas-prices,omitempty"`
	DefaultDenom   string  `yaml:"default-denom,omitempty" json:"default-denom,omitempty"`
	Memo           string  `yaml:"memo,omitempty" json:"memo,omitempty"`
	TrustingPeriod string  `yaml:"trusting-period" json:"trusting-period"`
}

// Update returns
func (c ChainConfig) Update(key, value string) (out ChainConfig, err error) {
	out = c
	switch key {
	case "key":
		out.Key = value
	case "chain-id":
		out.ChainID = value
	case "rpc-addr":
		if _, err = rpcclient.NewHTTP(value, "/websocket"); err != nil {
			return
		}
		out.RPCAddr = value
	case "account-prefix":
		out.AccountPrefix = value
	case "gas":
		var gas uint64
		gas, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return
		}
		out.Gas = gas
	case "gas-prices":
		if _, err = sdk.ParseDecCoins(value); err != nil {
			return
		}
		out.GasPrices = value
	case "default-denom":
		out.DefaultDenom = value
	case "memo":
		out.Memo = value
	case "trusting-period":
		if _, err = time.ParseDuration(value); err != nil {
			return
		}
		out.TrustingPeriod = value
	default:
		return out, fmt.Errorf("key %s not found", key)
	}

	return
}

// Called to set the relayer.Chain types on Config
func setChains(c *Config, home string) error {
	var out []*relayer.Chain
	var new = &Config{Global: c.Global, Chains: c.Chains, Paths: c.Paths}
	for _, i := range c.Chains {
		homeDir := path.Join(home, "lite")
		chain, err := relayer.NewChain(i.Key, i.ChainID, i.RPCAddr,
			i.AccountPrefix, i.Gas, i.GasAdjustment, i.GasPrices,
			i.DefaultDenom, i.Memo, homePath, c.Global.LiteCacheSize,
			i.TrustingPeriod, homeDir, appCodec, cdc)
		if err != nil {
			return err
		}
		out = append(out, chain)
	}
	new.c = out
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
			err = setChains(config, home)
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
			// ensure setChains runs properly
			err = setChains(config, home)
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
