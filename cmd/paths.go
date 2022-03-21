package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cosmos/relayer/relayer"
	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	REPOURL  = "https://github.com/cosmos/relayer"
	PATHSURL = "https://github.com/cosmos/relayer/tree/main/interchain"
)

func pathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "paths",
		Aliases: []string{"pth"},
		Short:   "Manage path configurations",
		Long: `
A path represents the "full path" or "link" for communication between two chains. 
This includes the client, connection, and channel ids from both the source and destination chains as well as the strategy to use when relaying`,
	}

	cmd.AddCommand(
		pathsListCmd(),
		pathsShowCmd(),
		pathsAddCmd(),
		pathsNewCmd(),
		pathsFetchCmd(),
		pathsDeleteCmd(),
	)

	return cmd
}

func pathsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [index]",
		Aliases: []string{"d"},
		Short:   "Delete a path with a given index",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths delete demo-path
$ %s pth d path-name`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Paths.Get(args[0]); err != nil {
				return err
			}
			cfg := config
			delete(cfg.Paths, args[0])
			return overWriteConfig(cfg)
		},
	}
	return cmd
}

func pathsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "Print out configured paths",
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
				out, err := yaml.Marshal(config.Paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			case jsn:
				out, err := json.Marshal(config.Paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				i := 0
				for k, pth := range config.Paths {
					chains, err := config.Chains.Gets(pth.Src.ChainID, pth.Dst.ChainID)
					if err != nil {
						return err
					}
					stat := pth.QueryPathStatus(chains[pth.Src.ChainID], chains[pth.Dst.ChainID]).Status

					printPath(cmd.OutOrStdout(), i, k, pth, checkmark(stat.Chains), checkmark(stat.Clients),
						checkmark(stat.Connection))

					i++
				}
				return nil
			}
		},
	}
	return yamlFlag(jsonFlag(cmd))
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

func pathsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show [path-name]",
		Aliases: []string{"s"},
		Short:   "Show a path given its name",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths show demo-path --yaml
$ %s paths show demo-path --json
$ %s pth s path-name`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			chains, err := config.Chains.Gets(p.Src.ChainID, p.Dst.ChainID)
			if err != nil {
				return err
			}
			jsn, _ := cmd.Flags().GetBool(flagJSON)
			yml, _ := cmd.Flags().GetBool(flagYAML)
			pathWithStatus := p.QueryPathStatus(chains[p.Src.ChainID], chains[p.Dst.ChainID])
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
	return yamlFlag(jsonFlag(cmd))
}

func pathsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [src-chain-id] [dst-chain-id] [path-name]",
		Aliases: []string{"a"},
		Short:   "Add a path to the list of paths",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths add ibc-0 ibc-1 demo-path
$ %s paths add ibc-0 ibc-1 demo-path --file paths/demo.json
$ %s pth a ibc-0 ibc-1 demo-path`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			var out *Config
			file, err := cmd.Flags().GetString(flagFile)
			if err != nil {
				return err
			}

			if file != "" {
				if out, err = fileInputPathAdd(cmd.ErrOrStderr(), file, args[2]); err != nil {
					return err
				}
			} else {
				if out, err = userInputPathAdd(cmd.InOrStdin(), cmd.ErrOrStderr(), src, dst, args[2]); err != nil {
					return err
				}
			}

			return overWriteConfig(out)
		},
	}
	return fileFlag(cmd)
}

func pathsNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "new [src-chain-id] [dst-chain-id] [path-name]",
		Aliases: []string{"n"},
		Short:   "Create a new blank path to be used in generating a new path (connection & client) between two chains",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths new ibc-0 ibc-1 demo-path
$ %s pth n ibc-0 ibc-1 demo-path`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			p := &relayer.Path{
				Src: &relayer.PathEnd{ChainID: src},
				Dst: &relayer.PathEnd{ChainID: dst},
			}

			name := args[2]
			if err = config.Paths.Add(name, p); err != nil {
				return err
			}

			return overWriteConfig(config)
		},
	}
	return channelParameterFlags(cmd)
}

// pathsFetchCmd attempts to fetch the json files containing the path metadata, for each configured chain, from GitHub
func pathsFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch",
		Aliases: []string{"fch"},
		Short:   "Fetches the json files necessary to setup the paths for the configured chains",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths fetch --home %s
$ %s pth fch`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Clone the GH repo to tmp dir, we will extract the path files from here
			localRepo, err := os.MkdirTemp("", "")
			if err != nil {
				return err
			}

			if _, err = git.PlainClone(localRepo, false, &git.CloneOptions{
				URL:           REPOURL,
				Progress:      io.Discard,
				ReferenceName: "refs/heads/main",
			}); err != nil {
				return err
			}

			// Try to fetch path info for each configured chain that has canonical chain/path info in the GH repo
			for _, srcChain := range config.Chains {
				for _, dstChain := range config.Chains {

					// Add paths to rly config from {localRepo}/interchain/chaind-id/
					localPathsDir := path.Join(localRepo, "interchain", srcChain.ChainID())

					dir := path.Clean(localPathsDir)
					files, err := ioutil.ReadDir(dir)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "path info does not exist for chain: %s. Consider adding its info to %s. Error: %v\n", srcChain.ChainID(), path.Join(PATHSURL, "interchain"), err)
						break
					}
					cfg := config

					// For each path file, check that the dst is also a configured chain in the relayers config
					for _, f := range files {
						pth := filepath.Join(dir, f.Name())

						if f.IsDir() {
							fmt.Fprintf(cmd.ErrOrStderr(), "directory at %s, skipping...\n", pth)
							continue
						}

						byt, err := os.ReadFile(pth)
						if err != nil {
							cleanupDir(localRepo)
							return fmt.Errorf("failed to read file %s: %w", pth, err)
						}

						p := &relayer.Path{}
						if err = json.Unmarshal(byt, p); err != nil {
							cleanupDir(localRepo)
							return fmt.Errorf("failed to unmarshal file %s: %w", pth, err)
						}

						if p.Dst.ChainID == dstChain.ChainID() {
							pthName := strings.Split(f.Name(), ".")[0]
							if err = cfg.AddPath(pthName, p); err != nil {
								return fmt.Errorf("failed to add path %s: %w", pth, err)
							}

							fmt.Fprintf(cmd.ErrOrStderr(), "added path %s...\n", pthName)
						}
					}

					err = overWriteConfig(cfg)
					if err != nil {
						return err
					}
				}
			}
			cleanupDir(localRepo)
			return nil
		},
	}
	return cmd
}

func cleanupDir(dir string) {
	_ = os.RemoveAll(dir)
}

func fileInputPathAdd(stderr io.Writer, file, name string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	p := &relayer.Path{}
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, &p); err != nil {
		return nil, err
	}

	if err = config.ValidatePath(stderr, p); err != nil {
		return nil, err
	}

	if err = config.Paths.Add(name, p); err != nil {
		return nil, err
	}

	return config, nil
}

func userInputPathAdd(stdin io.Reader, stderr io.Writer, src, dst, name string) (*Config, error) {
	var (
		value string
		err   error
		path  = &relayer.Path{
			Src: &relayer.PathEnd{
				ChainID: src,
			},
			Dst: &relayer.PathEnd{
				ChainID: dst,
			},
		}
	)

	fmt.Fprintf(stderr, "enter src(%s) client-id...\n", src)
	if value, err = readLine(stdin); err != nil {
		return nil, err
	}

	path.Src.ClientID = value

	if err = path.Src.Vclient(); err != nil {
		return nil, err
	}

	fmt.Fprintf(stderr, "enter src(%s) connection-id...\n", src)
	if value, err = readLine(stdin); err != nil {
		return nil, err
	}

	path.Src.ConnectionID = value

	if err = path.Src.Vconn(); err != nil {
		return nil, err
	}

	fmt.Fprintf(stderr, "enter dst(%s) client-id...\n", dst)
	if value, err = readLine(stdin); err != nil {
		return nil, err
	}

	path.Dst.ClientID = value

	if err = path.Dst.Vclient(); err != nil {
		return nil, err
	}

	fmt.Fprintf(stderr, "enter dst(%s) connection-id...\n", dst)
	if value, err = readLine(stdin); err != nil {
		return nil, err
	}

	path.Dst.ConnectionID = value

	if err = path.Dst.Vconn(); err != nil {
		return nil, err
	}

	if err = config.ValidatePath(stderr, path); err != nil {
		return nil, err
	}

	if err = config.Paths.Add(name, path); err != nil {
		return nil, err
	}

	return config, nil
}
