package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/spf13/cobra"
	registry "github.com/strangelove-ventures/lens/client/chain_registry"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const (
	check = "✔"
	xIcon = "✘"
)

func chainsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chains",
		Aliases: []string{"ch"},
		Short:   "Manage chain configurations",
	}

	cmd.AddCommand(
		chainsListCmd(a),
		chainsRegistryList(a),
		chainsDeleteCmd(a),
		chainsAddCmd(a),
		chainsShowCmd(a),
		chainsAddrCmd(a),
		chainsAddDirCmd(a),
	)

	return cmd
}

func chainsAddrCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "address chain_name",
		Aliases: []string{"addr"},
		Short:   "Returns a chain's configured key's address",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains address ibc-0
$ %s ch addr ibc-0`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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

func chainsShowCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show chain_name",
		Aliases: []string{"s"},
		Short:   "Returns a chain's configuration data",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains show ibc-0 --json
$ %s chains show ibc-0 --yaml
$ %s ch s ibc-0 --json
$ %s ch s ibc-0 --yaml`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
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
	return jsonFlag(a.Viper, cmd)
}

func chainsDeleteCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete chain_name",
		Aliases: []string{"d"},
		Short:   "Removes chain from config based off chain-id",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains delete ibc-0
$ %s ch d ibc-0`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, ok := a.Config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}
			a.Config.DeleteChain(args[0])
			return a.OverwriteConfig(a.Config)
		},
	}
	return cmd
}

func chainsRegistryList(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "registry-list",
		Args:    withUsage(cobra.NoArgs),
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

			chains, err := registry.DefaultChainRegistry(a.Log).ListChains(cmd.Context())
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
	return yamlFlag(a.Viper, jsonFlag(a.Viper, cmd))
}

func chainsListCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "Returns chain configuration data",
		Args:    withUsage(cobra.NoArgs),
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

			configs := a.Config.Wrapped().ProviderConfigs
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
				i := 0
				for _, c := range a.Config.Chains {
					var (
						key = xIcon
						p   = xIcon
						bal = xIcon
					)
					// check that the key from config.yaml is set in keychain
					if c.ChainProvider.KeyExists(c.ChainProvider.Key()) {
						key = check
					}

					coins, err := c.ChainProvider.QueryBalance(cmd.Context(), c.ChainProvider.Key())
					if err == nil && !coins.Empty() {
						bal = check
					}

					for _, pth := range a.Config.Paths {
						if pth.Src.ChainID == c.ChainProvider.ChainId() || pth.Dst.ChainID == c.ChainID() {
							p = check
						}
					}
					i++
					fmt.Fprintf(cmd.OutOrStdout(), "%2d: %-20s -> type(%s) key(%s) bal(%s) path(%s)\n", i, c.ChainID(), c.ChainProvider.Type(), key, bal, p)
				}
				return nil
			}
		},
	}
	return yamlFlag(a.Viper, jsonFlag(a.Viper, cmd))
}

func chainsAddCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [chain-name...]",
		Aliases: []string{"a"},
		Short: "Add a new chain to the configuration file by fetching chain metadata from \n" +
			"                the chain-registry or passing a file (-f) or url (-u)",
		Args: withUsage(cobra.MinimumNArgs(0)),
		Example: fmt.Sprintf(` $ %s chains add cosmoshub
 $ %s chains add cosmoshub osmosis
 $ %s chains add --file chains/ibc0.json ibc0
 $ %s chains add --url https://relayer.com/ibc0.json ibc0`, appName, appName, appName, appName),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, url, err := getAddInputs(cmd)
			if err != nil {
				return err
			}

			if ok := a.Config; ok == nil {
				return fmt.Errorf("config not initialized, consider running `rly config init`")
			}

			// default behavior fetch from chain registry
			// still allow for adding config from url or file
			switch {
			case file != "":
				var chainName string
				switch len(args) {
				case 0:
					chainName = strings.Split(filepath.Base(file), ".")[0]
				case 1:
					chainName = args[0]
				default:
					return errors.New("one chain name is required")
				}
				if err := addChainFromFile(a, chainName, file); err != nil {
					return err
				}
			case url != "":
				if len(args) != 1 {
					return errors.New("one chain name is required")
				}
				if err := addChainFromURL(a, args[0], url); err != nil {
					return err
				}
			default:
				if err := addChainsFromRegistry(cmd.Context(), a, args); err != nil {
					return err
				}
			}

			if err := validateConfig(a.Config); err != nil {
				return err
			}

			return a.OverwriteConfig(a.Config)
		},
	}

	return chainsAddFlags(a.Viper, cmd)
}

func chainsAddDirCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add-dir dir",
		Aliases: []string{"ad"},
		Args:    withUsage(cobra.ExactArgs(1)),
		Short:   `Add chain configuration data in bulk from a directory. Example dir: 'configs/demo/chains'`,
		Long: `Add chain configuration data in bulk from a directory housing individual chain config files. This is useful for spinning up testnets.
		
		See 'configs/demo/chains' for an example of individual chain config files.`,
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s chains add-dir configs/demo/chains
$ %s ch ad testnet/chains/`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if err := addChainsFromDirectory(cmd.ErrOrStderr(), a, args[0]); err != nil {
				return err
			}
			return a.OverwriteConfig(a.Config)
		},
	}

	return cmd
}

// addChainFromFile reads a JSON-formatted chain from the named file
// and adds it to a's chains.
func addChainFromFile(a *appState, chainName string, file string) error {
	// If the user passes in a file, attempt to read the chain config from that file
	var pcw ProviderConfigWrapper
	if _, err := os.Stat(file); err != nil {
		return err
	}

	byt, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(byt, &pcw); err != nil {
		return err
	}

	prov, err := pcw.Value.NewProvider(
		a.Log.With(zap.String("provider_type", pcw.Type)),
		a.HomePath, a.Debug, chainName,
	)
	if err != nil {
		return fmt.Errorf("failed to build ChainProvider for %s: %w", file, err)
	}

	c := relayer.NewChain(a.Log, prov, a.Debug)
	if err = a.Config.AddChain(c); err != nil {
		return err
	}

	return nil
}

// addChainFromURL fetches a JSON-encoded chain from the given URL
// and adds it to a's chains.
func addChainFromURL(a *appState, chainName string, rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL %s", rawurl)
	}

	// TODO: add a rly user agent to this outgoing request.
	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var pcw ProviderConfigWrapper
	d := json.NewDecoder(resp.Body)
	d.DisallowUnknownFields()
	err = d.Decode(&pcw)
	if err != nil {
		return err
	}

	// build the ChainProvider before initializing the chain
	prov, err := pcw.Value.NewProvider(
		a.Log.With(zap.String("provider_type", pcw.Type)),
		a.HomePath, a.Debug, chainName,
	)
	if err != nil {
		return fmt.Errorf("failed to build ChainProvider for %s: %w", rawurl, err)
	}

	c := relayer.NewChain(a.Log, prov, a.Debug)
	if err := a.Config.AddChain(c); err != nil {
		return err
	}
	return nil
}

func addChainsFromRegistry(ctx context.Context, a *appState, chains []string) error {
	chainRegistry := registry.DefaultChainRegistry(a.Log)

	var existed, failed, added []string

	for _, chain := range chains {
		if _, ok := a.Config.Chains[chain]; ok {
			a.Log.Warn(
				"Chain already exists",
				zap.String("chain", chain),
				zap.String("source_link", chainRegistry.SourceLink()),
			)
			existed = append(existed, chain)
			continue
		}

		chainInfo, err := chainRegistry.GetChain(ctx, chain)
		if err != nil {
			a.Log.Warn(
				"Error retrieving chain",
				zap.String("chain", chain),
				zap.Error(err),
			)
			failed = append(failed, chain)
			continue
		}

		chainConfig, err := chainInfo.GetChainConfig(ctx)
		if err != nil {
			a.Log.Warn(
				"Error generating chain config",
				zap.String("chain", chain),
				zap.Error(err),
			)
			failed = append(failed, chain)
			continue
		}

		// build the ChainProvider
		pcfg := &cosmos.CosmosProviderConfig{
			Key:            chainConfig.Key,
			ChainName:      chainInfo.ChainName,
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

		prov, err := pcfg.NewProvider(
			a.Log.With(zap.String("provider_type", "cosmos")),
			a.HomePath, a.Debug, chainInfo.ChainName,
		)
		if err != nil {
			a.Log.Warn(
				"Failed to build ChainProvider",
				zap.String("chain_id", chainConfig.ChainID),
				zap.Error(err),
			)
			failed = append(failed, chain)
			continue
		}

		// add to config
		c := relayer.NewChain(a.Log, prov, a.Debug)
		if err = a.Config.AddChain(c); err != nil {
			a.Log.Warn(
				"Failed to add chain to config",
				zap.String("chain", chain),
				zap.Error(err),
			)
			failed = append(failed, chain)
			continue
		}

		added = append(added, chain)
		// found the correct chain so move on to next chain in chains

	}
	a.Log.Info("config update status",
		zap.Any("added", added),
		zap.Any("failed", failed),
		zap.Any("already existed", existed),
	)
	return nil
}
