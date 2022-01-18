package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	jsonURL = "https://raw.githubusercontent.com/cosmos/relayer/main/interchain/chains/"
	repoURL = "https://github.com/cosmos/relayer"
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
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(config.Paths)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				i := 0
				for k, pth := range config.Paths {
					chains, err := config.Chains.Gets(pth.Src.ChainID, pth.Dst.ChainID)
					if err != nil {
						return err
					}
					stat := pth.QueryPathStatus(chains[pth.Src.ChainID], chains[pth.Dst.ChainID]).Status
					printPath(i, k, pth, checkmark(stat.Chains), checkmark(stat.Clients),
						checkmark(stat.Connection), checkmark(stat.Channel))
					i++
				}
				return nil
			}
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func printPath(i int, k string, pth *relayer.Path, chains, clients, connection, channel string) {
	fmt.Printf("%2d: %-20s -> chns(%s) clnts(%s) conn(%s) chan(%s) (%s:%s<>%s:%s)\n",
		i, k, chains, clients, connection, channel, pth.Src.ChainID, pth.Src.PortID, pth.Dst.ChainID, pth.Dst.PortID)
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
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			chains, err := config.Chains.Gets(path.Src.ChainID, path.Dst.ChainID)
			if err != nil {
				return err
			}
			jsn, _ := cmd.Flags().GetBool(flagJSON)
			yml, _ := cmd.Flags().GetBool(flagYAML)
			pathWithStatus := path.QueryPathStatus(chains[path.Src.ChainID], chains[path.Dst.ChainID])
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(pathWithStatus)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(pathWithStatus)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				fmt.Println(pathWithStatus.PrintString(args[0]))
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
				if out, err = fileInputPathAdd(file, args[2]); err != nil {
					return err
				}
			} else {
				if out, err = userInputPathAdd(src, dst, args[2]); err != nil {
					return err
				}
			}

			return overWriteConfig(out)
		},
	}
	return fileFlag(cmd)
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
			localRepo, err := ioutil.TempDir("", "")
			if err != nil {
				return err
			}

			if _, err = git.PlainClone(localRepo, false, &git.CloneOptions{
				URL:           repoURL,
				Progress:      ioutil.Discard,
				ReferenceName: "refs/heads/main",
			}); err != nil {
				return err
			}

			// Try to fetch path info for each configured chain that has canonical chain/path info in the GH repo
			for _, srcChain := range config.Chains {
				for _, dstChain := range config.Chains {
					fName := fmt.Sprintf("%s.json", srcChain.ChainID())

					// Check that the constructed URL is valid
					u, err := url.Parse(fmt.Sprintf("%s%s", jsonURL, fName))
					if err != nil || u.Scheme == "" || u.Host == "" {
						cleanupDir(localRepo)
						return errors.New("invalid URL")
					}

					// Check that the chain srcChain, has provided canonical chain/path info in GH repo
					resp, err := http.Get(u.String())
					if err != nil || resp.StatusCode == 404 {
						fmt.Printf("Chain %s is not currently supported by fetch. Consider adding it's info to %s \n", srcChain.ChainID(), repoURL)
						continue
					}

					// Add paths to rly config from {localRepo}/interchain/chaind-id/
					pathsDir := path.Join(localRepo, "interchain", srcChain.ChainID())

					dir := path.Clean(pathsDir)
					files, err := ioutil.ReadDir(dir)
					if err != nil {
						return err
					}
					cfg := config

					// For each path file, check that the dst is also a configured chain in the relayers config
					for _, f := range files {
						pth := fmt.Sprintf("%s/%s", dir, f.Name())
						if f.IsDir() {
							fmt.Printf("directory at %s, skipping...\n", pth)
							continue
						}

						byt, err := ioutil.ReadFile(pth)
						if err != nil {
							return fmt.Errorf("failed to read file %s: %w", pth, err)
						}

						p := &relayer.Path{}
						if err = json.Unmarshal(byt, p); err != nil {
							return fmt.Errorf("failed to unmarshal file %s: %w", pth, err)
						}

						if p.Dst.ChainID == dstChain.ChainID() {
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
							if err = cfg.AddPath(pthName, p); err != nil {
								return fmt.Errorf("failed to add path %s: %w", pth, err)
							}

							fmt.Printf("added path %s...\n", pthName)
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

func fileInputPathAdd(file, name string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	p := &relayer.Path{}
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, &p); err != nil {
		return nil, err
	}

	if err = config.ValidatePath(p); err != nil {
		return nil, err
	}

	if err = config.Paths.Add(name, p); err != nil {
		return nil, err
	}

	return config, nil
}

func userInputPathAdd(src, dst, name string) (*Config, error) {
	var (
		value string
		err   error
		path  = &relayer.Path{
			Src: &relayer.PathEnd{
				ChainID: src,
				Order:   "ORDERED",
			},
			Dst: &relayer.PathEnd{
				ChainID: dst,
				Order:   "ORDERED",
			},
		}
	)

	fmt.Printf("enter src(%s) client-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.ClientID = value

	if err = path.Src.Vclient(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) connection-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.ConnectionID = value

	if err = path.Src.Vconn(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) channel-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.ChannelID = value

	if err = path.Src.Vchan(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) port-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.PortID = value

	if err = path.Src.Vport(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) version...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.Version = value

	if err = path.Src.Vversion(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) client-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.ClientID = value

	if err = path.Dst.Vclient(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) connection-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.ConnectionID = value

	if err = path.Dst.Vconn(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) channel-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.ChannelID = value

	if err = path.Dst.Vchan(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) port-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.PortID = value

	if err = path.Dst.Vport(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) version...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.Version = value

	if err = path.Dst.Vversion(); err != nil {
		return nil, err
	}

	if err = config.ValidatePath(path); err != nil {
		return nil, err
	}

	if err = config.Paths.Add(name, path); err != nil {
		return nil, err
	}

	return config, nil
}
