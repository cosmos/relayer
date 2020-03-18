package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func pathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "commands to manage path configurations",
	}

	cmd.AddCommand(
		pathsListCmd(),
		pathsShowCmd(),
		pathsAddCmd(),
		pathsGenCmd(),
		pathsDeleteCmd(),
	)

	return cmd
}

func pathsGenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen [src-chain-id] [dst-chain-id] [name]",
		Short: "generate identifiers for a new path between src and dst",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			path := &relayer.Path{
				Strategy: relayer.NewNaiveStrategy(),
				Src: &relayer.PathEnd{
					ChainID:      src,
					ClientID:     randString(16),
					ConnectionID: randString(16),
					ChannelID:    randString(16),
					PortID:       randString(16),
				},
				Dst: &relayer.PathEnd{
					ChainID:      dst,
					ClientID:     randString(16),
					ConnectionID: randString(16),
					ChannelID:    randString(16),
					PortID:       randString(16),
				},
			}

			pths, err := config.Paths.Add(args[2], path)
			if err != nil {
				return err
			}

			c := config
			c.Paths = pths

			return overWriteConfig(cmd, c)
		},
	}
	return cmd
}

func pathsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [index]",
		Short: "delete a path with a given index",
		Args:  cobra.ExactArgs(1),
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
		Use:   "list",
		Short: "print out configured paths with direction",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				out []byte
			)

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			if jsn {
				out, err = json.Marshal(config.Paths)
				if err != nil {
					return err
				}
			} else {
				out, err = yaml.Marshal(config.Paths)
				if err != nil {
					return err
				}

			}

			fmt.Println(string(out))

			return nil
		},
	}
	return jsonFlag(cmd)
}

func pathsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [path-name]",
		Short: "show a path given its name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}

			var (
				out []byte
			)

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			if jsn {
				out, err = json.Marshal(path)
				if err != nil {
					return err
				}
			} else {
				out, err = yaml.Marshal(path)
				if err != nil {
					return err
				}

			}

			fmt.Println(string(out))
			return nil
		},
	}
	return jsonFlag(cmd)
}

func pathsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [src-chain-id] [dst-chain-id] [path-name]",
		Short: "add a path to the list of paths",
		Args:  cobra.ExactArgs(3),
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

	paths, err := config.Paths.Add(name, p)
	if err != nil {
		return nil, err
	}

	cfg = config
	cfg.Paths = paths

	return cfg, nil
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

	paths, err := config.Paths.Add(name, path)
	if err != nil {
		return nil, err
	}

	out := config
	out.Paths = paths

	return out, nil
}

func randString(length int) string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}
