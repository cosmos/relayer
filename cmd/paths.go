package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/google/go-github/v43/github"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func pathsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "paths",
		Aliases: []string{"pth"},
		Short:   "Manage path configurations",
		Long: `
A path represents the "full path" or "link" for communication between two chains. 
This includes the client, connection, and channel ids from both the source and destination chains as well as the strategy to use when relaying`,
	}

	cmd.AddCommand(
		pathsListCmd(a),
		pathsShowCmd(a),
		pathsAddCmd(a),
		pathsAddDirCmd(a),
		pathsNewCmd(a),
		pathsUpdateCmd(a),
		pathsFetchCmd(a),
		pathsDeleteCmd(a),
	)

	return cmd
}

func pathsDeleteCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete index",
		Aliases: []string{"d"},
		Short:   "Delete a path with a given index",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths delete demo-path
$ %s pth d path-name`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.performConfigLockingOperation(cmd.Context(), func() error {
				if _, err := a.config.Paths.Get(args[0]); err != nil {
					return err
				}
				delete(a.config.Paths, args[0])
				return nil
			})
		},
	}
	return cmd
}

func pathsListCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "Print out configured paths",
		Args:    withUsage(cobra.NoArgs),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths list --yaml
$ %s paths list --json
$ %s pth l`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsn, _ := cmd.Flags().GetBool(flagJSON)
			yml, _ := cmd.Flags().GetBool(flagYAML)
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(a.config.Paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(a.config.Paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				i := 0
				for k, pth := range a.config.Paths {
					chains, err := a.config.Chains.Gets(pth.Src.ChainID, pth.Dst.ChainID)
					if err != nil {
						return err
					}
					stat := pth.QueryPathStatus(cmd.Context(), chains[pth.Src.ChainID], chains[pth.Dst.ChainID]).Status

					printPath(cmd.OutOrStdout(), i, k, pth, checkmark(stat.Chains), checkmark(stat.Clients),
						checkmark(stat.Connection))

					i++
				}
				return nil
			}
		},
	}
	return yamlFlag(a.viper, jsonFlag(a.viper, cmd))
}

func printPath(stdout io.Writer, i int, k string, pth *relayer.Path, chains, clients, connection string) {
	fmt.Fprintf(stdout, "%2d: %-20s -> chns(%s) clnts(%s) conn(%s) (%s<>%s)\n",
		i, k, chains, clients, connection, pth.Src.ChainID, pth.Dst.ChainID)
}

func checkmark(status bool) string {
	if status {
		return check
	}
	return xIcon
}

func pathsShowCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show path_name",
		Aliases: []string{"s"},
		Short:   "Show a path given its name",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths show demo-path --yaml
$ %s paths show demo-path --json
$ %s pth s path-name`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			chains, err := a.config.Chains.Gets(p.Src.ChainID, p.Dst.ChainID)
			if err != nil {
				return err
			}
			jsn, _ := cmd.Flags().GetBool(flagJSON)
			yml, _ := cmd.Flags().GetBool(flagYAML)
			pathWithStatus := p.QueryPathStatus(cmd.Context(), chains[p.Src.ChainID], chains[p.Dst.ChainID])
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(pathWithStatus)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(pathWithStatus)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				fmt.Fprintln(cmd.OutOrStdout(), pathWithStatus.PrintString(args[0]))
			}

			return nil
		},
	}
	return yamlFlag(a.viper, jsonFlag(a.viper, cmd))
}

func pathsAddCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add src_chain_id dst_chain_id path_name",
		Aliases: []string{"a"},
		Short:   "Add a path to the list of paths",
		Args:    withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths add ibc-0 ibc-1 demo-path
$ %s paths add ibc-0 ibc-1 demo-path --file paths/demo.json
$ %s pth a ibc-0 ibc-1 demo-path`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			return a.performConfigLockingOperation(cmd.Context(), func() error {
				_, err := a.config.Chains.Gets(src, dst)
				if err != nil {
					return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
				}

				file, err := cmd.Flags().GetString(flagFile)
				if err != nil {
					return err
				}

				if file != "" {
					if err := a.addPathFromFile(cmd.Context(), cmd.ErrOrStderr(), file, args[2]); err != nil {
						return err
					}
				} else {
					if err := a.addPathFromUserInput(cmd.Context(), cmd.InOrStdin(), cmd.ErrOrStderr(), src, dst, args[2]); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	return fileFlag(a.viper, cmd)
}

func pathsAddDirCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-dir dir",
		Args:  withUsage(cobra.ExactArgs(1)),
		Short: `Add path configuration data in bulk from a directory. Example dir: 'configs/demo/paths'`,
		Long: `Add path configuration data in bulk from a directory housing individual path config files. This is useful for spinning up testnets.
		
		See 'examples/demo/configs/paths' for an example of individual path config files.`,
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s config add-paths examples/demo/configs/paths`, appName)),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return addPathsFromDirectory(cmd.Context(), cmd.ErrOrStderr(), a, args[0])
		},
	}

	return cmd
}

func pathsNewCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "new src_chain_id dst_chain_id path_name",
		Aliases: []string{"n"},
		Short:   "Create a new blank path to be used in generating a new path (connection & client) between two chains",
		Args:    withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths new ibc-0 ibc-1 demo-path
$ %s pth n ibc-0 ibc-1 demo-path`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			return a.performConfigLockingOperation(cmd.Context(), func() error {
				_, err := a.config.Chains.Gets(src, dst)
				if err != nil {
					return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
				}

				p := &relayer.Path{
					Src: &relayer.PathEnd{ChainID: src},
					Dst: &relayer.PathEnd{ChainID: dst},
				}

				name := args[2]
				if err = a.config.AddPath(name, p); err != nil {
					return err
				}
				return nil
			})
		},
	}
	return channelParameterFlags(a.viper, cmd)
}

func pathsUpdateCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update path_name",
		Aliases: []string{"n"},
		Short:   `Update a path such as the filter rule ("allowlist", "denylist", or "" for no filtering), filter channels, and src/dst chain, client, or connection IDs`,
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths update demo-path --filter-rule allowlist --filter-channels channel-0,channel-1
$ %s paths update demo-path --filter-rule denylist --filter-channels channel-0,channel-1
$ %s paths update demo-path --src-chain-id chain-1 --dst-chain-id chain-2
$ %s paths update demo-path --src-client-id 07-tendermint-02 --dst-client-id 07-tendermint-04
$ %s paths update demo-path --src-connection-id connection-02 --dst-connection-id connection-04`,
			appName, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			flags := cmd.Flags()

			return a.performConfigLockingOperation(cmd.Context(), func() error {
				p := a.config.Paths.MustGet(name)

				actionTaken := false

				filterRule, _ := flags.GetString(flagFilterRule)
				if filterRule != blankValue {
					if filterRule != "" && filterRule != processor.RuleAllowList && filterRule != processor.RuleDenyList {
						return fmt.Errorf(
							`invalid filter rule : "%s". valid rules: ("", "%s", "%s")`,
							filterRule, processor.RuleAllowList, processor.RuleDenyList)
					}
					p.Filter.Rule = filterRule
					actionTaken = true
				}

				filterChannels, _ := flags.GetString(flagFilterChannels)
				if filterChannels != blankValue {
					var channelList []string

					if filterChannels != "" {
						channelList = strings.Split(filterChannels, ",")
					}

					p.Filter.ChannelList = channelList
					actionTaken = true
				}

				srcChainID, _ := flags.GetString(flagSrcChainID)
				if srcChainID != "" {
					p.Src.ChainID = srcChainID
					actionTaken = true
				}

				dstChainID, _ := flags.GetString(flagDstChainID)
				if dstChainID != "" {
					p.Dst.ChainID = dstChainID
					actionTaken = true
				}

				srcClientID, _ := flags.GetString(flagSrcClientID)
				if srcClientID != "" {
					p.Src.ClientID = srcClientID
					actionTaken = true
				}

				dstClientID, _ := flags.GetString(flagDstClientID)
				if dstClientID != "" {
					p.Dst.ClientID = dstClientID
					actionTaken = true
				}

				srcConnID, _ := flags.GetString(flagSrcConnID)
				if srcConnID != "" {
					p.Src.ConnectionID = srcConnID
					actionTaken = true
				}

				dstConnID, _ := flags.GetString(flagDstConnID)
				if dstConnID != "" {
					p.Dst.ConnectionID = dstConnID
					actionTaken = true
				}

				if !actionTaken {
					return fmt.Errorf("at least one flag must be provided")
				}

				return nil
			})
		},
	}
	cmd = pathFilterFlags(a.viper, cmd)
	return cmd
}

// pathsFetchCmd attempts to fetch the json files containing the path metadata, for each configured chain, from GitHub
func pathsFetchCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch",
		Aliases: []string{"fch"},
		Short:   "Fetches the json files necessary to setup the paths for the configured chains. Passing a chain name will only fetch paths for that chain",
		Args:    withUsage(cobra.RangeArgs(0, 1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths fetch --home %s
$ %s paths fetch --testnet
$ %s pth fch`, appName, defaultHome, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			overwrite, _ := cmd.Flags().GetBool(flagOverwriteConfig)
			testnet, _ := cmd.Flags().GetBool(flagTestnet)

			// allow the relayer to only pull paths for a specific chain
			chainReq := ""
			if len(args) > 0 {
				chainReq = args[0]
				_, exist := a.config.Chains[chainReq]
				if !exist {
					return fmt.Errorf("chain %s not found in config", chainReq)
				}
			}

			return a.performConfigLockingOperation(cmd.Context(), func() error {
				chains := []string{}
				for chainName := range a.config.Chains {
					chains = append(chains, chainName)
				}

				// find all combinations of paths for configured chains
				chainCombinations := make(map[string]bool)
				for _, chainA := range chains {
					for _, chainB := range chains {
						if chainA == chainB {
							continue
						}

						pair := chainA + "-" + chainB
						if chainB < chainA {
							pair = chainB + "-" + chainA
						}

						if chainReq != "" && !strings.Contains(pair, chainReq) {
							continue
						}

						chainCombinations[pair] = true
					}
				}

				client := github.NewClient(nil)
				for pthName := range chainCombinations {
					_, exist := a.config.Paths[pthName]
					if exist && !overwrite {
						fmt.Fprintf(cmd.ErrOrStderr(), "skipping:  %s already exists in config, use -o to overwrite (clears filters)\n", pthName)
						continue
					}

					// TODO: Don't use github api. Potentially use http.get like GetChain() does to avoid rate limits
					fileName := pthName + ".json"
					var regPath string
					if testnet {
						regPath = path.Join("testnets", "_IBC", fileName)
					} else {
						regPath = path.Join("_IBC", fileName)

					}
					client, _, err := client.Repositories.DownloadContents(cmd.Context(), "cosmos", "chain-registry", regPath, nil)
					if err != nil {
						if errors.As(err, new(*github.RateLimitError)) {
							fmt.Println("some paths failed: ", err)
							break
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "failure retrieving: %s: consider adding to cosmos/chain-registry: ERR: %v\n", pthName, err)
						continue
					}
					defer client.Close()

					b, err := io.ReadAll(client)
					if err != nil {
						return fmt.Errorf("error reading response body: %w", err)
					}

					ibc := &relayer.IBCdata{}
					if err = json.Unmarshal(b, &ibc); err != nil {
						return fmt.Errorf("failed to unmarshal: %w ", err)
					}

					srcChainName := ibc.Chain1.ChainName
					dstChainName := ibc.Chain2.ChainName

					srcPathEnd := &relayer.PathEnd{
						ChainID:      a.config.Chains[srcChainName].ChainID(),
						ClientID:     ibc.Chain1.ClientID,
						ConnectionID: ibc.Chain1.ConnectionID,
					}
					dstPathEnd := &relayer.PathEnd{
						ChainID:      a.config.Chains[dstChainName].ChainID(),
						ClientID:     ibc.Chain2.ClientID,
						ConnectionID: ibc.Chain2.ConnectionID,
					}
					newPath := &relayer.Path{
						Src: srcPathEnd,
						Dst: dstPathEnd,
					}
					client.Close()

					if err = a.config.AddPath(pthName, newPath); err != nil {
						return fmt.Errorf("failed to add path %s: %w", pthName, err)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "added:  %s\n", pthName)

				}
				return nil
			})
		},
	}
	OverwriteConfigFlag(a.viper, cmd)
	testnetFlag(a.viper, cmd)
	return cmd
}
