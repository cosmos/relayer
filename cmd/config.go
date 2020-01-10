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

// Config represents a config file for the relayer
type Config struct {
	Global GlobalConfig  `yaml:"global"`
	Chains []ChainConfig `yaml:"chains"`

	c []*relayer.Chain
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	Strategy      string `yaml:"strategy"`
	Timeout       string `yaml:"timeout"`
	LiteCacheSize int    `yaml:"lite-cache-size"`
}

// ChainConfig describes the config necessary for an individual chain
type ChainConfig struct {
	Key            string       `yaml:"key"`
	ChainID        string       `yaml:"chain-id"`
	RPCAddr        string       `yaml:"rpc-addr"`
	AccountPrefix  string       `yaml:"account-prefix"`
	Counterparties []string     `yaml:"counterparties"`
	Gas            uint64       `yaml:"gas,omitempty"`
	GasAdjustment  float64      `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins `yaml:"gas-prices,omitempty"`
	DefaultDenom   string       `yaml:"default-denom,omitempty"`
	Memo           string       `yaml:"memo,omitempty"`
}

func (c Config) setChains() error {
	var out []*relayer.Chain
	for _, i := range c.Chains {
		chain, err := relayer.NewChain(i.Key, i.ChainID, i.RPCAddr, i.AccountPrefix, i.Counterparties, i.Gas, i.GasAdjustment, i.GasPrices, i.DefaultDenom, i.Memo, homePath, c.Global.LiteCacheSize)
		if err != nil {
			fmt.Println("ERROR", err)
			return nil
		}
		out = append(out, chain)
	}
	c.c = out
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

			err = config.setChains()
			if err != nil {
				fmt.Println("Error parsing chain config:", err)
				os.Exit(1)
			}
			fmt.Println("HERE", config.c)
		}
	}
	return nil
}
