package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cosmos/relayer/v2/relayer"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	REPOURL  = "https://github.com/cosmos/relayer"
	PATHSURL = "https://github.com/cosmos/relayer/tree/main/interchain"
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
		pathsNewCmd(a),
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
			if _, err := a.Config.Paths.Get(args[0]); err != nil {
				return err
			}
			delete(a.Config.Paths, args[0])
			return a.OverwriteConfig(a.Config)
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
				out, err := yaml.Marshal(a.Config.Paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(a.Config.Paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				i := 0
				for k, pth := range a.Config.Paths {
					chains, err := a.Config.Chains.Gets(pth.Src.ChainID, pth.Dst.ChainID)
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
	return yamlFlag(a.Viper, jsonFlag(a.Viper, cmd))
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
			p, err := a.Config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			chains, err := a.Config.Chains.Gets(p.Src.ChainID, p.Dst.ChainID)
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
	return yamlFlag(a.Viper, jsonFlag(a.Viper, cmd))
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
			_, err := a.Config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			file, err := cmd.Flags().GetString(flagFile)
			if err != nil {
				return err
			}

			if file != "" {
				if err := a.AddPathFromFile(cmd.Context(), cmd.ErrOrStderr(), file, args[2]); err != nil {
					return err
				}
			} else {
				if err := a.AddPathFromUserInput(cmd.Context(), cmd.InOrStdin(), cmd.ErrOrStderr(), src, dst, args[2]); err != nil {
					return err
				}
			}

			return a.OverwriteConfig(a.Config)
		},
	}
	return fileFlag(a.Viper, cmd)
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
			_, err := a.Config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			p := &relayer.Path{
				Src: &relayer.PathEnd{ChainID: src},
				Dst: &relayer.PathEnd{ChainID: dst},
			}

			name := args[2]
			if err = a.Config.Paths.Add(name, p); err != nil {
				return err
			}

			return a.OverwriteConfig(a.Config)
		},
	}
	return channelParameterFlags(a.Viper, cmd)
}

func pathsFetchCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch",
		Aliases: []string{"fch"},
		Short:   "Fetches the json files necessary to setup the paths for the configured chains",
		Args:    withUsage(cobra.NoArgs),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths fetch --home %s
$ %s pth fch`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chains := []string{}
			for chain_name := range a.Config.Chains {
				chains = append(chains, chain_name)
			}

			combinations := permutations(chains)

			client := github.NewClient(nil)
			gh_ctx := context.Background()

			overwrite, _ := cmd.Flags().GetBool(flagOverwriteConfig)

			for _, pthName := range combinations {

				_, exist := a.Config.Paths[pthName]
				if exist && !overwrite {
					fmt.Printf("skipping -- %s already exists in config, use -o to overwrite (clears filters) \n", pthName)
					continue
				}

				// TDOD: download entire chain-registry/_IBC folder and search files that way to scale github client ratelimit

				fileName := pthName + ".json"
				reg_path := filepath.Join("_IBC", fileName)
				r, err := client.Repositories.DownloadContents(gh_ctx, "cosmos", "chain-registry", reg_path, nil)
				if err != nil {
					if _, ok := err.(*github.RateLimitError); ok {
						return fmt.Errorf("hit github rate limit ERR: %w", err)
					}
					fmt.Printf("not found -- path %s not found on chain-registry\n", pthName)
					continue
				}

				bytes, err := ioutil.ReadAll(r)
				if err != nil {
					fmt.Printf("error reading response body: %v", err)
				}

				ibc := &relayer.IBC_data{}
				if err = json.Unmarshal(bytes, &ibc); err != nil {
					fmt.Println("failed to unmarshal ", err)
				}

				src_chainName := ibc.Chain1.ChainName
				dst_chainName := ibc.Chain2.ChainName

				srcPathEnd := &relayer.PathEnd{
					ChainID:      a.Config.Chains[src_chainName].ChainID(),
					ClientID:     ibc.Chain1.ClientID,
					ConnectionID: ibc.Chain1.ConnectionID,
				}
				dstPathEnd := &relayer.PathEnd{
					ChainID:      a.Config.Chains[dst_chainName].ChainID(),
					ClientID:     ibc.Chain2.ClientID,
					ConnectionID: ibc.Chain2.ConnectionID,
				}
				newPath := &relayer.Path{
					Src: srcPathEnd,
					Dst: dstPathEnd,
				}
				r.Close()

				if err = a.Config.AddPath(pthName, newPath); err != nil {
					return fmt.Errorf("failed to add path %s: %w", pthName, err)
				}
				fmt.Printf("added -- %s\n", pthName)

			}

			if err := a.OverwriteConfig(a.Config); err != nil {
				return err
			}
			return nil

		},
	}
	return OverwriteConfigFlag(a.Viper, cmd)
}

func permutations(arr []string) []string {
	var helper func([]string, int)
	res := []string{}

	helper = func(arr []string, n int) {
		if n == 1 {
			tmp := make([]string, 2)
			copy(tmp, arr)
			// make alphabetical
			sort.Strings(tmp)
			// ibc0-ibc1
			tmp1 := string(tmp[0] + "-" + tmp[1])
			if !contains(res, tmp1) {
				res = append(res, tmp1)
			}
		} else {
			for i := 0; i < n; i++ {
				helper(arr, n-1)
				if n%2 == 1 {
					tmp := arr[i]
					arr[i] = arr[n-1]
					arr[n-1] = tmp
				} else {
					tmp := arr[0]
					arr[0] = arr[n-1]
					arr[n-1] = tmp
				}
			}
		}
	}
	helper(arr, len(arr))
	return res
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
