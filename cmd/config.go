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
	"io/ioutil"
	"os"
	"path"
	"strings"
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
		Short:   "manage configuration file",
	}

	cmd.AddCommand(
		configShowCmd(),
		configInitCmd(),
		configAddDirCmd(),
	)

	return cmd
}

// Command for printing current configuration
func configShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Aliases: []string{"s", "list", "l"},
		Short:   "Prints current configuration",
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
		Use:     "init",
		Aliases: []string{"i"},
		Short:   "Creates a default home directory at path defined by --home",
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

func configAddDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add-dir [dir]",
		Aliases: []string{"ad"},
		Short:   "Add new chains and paths to the configuration file from a directory full of chain and path configuration, useful for adding testnet configurations",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var out *Config
			if out, err = cfgFilesAdd(args[0]); err != nil {
				return err
			}
			return overWriteConfig(cmd, out)
		},
	}

	return cmd
}

func cfgFilesAdd(dir string) (cfg *Config, err error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cfg = config
	for _, f := range files {
		c := &relayer.Chain{}
		pth := fmt.Sprintf("%s%s", dir, f.Name())
		if f.IsDir() {
			fmt.Printf("directory at %s, skipping...\n", pth)
			continue
		}

		byt, err := ioutil.ReadFile(pth)
		if err != nil {
			fmt.Printf("failed to read file %s, skipping...\n", pth)
			continue
		}

		if err = json.Unmarshal(byt, c); err != nil {
			fmt.Printf("failed to unmarshal file %s, skipping...\n", pth)
			continue
		}

		if c.ChainID == "" && c.Key == "" && c.RPCAddr == "" {
			p := &relayer.Path{}
			if err = json.Unmarshal(byt, p); err != nil {
				fmt.Printf("failed to unmarshal file %s, skipping...\n", pth)
			}

			// In the case that order isn't added to the path, add it manually
			if p.Src.Order == "" || p.Dst.Order == "" {
				p.Src.Order = "ORDERED"
				p.Dst.Order = "ORDERED"
			}

			pthName := strings.Split(f.Name(), ".")[0]
			if err = cfg.AddPath(pthName, p); err != nil {
				fmt.Printf("%s: %s\n", pth, err.Error())
				continue
			}

			if err = p.Validate(); err == nil {
				fmt.Printf("added path %s...\n", pthName)
				continue
			} else if err != nil {
				fmt.Printf("%s did not contain valid path config, skipping...\n", pth)
				continue
			}
		}

		if err = cfg.AddChain(c); err != nil {
			fmt.Printf("%s: %s\n", pth, err.Error())
			continue
		}
		fmt.Printf("added chain %s...\n", c.ChainID)
	}
	return cfg, nil
}

// Config represents the config file for the relayer
type Config struct {
	Global GlobalConfig   `yaml:"global" json:"global"`
	Chains relayer.Chains `yaml:"chains" json:"chains"`
	Paths  relayer.Paths  `yaml:"paths" json:"paths"`
}

// ChainsFromPath takes the path name and returns the properly configured chains
func (c *Config) ChainsFromPath(path string) (map[string]*relayer.Chain, string, string, error) {
	pth, err := c.Paths.Get(path)
	if err != nil {
		return nil, "", "", err
	}

	src, dst := pth.Src.ChainID, pth.Dst.ChainID
	chains, err := config.Chains.Gets(src, dst)
	if err != nil {
		return nil, "", "", err
	}

	if err = chains[src].SetPath(pth.Src); err != nil {
		return nil, "", "", err
	}
	if err = chains[dst].SetPath(pth.Dst); err != nil {
		return nil, "", "", err
	}

	return chains, src, dst, nil
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
func (c *Config) AddChain(chain *relayer.Chain) (err error) {
	chn, err := c.Chains.Get(chain.ChainID)
	if chn == nil || err == nil {
		return fmt.Errorf("chain with ID %s already exists in config", chain.ChainID)
	}
	c.Chains = append(c.Chains, chain)
	return nil
}

// AddPath adds an additional path to the config
func (c *Config) AddPath(name string, path *relayer.Path) (err error) {
	return c.Paths.Add(name, path)
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
	to, err := time.ParseDuration(config.Global.Timeout)
	if err != nil {
		return err
	}

	for _, i := range c.Chains {
		if err := i.Init(homePath, appCodec, cdc, to, debug); err != nil {
			return err
		}
	}

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
