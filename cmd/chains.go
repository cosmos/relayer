package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func chainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "commands to configure chains",
	}

	cmd.AddCommand(
		chainsListCmd(),
		chainsDeleteCmd(),
		chainsAddCmd(),
		chainsEditCmd(),
		chainsShowCmd(),
	)

	return cmd
}

func chainsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [chain-id]",
		Short: "Returns a chain's configuration data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				out []byte
				err error
			)

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			if jsn {
				out, err = json.Marshal(config.Chains.Get(args[0]))
				if err != nil {
					return err
				}
			} else {
				out, err = yaml.Marshal(config.Chains.Get(args[0]))
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

func chainsEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [chain-id] [key] [value]",
		Short: "Returns chain configuration data",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0]).Update(args[1], args[2])
			if err != nil {
				return err
			}
			config.DeleteChain(args[0])
			config.AddChain(c)
			return overWriteConfig(cmd, config)
		},
	}
	return cmd
}

func chainsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [chain-id]",
		Short: "Returns chain configuration data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return overWriteConfig(cmd, config.DeleteChain(args[0]))
		},
	}
	return cmd
}

func chainsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Returns chain configuration data",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := yaml.Marshal(config.Chains)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func chainsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Reads in a series of user input and generates a new chain in the config",
		RunE: func(cmd *cobra.Command, args []string) error {

			// TODO: figure out how to parse gaia style configs to pull out necessary data
			// for chain configuration
			//
			// url, err := getURL(cmd)
			// if err != nil {
			// 	return err
			// }

			// if len(url) > 0 {
			// 	cl, err := rpcclient.NewHTTP(url, "/websocket")
			// 	if err != nil {
			// 		return err
			// 	}
			// 	gen, err := cl.Genesis()
			// 	if err != nil {
			// 		return err
			// 	}
			// }

			var out *Config
			file, err := cmd.Flags().GetString(flagFile)
			if err != nil {
				return err
			}

			if file != "" {
				if out, err = fileInputAdd(file); err != nil {
					return err
				}
			} else {
				if out, err = userInputAdd(cmd); err != nil {
					return err
				}
			}

			if err = setChains(out, file); err != nil {
				return err
			}

			return overWriteConfig(cmd, out)
		},
	}

	return fileFlag(cmd)
}

func fileInputAdd(file string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	c := ChainConfig{}
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, &c); err != nil {
		return nil, err
	}

	cfg, err = config.AddChain(c)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func userInputAdd(cmd *cobra.Command) (cfg *Config, err error) {
	c := ChainConfig{}

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

	fmt.Println("Gas (i.e. 200000):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("gas", value); err != nil {
		return nil, err
	}

	fmt.Println("Gas Prices (i.e. 0.025stake):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("gas-prices", value); err != nil {
		return nil, err
	}

	fmt.Println("Default Denom (i.e. stake):")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("default-denom", value); err != nil {
		return nil, err
	}

	fmt.Println("Trusting Period (i.e. 336h)")
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	if c, err = c.Update("trusting-period", value); err != nil {
		return nil, err
	}

	out, err := config.AddChain(c)
	if err != nil {
		return nil, err
	}

	return out, nil
}
