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

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Config represents the config file for the relayer
type Config struct {
	Global GlobalConfig  `yaml:"global"`
	Chains []ChainConfig `yaml:"chains"`

	c relayer.Chains
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	Strategy      string `yaml:"strategy"`
	Timeout       string `yaml:"timeout"`
	LiteCacheSize int    `yaml:"lite-cache-size"`
}

// ChainConfig describes the config necessary for an individual chain
// TODO: Are there additional parameters needed here
type ChainConfig struct {
	Key            string               `yaml:"key"`
	ChainID        string               `yaml:"chain-id"`
	RPCAddr        string               `yaml:"rpc-addr"`
	AccountPrefix  string               `yaml:"account-prefix"`
	Counterparties []CounterpartyConfig `yaml:"counterparties"`
	Gas            uint64               `yaml:"gas,omitempty"`
	GasAdjustment  float64              `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins         `yaml:"gas-prices,omitempty"`
	DefaultDenom   string               `yaml:"default-denom,omitempty"`
	Memo           string               `yaml:"memo,omitempty"`
	TrustingPeriod string               `yaml:"trusting-period"`
}

// CounterpartyConfig represents a chain's counterparty
type CounterpartyConfig struct {
	ChainID  string `yaml:"chain-id"`
	ClientID string `yaml:"client-id"`
}

// Called to set the relayer.Chain types on Config
func setChains(c *Config, home string) error {
	var out []*relayer.Chain
	var new = &Config{Global: c.Global, Chains: c.Chains}
	for _, i := range c.Chains {
		var cps []relayer.Counterparty
		for _, cp := range i.Counterparties {
			cps = append(cps, relayer.NewCounterparty(cp.ChainID, cp.ClientID))
		}
		homeDir := path.Join(home, i.ChainID)
		chain, err := relayer.NewChain(i.Key, i.ChainID, i.RPCAddr, i.AccountPrefix, cps, i.Gas, i.GasAdjustment,
			i.GasPrices, i.DefaultDenom, i.Memo, homePath, c.Global.LiteCacheSize, i.TrustingPeriod,
			homeDir)
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
	home, err := cmd.PersistentFlags().GetString(homeFlag)
	if err != nil {
		return err
	}

	config = &Config{}
	cfgPath := path.Join(home, cfgDir, cfgFile)
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
