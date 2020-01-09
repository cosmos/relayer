package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/crypto/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Config represents a config file for the relayer
type Config struct {
	Global GlobalConfig  `yaml:"global"`
	Chains []ChainConfig `yaml:"chains"`
}

// Chain returns the configuration for a given chain
func (c Config) Chain(chainID string) (ChainConfig, error) {
	for _, chain := range c.Chains {
		if chainID == chain.ChainID {
			return chain, nil
		}
	}
	return ChainConfig{}, NewChainDoesNotExistError(chainID)
}

// Exists Returns true if the chain is configured
func (c Config) Exists(chainID string) bool {
	for _, chain := range c.Chains {
		if chainID == chain.ChainID {
			return true
		}
	}
	return false
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	Timeout string `yaml:"timeout"`
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

// Keyring returns the test keyring associated with this Chain
func (c ChainConfig) Keyring() (keys.Keybase, error) {
	out, err := keys.NewTestKeyring(c.ChainID, path.Join(homePath, keyDir, c.ChainID))
	if err != nil {
		return nil, err
	}
	return out, nil
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
		}
	}
	return nil
}
