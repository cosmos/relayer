package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/cosmos/relayer/relayer"
	"github.com/cosmos/relayer/relayer/provider/cosmos"
	"github.com/spf13/cobra"
	registry "github.com/strangelove-ventures/lens/client/chain_registry"
	"gopkg.in/yaml.v3"
)

const (
	check = "✔"
	xIcon = "✘"
)

func chainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chains",
		Aliases: []string{"ch"},
		Short:   "Manage chain configurations",
	}

	cmd.AddCommand(
		chainsListCmd(),
		chainsRegistryList(),
		chainsDeleteCmd(),
		chainsAddCmd(),
		chainsShowCmd(),
		chainsAddrCmd(),
		chainsAddDirCmd(),
	)

	return cmd
}

func chainsAddrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "address [chain-id]",
		Aliases: []string{"addr"},
		Short:   "Returns a chain's configured key's address",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains address ibc-0
$ %s ch addr ibc-0`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			address, err := chain.ChainProvider.ShowAddress(chain.ChainProvider.Key())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), address)
			return nil
		},
	}

	return cmd
}

func chainsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show [chain-id]",
		Aliases: []string{"s"},
		Short:   "Returns a chain's configuration data",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains show ibc-0 --json
$ %s chains show ibc-0 --yaml
$ %s ch s ibc-0 --json
$ %s ch s ibc-0 --yaml`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}
			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}
			switch {
			case jsn:
				pcfgw := &ProviderConfigWrapper{
					Type:  c.ChainProvider.Type(),
					Value: c.ChainProvider.ProviderConfig(),
				}
				out, err := json.Marshal(pcfgw)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				pcfgw := &ProviderConfigWrapper{
					Type:  c.ChainProvider.Type(),
					Value: c.ChainProvider.ProviderConfig(),
				}
				out, err := yaml.Marshal(pcfgw)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
		},
	}
	return jsonFlag(cmd)
}

func chainsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [chain-id]",
		Aliases: []string{"d"},
		Short:   "Returns chain configuration data",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains delete ibc-0
$ %s ch d ibc-0`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return overWriteConfig(config.DeleteChain(args[0]))
		},
	}
	return cmd
}

func chainsRegistryList() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "registry-list",
		Args:    cobra.NoArgs,
		Aliases: []string{"rl"},
		Short:   "List chains available for configuration from the registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			yml, err := cmd.Flags().GetBool(flagYAML)
			if err != nil {
				return err
			}

			chains, err := registry.DefaultChainRegistry().ListChains()
			if err != nil {
				return err
			}

			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(chains)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(chains)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				for _, chain := range chains {
					fmt.Fprintln(cmd.OutOrStdout(), chain)
				}
			}
			return nil
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func chainsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "Returns chain configuration data",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains list
$ %s ch l`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			yml, err := cmd.Flags().GetBool(flagYAML)
			if err != nil {
				return err
			}

			configs := ConfigToWrapper(config).ProviderConfigs
			if len(configs) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: no chains found (do you need to run 'rly chains add'?)")
			}

			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(configs)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(configs)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				for i, c := range config.Chains {
					var (
						key = xIcon
						p   = xIcon
						bal = xIcon
					)
					// check that the key from config.yaml is set in keychain
					if c.ChainProvider.KeyExists(c.ChainProvider.Key()) {
						key = check
					}

					coins, err := c.ChainProvider.QueryBalance(c.ChainProvider.Key())
					if err == nil && !coins.Empty() {
						bal = check
					}

					for _, pth := range config.Paths {
						if pth.Src.ChainID == c.ChainProvider.ChainId() || pth.Dst.ChainID == c.ChainID() {
							p = check
						}
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%2d: %-20s -> type(%s) key(%s) bal(%s) path(%s)\n", i, c.ChainID(), c.ChainProvider.Type(), key, bal, p)
				}
				return nil
			}
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func chainsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [[chain-name]]",
		Aliases: []string{"a"},
		Short: "Add a new chain to the configuration file by fetching chain metadata from \n" +
			"                the chain-registry or passing a file (-f) or url (-u)",
		Args: cobra.MinimumNArgs(0),
		Example: fmt.Sprintf(` $ %s chains add cosmoshub
 $ %s chains add cosmoshub osmosis
 $ %s chains add --file chains/ibc0.json
 $ %s chains add --url https://relayer.com/ibc0.json`, appName, appName, appName, appName),
		RunE: func(cmd *cobra.Command, args []string) error {
			var out *Config

			file, url, err := getAddInputs(cmd)
			if err != nil {
				return err
			}

			// default behavior fetch from chain registry
			// still allow for adding config from url or file
			switch {
			case file != "":
				if out, err = fileInputAdd(file); err != nil {
					return err
				}
			case url != "":
				if out, err = urlInputAdd(url); err != nil {
					return err
				}
			default:
				if out, err = chainRegistryAdd(args); err != nil {
					return err
				}
			}

			if err = validateConfig(out); err != nil {
				return err
			}

			return overWriteConfig(out)
		},
	}

	return chainsAddFlags(cmd)
}

func chainsAddDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add-dir [dir]",
		Aliases: []string{"ad"},
		Args:    cobra.ExactArgs(1),
		Short: `Add new chains to the configuration file from a directory 
		full of chain configuration, useful for adding testnet configurations`,
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains add-dir testnet/chains/
$ %s ch ad testnet/chains/`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var out *Config
			if out, err = cfgFilesAddChains(cmd, args[0]); err != nil {
				return err
			}
			return overWriteConfig(out)
		},
	}

	return cmd
}

func fileInputAdd(file string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	var pcw ProviderConfigWrapper
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, &pcw); err != nil {
		return nil, err
	}

	prov, err := pcw.Value.NewProvider(homePath, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to build ChainProvider for %s: %w", file, err)
	}

	c := &relayer.Chain{ChainProvider: prov}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}

	return config, nil
}

// urlInputAdd validates a chain config URL and fetches its contents
func urlInputAdd(rawurl string) (cfg *Config, err error) {
	u, err := url.Parse(rawurl)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return cfg, errors.New("invalid URL")
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pcw ProviderConfigWrapper
	d := json.NewDecoder(resp.Body)
	d.DisallowUnknownFields()
	err = d.Decode(&pcw)
	if err != nil {
		return cfg, err
	}

	// build the ChainProvider before initializing the chain
	prov, err := pcw.Value.NewProvider(homePath, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to build ChainProvider for %s: %w", rawurl, err)
	}

	c := &relayer.Chain{ChainProvider: prov}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}
	return config, err
}

func chainRegistryAdd(chains []string) (*Config, error) {
	chainRegistry := registry.DefaultChainRegistry()
	allChains, err := chainRegistry.ListChains()
	if err != nil {
		return nil, err
	}

	for _, chain := range chains {
		found := false
		for _, possibleChain := range allChains {
			if chain == possibleChain {
				found = true
			}

			if !found {
				log.Printf("unable to find chain %s in %s", chain, chainRegistry.SourceLink())
				continue
			}

			chainInfo, err := chainRegistry.GetChain(chain)
			if err != nil {
				log.Printf("error getting chain: %s", err)
				continue
			}

			chainConfig, err := chainInfo.GetChainConfig()
			if err != nil {
				log.Printf("error generating chain config: %s", err)
				continue
			}

			// build the ChainProvider
			pcfg := &cosmos.CosmosProviderConfig{
				Key:            chainConfig.Key,
				ChainID:        chainConfig.ChainID,
				RPCAddr:        chainConfig.RPCAddr,
				AccountPrefix:  chainConfig.AccountPrefix,
				KeyringBackend: chainConfig.KeyringBackend,
				GasAdjustment:  chainConfig.GasAdjustment,
				GasPrices:      chainConfig.GasPrices,
				Debug:          chainConfig.Debug,
				Timeout:        chainConfig.Timeout,
				OutputFormat:   chainConfig.OutputFormat,
				SignModeStr:    chainConfig.SignModeStr,
			}

			prov, err := pcfg.NewProvider(homePath, debug)
			if err != nil {
				log.Printf("failed to build ChainProvider for %s. Err: %v", chainConfig.ChainID, err)
				continue
			}

			// build the chain
			c := &relayer.Chain{ChainProvider: prov}

			// add to config
			if err = config.AddChain(c); err != nil {
				log.Printf("failed to add chaiin %s to config. Err: %v", chain, err)
				return nil, err
			}

			// found the correct chain so move on to next chain in chains
			break
		}
	}

	return config, err
}
