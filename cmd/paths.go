package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
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
		Use:   "gen [src-chain-id] [dst-chain-id]",
		Short: "generate identifiers for a new path between src and dst",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.c.GetChains(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			path := relayer.Path{
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

			c, err := config.AddPath(path)
			if err != nil {
				return err
			}

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
			index, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}

			if int(index) >= len(config.Paths) {
				return fmt.Errorf("only %d paths in array, index(%d) passed", len(config.Paths), index)
			}

			cfg := config
			cfg.Paths = removePath(config.Paths, int(index))
			return overWriteConfig(cmd, cfg)
		},
	}
	return cmd
}

func removePath(paths []relayer.Path, index int) []relayer.Path {
	return append(paths[:index], paths[index+1:]...)
}

func pathsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "print out configured paths with direction",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range config.Paths.SetIndices() {
				fmt.Println(p.Index)
				out, err := yaml.Marshal(p)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
			}
			return nil
		},
	}
	return cmd
}

func pathsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [index]",
		Short: "show a path at a given index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			if len(config.Paths) > int(index+1) {
				return fmt.Errorf("index %d out of range, %d paths configured", index, len(config.Paths))
			}

			var (
				out []byte
			)

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			if jsn {
				out, err = json.Marshal(config.Paths[index])
				if err != nil {
					return err
				}
			} else {
				out, err = yaml.Marshal(config.Paths[index])
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
		Use:   "add [src-chain-id] [dst-chain-id]",
		Short: "add a path to the list of paths",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.c.GetChains(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			path := relayer.Path{
				Src: &relayer.PathEnd{
					ChainID: src,
				},
				Dst: &relayer.PathEnd{
					ChainID: dst,
				},
			}

			var out *Config
			file, err := cmd.Flags().GetString(flagFile)
			if err != nil {
				return err
			}

			if file != "" {
				if out, err = fileInputPathAdd(file); err != nil {
					return err
				}
			} else {
				if out, err = userInputPathAdd(path, src, dst); err != nil {
					return err
				}
			}

			return overWriteConfig(cmd, out)
		},
	}
	return fileFlag(cmd)
}

func fileInputPathAdd(file string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	p := relayer.Path{}
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

	cfg, err = config.AddPath(p)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func userInputPathAdd(path relayer.Path, src, dst string) (*Config, error) {
	var (
		value string
		err   error
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

	out, err := config.AddPath(path)
	if err != nil {
		return nil, err
	}

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
