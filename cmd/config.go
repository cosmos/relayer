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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/chains/penumbra"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func configCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Manage configuration file",
	}

	cmd.AddCommand(
		configShowCmd(a),
		configInitCmd(a),
	)
	return cmd
}

// Command for printing current configuration
func configShowCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Aliases: []string{"s", "list", "l"},
		Short:   "Prints current configuration",
		Args:    withUsage(cobra.NoArgs),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config show --home %s
$ %s cfg list`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flagHome)
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
				out, err := json.Marshal(a.config.Wrapped())
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				out, err := yaml.Marshal(a.config.Wrapped())
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
		},
	}

	return yamlFlag(a.viper, jsonFlag(a.viper, cmd))
}

// Command for initializing an empty config at the --home location
func configInitCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"i"},
		Short:   "Creates a default home directory at path defined by --home",
		Args:    withUsage(cobra.NoArgs),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config init --home %s
$ %s cfg i`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := cmd.Flags().GetString(flagHome)
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

				memo, _ := cmd.Flags().GetString(flagMemo)

				// And write the default config to that location...
				if _, err = f.Write(defaultConfigYAML(memo)); err != nil {
					return err
				}

				// And return no error...
				return nil
			}

			// Otherwise, the config file exists, and an error is returned...
			return fmt.Errorf("config already exists: %s", cfgPath)
		},
	}
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

// addChainsFromDirectory finds all JSON-encoded config files in dir,
// and optimistically adds them to a's chains.
//
// If any files fail to parse or otherwise are not able to be added to a's chains,
// the error is logged.
// An error is only returned if the directory cannot be read at all.
func addChainsFromDirectory(ctx context.Context, stderr io.Writer, a *appState, dir string) error {
	dir = path.Clean(dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	return a.performConfigLockingOperation(ctx, func() error {
		for _, f := range files {
			pth := filepath.Join(dir, f.Name())
			if f.IsDir() {
				fmt.Fprintf(stderr, "directory at %s, skipping...\n", pth)
				continue
			}

			byt, err := os.ReadFile(pth)
			if err != nil {
				fmt.Fprintf(stderr, "failed to read file %s. Err: %v skipping...\n", pth, err)
				continue
			}

			var pcw ProviderConfigWrapper
			if err = json.Unmarshal(byt, &pcw); err != nil {
				fmt.Fprintf(stderr, "failed to unmarshal file %s. Err: %v skipping...\n", pth, err)
				continue
			}
			chainName := strings.Split(f.Name(), ".")[0]
			prov, err := pcw.Value.NewProvider(
				a.log.With(zap.String("provider_type", pcw.Type)),
				a.homePath, a.debug, chainName,
			)
			if err != nil {
				fmt.Fprintf(stderr, "failed to build ChainProvider for %s. Err: %v \n", pth, err)
				continue
			}

			c := relayer.NewChain(a.log, prov, a.debug)
			if err = a.config.AddChain(c); err != nil {
				fmt.Fprintf(stderr, "failed to add chain %s: %v \n", pth, err)
				continue
			}
			fmt.Fprintf(stderr, "added chain %s...\n", c.ChainProvider.ChainId())
		}
		return nil
	})
}

// addPathsFromDirectory parses all the files containing JSON-encoded paths in dir,
// and it adds them to a's paths.
//
// addPathsFromDirectory returns the first error encountered,
// which means a's paths may include a subset of the path files in dir.
func addPathsFromDirectory(ctx context.Context, stderr io.Writer, a *appState, dir string) error {
	dir = path.Clean(dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	return a.performConfigLockingOperation(ctx, func() error {
		for _, f := range files {
			pth := filepath.Join(dir, f.Name())
			if f.IsDir() {
				fmt.Fprintf(stderr, "directory at %s, skipping...\n", pth)
				continue
			}

			byt, err := os.ReadFile(pth)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", pth, err)
			}

			p := &relayer.Path{}
			if err = json.Unmarshal(byt, p); err != nil {
				return fmt.Errorf("failed to unmarshal file %s: %w", pth, err)
			}

			pthName := strings.Split(f.Name(), ".")[0]
			if err := a.config.ValidatePath(ctx, stderr, p); err != nil {
				return fmt.Errorf("failed to validate path %s: %w", pth, err)
			}

			if err := a.config.AddPath(pthName, p); err != nil {
				return fmt.Errorf("failed to add path %s: %w", pth, err)
			}

			fmt.Fprintf(stderr, "added path %s...\n\n", pthName)
		}

		return nil
	})
}

// Wrapped converts the Config struct into a ConfigOutputWrapper struct
func (c *Config) Wrapped() *ConfigOutputWrapper {
	providers := make(ProviderConfigs)
	for _, chain := range c.Chains {
		pcfgw := &ProviderConfigWrapper{
			Type:  chain.ChainProvider.Type(),
			Value: chain.ChainProvider.ProviderConfig(),
		}
		providers[chain.ChainProvider.ChainName()] = pcfgw
	}
	return &ConfigOutputWrapper{Global: c.Global, ProviderConfigs: providers, Paths: c.Paths}
}

// rlyMemo returns a formatted message memo string
// that includes "rly" and the version, e.g. "rly(v2.3.0)"
// or "My custom memo | rly(v2.3.0)"
func rlyMemo(memo string) string {
	if memo == "-" {
		// omit memo entirely
		return ""
	}
	defaultMemo := fmt.Sprintf("rly(%s)", Version)
	if memo == "" {
		return defaultMemo
	}
	return fmt.Sprintf("%s | %s", memo, defaultMemo)
}

// memo returns a formatted message memo string,
// provided either by the memo flag or the config.
func (c *Config) memo(cmd *cobra.Command) string {
	memoFlag, _ := cmd.Flags().GetString(flagMemo)
	if memoFlag != "" {
		return rlyMemo(memoFlag)
	}

	return rlyMemo(c.Global.Memo)
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
	Global          GlobalConfig                          `yaml:"global"`
	ProviderConfigs map[string]*ProviderConfigYAMLWrapper `yaml:"chains"`
	Paths           relayer.Paths                         `yaml:"paths"`
}

// RuntimeConfig converts the input disk config into the relayer runtime config.
func (c *ConfigInputWrapper) RuntimeConfig(ctx context.Context, a *appState) (*Config, error) {
	// build providers for each chain
	chains := make(relayer.Chains)
	for chainName, pcfg := range c.ProviderConfigs {
		prov, err := pcfg.Value.(provider.ProviderConfig).NewProvider(
			a.log.With(zap.String("provider_type", pcfg.Type)),
			a.homePath, a.debug, chainName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build ChainProviders: %w", err)
		}

		if err := prov.Init(ctx); err != nil {
			return nil, fmt.Errorf("failed to initialize provider: %w", err)
		}

		chain := relayer.NewChain(a.log, prov, a.debug)
		chains[chainName] = chain
	}

	return &Config{
		Global: c.Global,
		Chains: chains,
		Paths:  c.Paths,
	}, nil
}

type ProviderConfigs map[string]*ProviderConfigWrapper

// ProviderConfigWrapper is an intermediary type for parsing arbitrary ProviderConfigs from json files and writing to json/yaml files
type ProviderConfigWrapper struct {
	Type  string                  `yaml:"type"  json:"type"`
	Value provider.ProviderConfig `yaml:"value" json:"value"`
}

// ProviderConfigYAMLWrapper is an intermediary type for parsing arbitrary ProviderConfigs from yaml files
type ProviderConfigYAMLWrapper struct {
	Type  string `yaml:"type"`
	Value any    `yaml:"-"`
}

// UnmarshalJSON adds support for unmarshalling data from an arbitrary ProviderConfig
// NOTE: Add new ProviderConfig types in the map here with the key set equal to the type of ChainProvider (e.g. cosmos, substrate, etc.)
func (pcw *ProviderConfigWrapper) UnmarshalJSON(data []byte) error {
	customTypes := map[string]reflect.Type{
		"cosmos":   reflect.TypeOf(cosmos.CosmosProviderConfig{}),
		"penumbra": reflect.TypeOf(penumbra.PenumbraProviderConfig{}),
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
func UnmarshalJSONProviderConfig(data []byte, customTypes map[string]reflect.Type) (any, error) {
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	typeName, ok := m["type"].(string)
	if !ok {
		return nil, errors.New("cannot find type")
	}

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
		iw.Value = new(cosmos.CosmosProviderConfig)
	case "penumbra":
		iw.Value = new(penumbra.PenumbraProviderConfig)
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
	chains, err := c.Chains.Gets(src, dst)
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

func defaultConfigYAML(memo string) []byte {
	return DefaultConfig(memo).MustYAML()
}

func DefaultConfig(memo string) *Config {
	return &Config{
		Global: newDefaultGlobalConfig(memo),
		Chains: make(relayer.Chains),
		Paths:  make(relayer.Paths),
	}
}

// GlobalConfig describes any global relayer settings
type GlobalConfig struct {
	APIListenPort  string `yaml:"api-listen-addr" json:"api-listen-addr"`
	Timeout        string `yaml:"timeout" json:"timeout"`
	Memo           string `yaml:"memo" json:"memo"`
	LightCacheSize int    `yaml:"light-cache-size" json:"light-cache-size"`
}

// newDefaultGlobalConfig returns a global config with defaults set
func newDefaultGlobalConfig(memo string) GlobalConfig {
	return GlobalConfig{
		APIListenPort:  ":5183",
		Timeout:        "10s",
		LightCacheSize: 20,
		Memo:           memo,
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
	c.Chains[chain.ChainProvider.ChainName()] = chain
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

	return nil
}

// AddPath adds an additional path to the config
func (c *Config) AddPath(name string, path *relayer.Path) (err error) {
	// Ensure path is initialized.
	if c.Paths == nil {
		c.Paths = make(relayer.Paths)
	}
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

// DeleteChain modifies c in-place to remove any chains that have the given name.
func (c *Config) DeleteChain(chain string) {
	delete(c.Chains, chain)
}

// validateConfig is used to validate the GlobalConfig values
func (c *Config) validateConfig() error {
	_, err := time.ParseDuration(c.Global.Timeout)
	if err != nil {
		return fmt.Errorf("did you remember to run 'rly config init' error:%w", err)
	}

	// verify that the channel filter rule is valid for every path in the config
	for _, p := range c.Paths {
		if err := p.ValidateChannelFilterRule(); err != nil {
			return fmt.Errorf("error initializing the relayer config for path %s: %w", p.String(), err)
		}
	}

	return nil
}

// ValidatePath checks that a path is valid
func (c *Config) ValidatePath(ctx context.Context, stderr io.Writer, p *relayer.Path) (err error) {
	if err = c.ValidatePathEnd(ctx, stderr, p.Src); err != nil {
		return fmt.Errorf("chain %s failed path validation: %w", p.Src.ChainID, err)
	}
	if err = c.ValidatePathEnd(ctx, stderr, p.Dst); err != nil {
		return fmt.Errorf("chain %s failed path validation: %w", p.Dst.ChainID, err)
	}
	return nil
}

// ValidatePathEnd validates provided pathend and returns error for invalid identifiers
func (c *Config) ValidatePathEnd(ctx context.Context, stderr io.Writer, pe *relayer.PathEnd) error {
	chain, err := c.Chains.Get(pe.ChainID)
	if err != nil {
		fmt.Fprintf(stderr, "Chain %s is not currently configured.\n", pe.ChainID)
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

	height, err := chain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}

	if pe.ClientID != "" {
		if err := c.ValidateClient(ctx, chain, height, pe); err != nil {
			return err
		}

		if pe.ConnectionID != "" {
			if err := c.ValidateConnection(ctx, chain, height, pe); err != nil {
				return err
			}
		}
	}

	if pe.ClientID == "" && pe.ConnectionID != "" {
		return fmt.Errorf("clientID is not configured for the connection: %s", pe.ConnectionID)
	}

	return nil
}

// ValidateClient validates client id in provided pathend
func (c *Config) ValidateClient(ctx context.Context, chain *relayer.Chain, height int64, pe *relayer.PathEnd) error {
	if err := pe.Vclient(); err != nil {
		return err
	}

	_, err := chain.ChainProvider.QueryClientStateResponse(ctx, height, pe.ClientID)
	if err != nil {
		return err
	}

	return nil
}

// ValidateConnection validates connection id in provided pathend
func (c *Config) ValidateConnection(ctx context.Context, chain *relayer.Chain, height int64, pe *relayer.PathEnd) error {
	if err := pe.Vconn(); err != nil {
		return err
	}

	connection, err := chain.ChainProvider.QueryConnection(ctx, height, pe.ConnectionID)
	if err != nil {
		return err
	}

	if connection.Connection.ClientId != pe.ClientID {
		return fmt.Errorf("clientID of connection: %s didn't match with provided ClientID", pe.ConnectionID)
	}

	return nil
}
