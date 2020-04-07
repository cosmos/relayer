package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func pathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "paths",
		Aliases: []string{"pth"},
		Short:   "commands to manage path configurations",
		Long: `
A path represents the full path for communication between two chains, including the client, 
connection, and channel ids from both the source and destination chains.`,
	}

	cmd.AddCommand(
		pathsListCmd(),
		pathsShowCmd(),
		pathsAddCmd(),
		pathsGenCmd(),
		pathsDeleteCmd(),
		pathsFindCmd(),
	)

	return cmd
}

func pathsFindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "finds any existing paths between any configured chains",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := relayer.FindPaths(config.Chains)
			if err != nil {
				return err
			}
			return config.Chains[0].Print(paths, false, false)
		},
	}
	return cmd
}

func pathsGenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate [src-chain-id] [src-port] [dst-chain-id] [dst-port] [name]",
		Aliases: []string{"gen"},
		Short:   "generate identifiers for a new path between src and dst",
		Args:    cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, srcPort, dst, dstPort := args[0], args[1], args[2], args[3]
			_, err := config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			path := relayer.GenPath(src, dst, srcPort, dstPort)
			if err = config.Paths.Add(args[4], path); err != nil {
				return err
			}

			return overWriteConfig(cmd, config)
		},
	}
	return cmd
}

func pathsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [index]",
		Aliases: []string{"d"},
		Short:   "delete a path with a given index",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Paths.Get(args[0]); err != nil {
				return err
			}
			cfg := config
			delete(cfg.Paths, args[0])
			return overWriteConfig(cmd, cfg)
		},
	}
	return cmd
}

func pathsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "print out configured paths",
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
					// TODO: replace this with relayer.QueryPathStatus
					var (
						chains     = "✘"
						clients    = "✘"
						connection = "✘"
						channel    = "✘"
					)
					src, dst := pth.Src.ChainID, pth.Dst.ChainID
					ch, err := config.Chains.Gets(src, dst)
					if err == nil {
						chains = "✔"
						err = ch[src].SetPath(pth.Src)
						if err != nil {
							printPath(i, k, pth, chains, clients, connection, channel)
							i++
							continue
						}
						err = ch[dst].SetPath(pth.Dst)
						if err != nil {
							printPath(i, k, pth, chains, clients, connection, channel)
							i++
							continue
						}
					} else {
						printPath(i, k, pth, chains, clients, connection, channel)
						i++
						continue
					}

					srcCs, err := ch[src].QueryClientState()
					dstCs, _ := ch[dst].QueryClientState()
					if err == nil && srcCs != nil && dstCs != nil {
						clients = "✔"
					} else {
						printPath(i, k, pth, chains, clients, connection, channel)
						i++
						continue
					}

					srch, err := ch[src].QueryLatestHeight()
					dsth, _ := ch[dst].QueryLatestHeight()
					if err != nil || srch == -1 || dsth == -1 {
						printPath(i, k, pth, chains, clients, connection, channel)
						i++
						continue
					}

					srcConn, err := ch[src].QueryConnection(srch)
					dstConn, _ := ch[dst].QueryConnection(dsth)
					if err == nil && srcConn.Connection.Connection.State.String() == "OPEN" && dstConn.Connection.Connection.State.String() == "OPEN" {
						connection = "✔"
					} else {
						printPath(i, k, pth, chains, clients, connection, channel)
						i++
						continue
					}

					srcChan, err := ch[src].QueryChannel(srch)
					dstChan, _ := ch[dst].QueryChannel(dsth)
					if err == nil && srcChan.Channel.Channel.State.String() == "OPEN" && dstChan.Channel.Channel.State.String() == "OPEN" {
						channel = "✔"
					} else {
						printPath(i, k, pth, chains, clients, connection, channel)
						i++
						continue
					}

					printPath(i, k, pth, chains, clients, connection, channel)
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

func pathsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show [path-name]",
		Aliases: []string{"s"},
		Short:   "show a path given its name",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Paths.Get(args[0])
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
				out, err := yaml.Marshal(path)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(path)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				// TODO: transition this to use relayer.QueryPathStatus
				var (
					chains     = "✘"
					clients    = "✘"
					connection = "✘"
					channel    = "✘"
					srch, dsth int64
				)
				src, dst := path.Src.ChainID, path.Dst.ChainID
				ch, err := config.Chains.Gets(src, dst)
				if err == nil {
					srch, err = ch[src].QueryLatestHeight()
					dsth, _ = ch[dst].QueryLatestHeight()
					if err == nil {
						chains = "✔"
						_ = ch[src].SetPath(path.Src)
						_ = ch[dst].SetPath(path.Dst)
					}
				}

				srcCs, err := ch[src].QueryClientState()
				dstCs, _ := ch[dst].QueryClientState()
				if err == nil && srcCs != nil && dstCs != nil {
					clients = "✔"
				}

				srcConn, err := ch[src].QueryConnection(srch)
				dstConn, _ := ch[dst].QueryConnection(dsth)
				if err == nil && srcConn.Connection.Connection.State.String() == "OPEN" && dstConn.Connection.Connection.State.String() == "OPEN" {
					connection = "✔"
				}

				srcChan, err := ch[src].QueryChannel(srch)
				dstChan, _ := ch[dst].QueryChannel(dsth)
				if err == nil && srcChan.Channel.Channel.State.String() == "OPEN" && dstChan.Channel.Channel.State.String() == "OPEN" {
					channel = "✔"
				}

				fmt.Printf(`Path "%s" strategy(%s):
  SRC(%s)
    ClientID:     %s
    ConnectionID: %s
    ChannelID:    %s
    PortID:       %s
  DST(%s)
    ClientID:     %s
    ConnectionID: %s
    ChannelID:    %s
    PortID:       %s
  STATUS:
    Chains:       %s
    Clients:      %s
    Connection:   %s
    Channel:      %s
`, args[0], path.Strategy.Type, path.Src.ChainID, path.Src.ClientID, path.Src.ConnectionID, path.Src.ChannelID, path.Src.PortID,
					path.Dst.ChainID, path.Dst.ClientID, path.Dst.ConnectionID, path.Dst.ChannelID, path.Dst.PortID, chains, clients, connection, channel)
				return nil
			}

		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func pathsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [src-chain-id] [dst-chain-id] [path-name]",
		Aliases: []string{"a"},
		Short:   "add a path to the list of paths",
		Args:    cobra.ExactArgs(3),
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

			return overWriteConfig(cmd, out)
		},
	}
	return fileFlag(cmd)
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
			Strategy: relayer.NewNaiveStrategy(),
			Src: &relayer.PathEnd{
				ChainID: src,
			},
			Dst: &relayer.PathEnd{
				ChainID: dst,
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

	if err = config.Paths.Add(name, path); err != nil {
		return nil, err
	}

	return config, nil
}
