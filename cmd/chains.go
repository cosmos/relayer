package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/cosmos/relayer/relayer"
)

const (
	check = "✔"
	xIcon = "✘"
)

func chainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chains",
		Aliases: []string{"ch"},
		Short:   "manage chain configurations",
	}

	cmd.AddCommand(
		chainsListCmd(),
		chainsDeleteCmd(),
		chainsAddCmd(),
		chainsEditCmd(),
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
			fmt.Println(address)
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
			yml, err := cmd.Flags().GetBool(flagYAML)
			if err != nil {
				return err
			}
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				pcfgw := &ProviderConfigWrapper{
					Type:  c.ChainProvider.Type(),
					Value: c.ChainProvider.ProviderConfig(),
				}
				out, err := yaml.Marshal(pcfgw)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				pcfgw := &ProviderConfigWrapper{
					Type:  c.ChainProvider.Type(),
					Value: c.ChainProvider.ProviderConfig(),
				}
				out, err := json.Marshal(pcfgw)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				// TODO still need to tweak this for new config
				fmt.Printf(`chain-id:        %s
rpc-addr:        %s
trusting-period: %s
key:             %s
account-prefix:  %s
`, c.ChainID, c.RPCAddr, c.TrustingPeriod, c.Key, c.AccountPrefix)
				return nil
			}
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

// TODO still needs to be tweaked for new config
func chainsEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit [chain-id] [key] [value]",
		Aliases: []string{"e"},
		Short:   "Returns chain configuration data",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains edit ibc-0 trusting-period 32h
$ %s ch e ibc-0 trusting-period 32h`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			c, err := chain.Update(args[1], args[2])
			if err != nil {
				return err
			}

			if err = config.DeleteChain(args[0]).AddChain(c); err != nil {
				return err
			}

			return overWriteConfig(config)
		},
	}
	return cmd
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

			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(ConfigToWrapper(config).ProviderConfigs)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(ConfigToWrapper(config).ProviderConfigs)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
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
						if pth.Src.ChainID == c.ChainID || pth.Dst.ChainID == c.ChainID {
							p = check
						}
					}
					fmt.Printf("%2d: %-20s -> key(%s) bal(%s) path(%s)\n", i, c.ChainID, key, bal, p)
				}
				return nil
			}
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func chainsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"a"},
		Short:   "Add a new chain to the configuration file by passing a file (-f) or url (-u), or user input",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains add
$ %s ch a
$ %s chains add --file chains/ibc0.json
$ %s chains add --url http://relayer.com/ibc0.json
`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			var out *Config

			file, url, err := getAddInputs(cmd)
			if err != nil {
				return err
			}

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
				if out, err = userInputAdd(); err != nil {
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
			if out, err = cfgFilesAddChains(args[0]); err != nil {
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
	c := &relayer.Chain{}
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, &pcw); err != nil {
		return nil, err
	}

	prov, err := pcw.Value.NewProvider(homePath, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to build ChainProvider for %s. Err: %w", file, err)
	}

	c = &relayer.Chain{ChainProvider: prov, ChainID: prov.ChainId()}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}

	return config, nil
}

// TODO this still needs to be changed for new config structure
func userInputAdd() (cfg *Config, err error) {
	c := &relayer.Chain{}

	var value string
	fmt.Println("ChainID (i.e. cosmoshub2):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("chain-id", value); err != nil {
		return nil, err
	}

	fmt.Println("Default Key (i.e. testkey):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("key", value); err != nil {
		return nil, err
	}

	fmt.Println("RPC Address (i.e. http://localhost:26657):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("rpc-addr", value); err != nil {
		return nil, err
	}

	fmt.Println("Account Prefix (i.e. cosmos):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("account-prefix", value); err != nil {
		return nil, err
	}

	fmt.Println("Gas Adjustment (i.e. 1.3):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("gas-adjustment", value); err != nil {
		return nil, err
	}

	fmt.Println("Gas Prices (i.e. 0.025stake):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("gas-prices", value); err != nil {
		return nil, err
	}

	fmt.Println("Trusting Period (i.e. 336h)")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("trusting-period", value); err != nil {
		return nil, err
	}

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
		return
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
		return nil, fmt.Errorf("failed to build ChainProvider for %s. Err: %w", rawurl, err)
	}

	c := &relayer.Chain{ChainProvider: prov, ChainID: prov.ChainId()}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}
	return config, err
}
