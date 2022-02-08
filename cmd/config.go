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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/relayer/relayer"
	"github.com/cosmos/relayer/relayer/provider"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	lens "github.com/strangelove-ventures/lens/client"
	"gopkg.in/yaml.v3"
)

const (
	// ORDERED is exported channel type constant
	ORDERED = "ordered"
	// UNORDERED is exported channel type constant
	UNORDERED      = "unordered"
	defaultOrder   = ORDERED
	defaultVersion = "ics20-1"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Manage configuration file",
	}

	cmd.AddCommand(
		configShowCmd(),
		configInitCmd(),
		configAddChainsCmd(),
		configAddPathsCmd(),
	)

	return cmd
}

// Command for printing current configuration
func configShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Aliases: []string{"s", "list", "l"},
		Short:   "Prints current configuration",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config show --home %s
$ %s cfg list`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			cfgPath := path.Join(home, "config", "config.yaml")
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if _, err := os.Stat(home); os.IsNotExist(err) {
					return fmt.Errorf("home path does not exist: %s", home)
				}
				return fmt.Errorf("config does not exist: %s", cfgPath)
			}

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}
			yml, err := cmd.Flags().GetBool(flagYAML)
			if err != nil {
				return err
			}
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case jsn:
				out, err := json.Marshal(ConfigToWrapper(config))
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				out, err := yaml.Marshal(ConfigToWrapper(config))
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}
		},
	}

	return yamlFlag(jsonFlag(cmd))
}

// Command for inititalizing an empty config at the --home location
func configInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"i"},
		Short:   "Creates a default home directory at path defined by --home",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config init --home %s
$ %s cfg i`, appName, defaultHome, appName)),
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
			return fmt.Errorf("config already exists: %s", cfgPath)
		},
	}
	return cmd
}

func configAddChainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "add-chains [/path/to/chains/]",
		Args: cobra.ExactArgs(1),
		Short: `Add new chains to the configuration file from a directory full of chain 
              configurations, useful for adding testnet configurations`,
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config add-chains configs/chains`, appName)),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var out *Config
			if out, err = cfgFilesAddChains(args[0]); err != nil {
				return err
			}
			return overWriteConfig(out)
		},
	}

	return cmd
}

func configAddPathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "add-paths [/path/to/paths/]",
		Args: cobra.ExactArgs(1),
		//nolint:lll
		Short: `Add new paths to the configuration file from a directory full of path 
              configurations, useful for adding testnet configurations. 
              NOTE: Chain configuration files must be added before calling this command.`,
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config add-paths configs/paths`, appName)),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var out *Config
			if out, err = cfgFilesAddPaths(args[0]); err != nil {
				return err
			}
			return overWriteConfig(out)
		},
	}

	return cmd
}

func cfgFilesAddChains(dir string) (cfg *Config, err error) {
	dir = path.Clean(dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cfg = config
	for _, f := range files {
		pth := fmt.Sprintf("%s/%s", dir, f.Name())
		if f.IsDir() {
			fmt.Printf("directory at %s, skipping...\n", pth)
			continue
		}

		byt, err := ioutil.ReadFile(pth)
		if err != nil {
			fmt.Printf("failed to read file %s. Err: %v skipping...\n", pth, err)
			continue
		}

		var pcw ProviderConfigWrapper
		if err = json.Unmarshal(byt, &pcw); err != nil {
			fmt.Printf("failed to unmarshal file %s. Err: %v skipping...\n", pth, err)
			continue
		}

		prov, err := pcw.Value.NewProvider(homePath, debug)
		if err != nil {
			fmt.Printf("failed to build ChainProvider for %s. Err: %v \n", pth, err)
			continue
		}

		c := &relayer.Chain{ChainProvider: prov}

		if err = cfg.AddChain(c); err != nil {
			fmt.Printf("failed to add chain %s: %v \n", pth, err)
			continue
		}
		fmt.Printf("added chain %s...\n", c.ChainProvider.ChainId())
	}
	return cfg, nil
}

func cfgFilesAddPaths(dir string) (cfg *Config, err error) {
	dir = path.Clean(dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cfg = config
	for _, f := range files {
		pth := fmt.Sprintf("%s/%s", dir, f.Name())
		if f.IsDir() {
			fmt.Printf("directory at %s, skipping...\n", pth)
			continue
		}

		byt, err := ioutil.ReadFile(pth)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", pth, err)
		}

		p := &relayer.Path{}
		if err = json.Unmarshal(byt, p); err != nil {
			return nil, fmt.Errorf("failed to unmarshal file %s: %w", pth, err)
		}

		// In the case that order isn't added to the path, add it manually
		if p.Src.Order == "" || p.Dst.Order == "" {
			p.Src.Order = defaultOrder
			p.Dst.Order = defaultOrder
		}

		// If the version isn't added to the path, add it manually
		if p.Src.Version == "" {
			p.Src.Version = defaultVersion
		}
		if p.Dst.Version == "" {
			p.Dst.Version = defaultVersion
		}

		pthName := strings.Split(f.Name(), ".")[0]
		if err = config.ValidatePath(p); err != nil {
			return nil, fmt.Errorf("failed to validate path %s: %w", pth, err)
		}

		if err = cfg.AddPath(pthName, p); err != nil {
			return nil, fmt.Errorf("failed to add path %s: %w", pth, err)
		}

		fmt.Printf("added path %s...\n\n", pthName)
	}

	return cfg, nil
}

// ConfigToWrapper converts the Config struct into a ConfigOutputWrapper struct
func ConfigToWrapper(config *Config) *ConfigOutputWrapper {
	cfgw := &ConfigOutputWrapper{Global: config.Global, Paths: config.Paths}
	var providers []*ProviderConfigWrapper
	for _, chain := range config.Chains {
		pcfgw := &ProviderConfigWrapper{
			Type:  chain.ChainProvider.Type(),
			Value: chain.ChainProvider.ProviderConfig(),
		}
		providers = append(providers, pcfgw)
	}
	cfgw.ProviderConfigs = providers
	return cfgw
}

// Config represents the config file for the relayer
type Config struct {
	Global GlobalConfig   `yaml:"global" json:"global"`
	Chains relayer.Chains `yaml:"chains" json:"chains"`
	Paths  relayer.Paths  `yaml:"paths" json:"paths"`
}

// ConfigOutputWrapper is an intermediary type for writing the config to disk and stdout
type ConfigOutputWrapper struct {
	Global          GlobalConfig    `yaml:"global" json:"global"`
	ProviderConfigs ProviderConfigs `yaml:"chains" json:"chains"`
	Paths           relayer.Paths   `yaml:"paths" json:"paths"`
}

// ConfigInputWrapper is an intermediary type for parsing the config.yaml file
type ConfigInputWrapper struct {
	Global          GlobalConfig                 `yaml:"global"`
	ProviderConfigs []*ProviderConfigYAMLWrapper `yaml:"chains"`
	Paths           relayer.Paths                `yaml:"paths"`
}

type ProviderConfigs []*ProviderConfigWrapper

// ProviderConfigWrapper is an intermediary type for parsing arbitrary ProviderConfigs from json files and writing to json/yaml files
type ProviderConfigWrapper struct {
	Type  string                  `yaml:"type"  json:"type"`
	Value provider.ProviderConfig `yaml:"value" json:"value"`
}

// ProviderConfigYAMLWrapper is an intermediary type for parsing arbitrary ProviderConfigs from yaml files
type ProviderConfigYAMLWrapper struct {
	Type  string      `yaml:"type"`
	Value interface{} `yaml:"-"`
}

// UnmarshalJSON adds support for unmarshalling data from an arbitrary ProviderConfig
// NOTE: Add new ProviderConfig types in the map here with the key set equal to the type of ChainProvider (e.g. cosmos, substrate, etc.)
func (pcw *ProviderConfigWrapper) UnmarshalJSON(data []byte) error {
	customTypes := map[string]reflect.Type{
		"cosmos": reflect.TypeOf(lens.ChainClientConfig{}),
	}
	val, err := UnmarshalJSONProviderConfig(data, customTypes)
	if err != nil {
		return err
	}
	pc := val.(provider.ProviderConfig)
	pcw.Value = pc
	return nil
}

// UnmarshalJSONProviderConfig contains the custom unmarshalling logic for ProviderConfig structs
func UnmarshalJSONProviderConfig(data []byte, customTypes map[string]reflect.Type) (interface{}, error) {
	m := map[string]interface{}{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	typeName := m["type"].(string)
	var provCfg provider.ProviderConfig
	if ty, found := customTypes[typeName]; found {
		provCfg = reflect.New(ty).Interface().(provider.ProviderConfig)
	}

	valueBytes, err := json.Marshal(m["value"])
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(valueBytes, &provCfg); err != nil {
		return nil, err
	}

	return provCfg, nil
}

// UnmarshalYAML adds support for unmarshalling data from arbitrary ProviderConfig entries found in the config file
// NOTE: Add logic for new ProviderConfig types in a switch case here
func (iw *ProviderConfigYAMLWrapper) UnmarshalYAML(n *yaml.Node) error {
	type inputWrapper ProviderConfigYAMLWrapper
	type T struct {
		*inputWrapper `yaml:",inline"`
		Wrapper       yaml.Node `yaml:"value"`
	}

	obj := &T{inputWrapper: (*inputWrapper)(iw)}
	if err := n.Decode(obj); err != nil {
		return err
	}

	switch iw.Type {
	case "cosmos":
		iw.Value = new(lens.ChainClientConfig)
	default:
		return fmt.Errorf("%s is an invalid chain type, check your config file", iw.Type)
	}

	return obj.Wrapper.Decode(iw.Value)
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
	APIListenPort  string `yaml:"api-listen-addr" json:"api-listen-addr"`
	Timeout        string `yaml:"timeout" json:"timeout"`
	LightCacheSize int    `yaml:"light-cache-size" json:"light-cache-size"`
}

// newDefaultGlobalConfig returns a global config with defaults set
func newDefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		APIListenPort:  ":5183",
		Timeout:        "10s",
		LightCacheSize: 20,
	}
}

// AddChain adds an additional chain to the config
func (c *Config) AddChain(chain *relayer.Chain) (err error) {
	chainId := chain.ChainProvider.ChainId()
	if chainId == "" {
		return fmt.Errorf("chain ID cannot be empty")
	}
	chn, err := c.Chains.Get(chainId)
	if chn != nil || err == nil {
		return fmt.Errorf("chain with ID %s already exists in config", chainId)
	}
	c.Chains = append(c.Chains, chain)
	return nil
}

func checkPathConflict(pathID, fieldName, oldP, newP string) (err error) {
	if oldP != "" && oldP != newP {
		return fmt.Errorf(
			"path with ID %s and conflicting %s (%s) already exists",
			pathID, fieldName, oldP,
		)
	}
	return nil
}

func checkPathEndConflict(pathID, direction string, oldPe, newPe *relayer.PathEnd) (err error) {
	if err = checkPathConflict(
		pathID, direction+" chain ID",
		oldPe.ChainID, newPe.ChainID); err != nil {
		return err
	}
	if err = checkPathConflict(
		pathID, direction+" client ID",
		oldPe.ClientID, newPe.ClientID); err != nil {
		return err
	}
	if err = checkPathConflict(
		pathID, direction+" connection ID",
		oldPe.ConnectionID, newPe.ConnectionID); err != nil {
		return err
	}
	if err = checkPathConflict(
		pathID, direction+" port ID",
		oldPe.PortID, newPe.PortID); err != nil {
		return err
	}
	if err = checkPathConflict(
		pathID, direction+" order",
		strings.ToLower(oldPe.Order), strings.ToLower(newPe.Order)); err != nil {
		return err
	}
	if err = checkPathConflict(
		pathID, direction+" version",
		oldPe.Version, newPe.Version); err != nil {
		return err
	}
	if err = checkPathConflict(
		pathID, direction+" channel ID",
		oldPe.ChannelID, newPe.ChannelID); err != nil {
		return err
	}
	return nil
}

// AddPath adds an additional path to the config
func (c *Config) AddPath(name string, path *relayer.Path) (err error) {
	// Check if the path does not yet exist.
	oldPath, err := c.Paths.Get(name)
	if err != nil {
		return c.Paths.Add(name, path)
	}
	// Now check if the update would cause any conflicts.
	if err = checkPathEndConflict(name, "source", oldPath.Src, path.Src); err != nil {
		return err
	}
	if err = checkPathEndConflict(name, "destination", oldPath.Dst, path.Dst); err != nil {
		return err
	}
	// Update the existing path.
	*oldPath = *path
	return nil
}

// DeleteChain removes a chain from the config
func (c *Config) DeleteChain(chain string) *Config {
	var set relayer.Chains
	for _, ch := range c.Chains {
		if ch.ChainID() != chain {
			set = append(set, ch)
		}
	}
	c.Chains = set
	return c
}

// validateConfig is used to validate the GlobalConfig values
func validateConfig(c *Config) error {
	_, err := time.ParseDuration(c.Global.Timeout)
	if err != nil {
		return fmt.Errorf("did you remember to run 'rly config init' error:%w", err)
	}

	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command) error {
	home, err := cmd.PersistentFlags().GetString(flags.FlagHome)
	if err != nil {
		return err
	}

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

			// unmarshall them into the wrapper struct
			cfgWrapper := &ConfigInputWrapper{}
			err = yaml.Unmarshal(file, cfgWrapper)
			if err != nil {
				fmt.Println("Error unmarshalling config:", err)
				os.Exit(1)
			}

			// build the config struct
			var chains relayer.Chains
			for _, pcfg := range cfgWrapper.ProviderConfigs {
				prov, err := pcfg.Value.(provider.ProviderConfig).NewProvider(homePath, debug)
				if err != nil {
					return fmt.Errorf("Error while building ChainProviders. Err: %w\n", err)
				}

				chain := &relayer.Chain{ChainProvider: prov}
				chain.Init(nil, debug)
				chains = append(chains, chain)
			}

			config = &Config{
				Global: cfgWrapper.Global,
				Chains: chains,
				Paths:  cfgWrapper.Paths,
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

func overWriteConfig(cfg *Config) (err error) {
	cfgPath := path.Join(homePath, "config", "config.yaml")
	if _, err = os.Stat(cfgPath); err == nil {
		viper.SetConfigFile(cfgPath)
		if err = viper.ReadInConfig(); err == nil {
			// ensure validateConfig runs properly
			err = validateConfig(config)
			if err != nil {
				return err
			}

			// marshal the new config
			out, err := yaml.Marshal(ConfigToWrapper(config))
			if err != nil {
				return err
			}

			// overwrite the config file
			err = ioutil.WriteFile(viper.ConfigFileUsed(), out, 0600)
			if err != nil {
				return err
			}

			// set the global variable
			config = cfg
		}
	}
	return err
}

// ValidatePath checks that a path is valid
func (c *Config) ValidatePath(p *relayer.Path) (err error) {
	if p.Src.Version == "" {
		return fmt.Errorf("source must specify a version")
	}
	if err = c.ValidatePathEnd(p.Src); err != nil {
		return sdkerrors.Wrapf(err, "chain %s failed path validation", p.Src.ChainID)
	}
	if err = c.ValidatePathEnd(p.Dst); err != nil {
		return sdkerrors.Wrapf(err, "chain %s failed path validation", p.Dst.ChainID)
	}
	if p.Src.Order != p.Dst.Order {
		return fmt.Errorf("both sides must have same order ('ORDERED' or 'UNORDERED'), got src(%s) and dst(%s)",
			p.Src.Order, p.Dst.Order)
	}
	return nil
}

// ValidatePathEnd validates provided pathend and returns error for invalid identifiers
func (c *Config) ValidatePathEnd(pe *relayer.PathEnd) error {
	if err := pe.ValidateBasic(); err != nil {
		return err
	}

	chain, err := c.Chains.Get(pe.ChainID)
	if err != nil {
		fmt.Printf("Chain %s is not currently configured. \n", pe.ChainID)
		return nil
	}

	// if the identifiers are empty, don't do any validation
	if pe.ClientID == "" && pe.ConnectionID == "" {
		return nil
	}

	// NOTE: this is just to do validation, the path
	// is not written to the config file
	if err = chain.SetPath(pe); err != nil {
		return err
	}

	height, err := chain.ChainProvider.QueryLatestHeight()
	if err != nil {
		return err
	}

	if pe.ClientID != "" {
		if err := c.ValidateClient(chain, height, pe); err != nil {
			return err
		}

		if pe.ConnectionID != "" {
			if err := c.ValidateConnection(chain, height, pe); err != nil {
				return err
			}

			if pe.ChannelID != "" {
				if err := c.ValidateChannel(chain, height, pe); err != nil {
					return err
				}
			}
		}

		if pe.ConnectionID == "" && pe.ChannelID != "" {
			return fmt.Errorf("connectionID is not configured for the channel: %s", pe.ChannelID)
		}
	}

	if pe.ClientID == "" && pe.ConnectionID != "" {
		return fmt.Errorf("clientID is not configured for the connection: %s", pe.ConnectionID)
	}

	return nil
}

// ValidateClient validates client id in provided pathend
func (c *Config) ValidateClient(chain *relayer.Chain, height int64, pe *relayer.PathEnd) error {
	if err := pe.Vclient(); err != nil {
		return err
	}

	_, err := chain.ChainProvider.QueryClientStateResponse(height, pe.ClientID)
	if err != nil {
		return err
	}

	return nil
}

// ValidateConnection validates connection id in provided pathend
func (c *Config) ValidateConnection(chain *relayer.Chain, height int64, pe *relayer.PathEnd) error {
	if err := pe.Vconn(); err != nil {
		return err
	}

	connection, err := chain.ChainProvider.QueryConnection(height, pe.ConnectionID)
	if err != nil {
		return err
	}

	if connection.Connection.ClientId != pe.ClientID {
		return fmt.Errorf("clientID of connection: %s didn't match with provided ClientID", pe.ConnectionID)
	}

	return nil
}

// ValidateChannel validates channel id in provided pathend
func (c *Config) ValidateChannel(chain *relayer.Chain, height int64, pe *relayer.PathEnd) error {
	if err := pe.Vchan(); err != nil {
		return err
	}

	channel, err := chain.ChainProvider.QueryChannel(height, pe.ChannelID, pe.PortID)
	if err != nil {
		return err
	}

	for _, connection := range channel.Channel.ConnectionHops {
		if connection == pe.ConnectionID {
			return nil
		}
	}

	return fmt.Errorf("connectionID of channel: %s didn't match with provided ConnectionID", pe.ChannelID)
}
