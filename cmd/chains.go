package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/cosmos/relayer/helpers"
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

			addr, err := chain.GetAddress()
			if err != nil {
				return err
			}

			fmt.Println(addr.String())
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
				out, err := yaml.Marshal(c)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(c)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
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
				out, err := yaml.Marshal(config.Chains)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(config.Chains)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				for i, c := range config.Chains {
					var (
						light = xIcon
						key   = xIcon
						path  = xIcon
						bal   = xIcon
					)
					_, err := c.GetAddress()
					if err == nil {
						key = check
					}

					coins, err := c.QueryBalance(c.Key)
					if err == nil && !coins.Empty() {
						bal = check
					}

					_, err = c.GetLatestLightHeader()
					if err == nil {
						light = check
					}

					for _, pth := range config.Paths {
						if pth.Src.ChainID == c.ChainID || pth.Dst.ChainID == c.ChainID {
							path = check
						}
					}
					fmt.Printf("%2d: %-20s -> key(%s) bal(%s) light(%s) path(%s)\n", i, c.ChainID, key, bal, light, path)
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
				if out, err = userInputAdd(cmd); err != nil {
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
			if out, err = filesAdd(args[0]); err != nil {
				return err
			}
			return overWriteConfig(out)
		},
	}

	return cmd
}

func filesAdd(dir string) (cfg *Config, err error) {
	dir = path.Clean(dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cfg = config
	for _, f := range files {
		c := &relayer.Chain{}
		pth := fmt.Sprintf("%s/%s", dir, f.Name())
		if f.IsDir() {
			fmt.Printf("directory at %s, skipping...\n", pth)
			continue
		}
		byt, err := ioutil.ReadFile(pth)
		if err != nil {
			fmt.Printf("failed to read file %s, skipping...\n", pth)
			continue
		}
		if err = json.Unmarshal(byt, c); err != nil {
			fmt.Printf("failed to unmarshal file %s, skipping...\n", pth)
			continue
		}
		if c.ChainID == "" && c.Key == "" && c.RPCAddr == "" {
			p := &relayer.Path{}
			if err = json.Unmarshal(byt, p); err == nil {
				fmt.Printf("%s is a path file, try adding it with 'rly pth add -f %s'...\n", f.Name(), pth)
				continue
			}
			fmt.Printf("%s did not contain valid chain config, skipping...\n", pth)
			continue

		}
		if err = cfg.AddChain(c); err != nil {
			fmt.Printf("%s: %s\n", pth, err.Error())
			continue
		}
		fmt.Printf("added %s...\n", c.ChainID)
	}
	return cfg, nil
}

func fileInputAdd(file string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	c := &relayer.Chain{}
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, c); err != nil {
		return nil, err
	}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}

	return config, nil
}

func userInputAdd(cmd *cobra.Command) (cfg *Config, err error) {
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

	var c *relayer.Chain
	d := json.NewDecoder(resp.Body)
	d.DisallowUnknownFields()
	err = d.Decode(c)
	if err != nil {
		return cfg, err
	}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}
	return config, err
}

// API Handlers

// GetChainsHandler returns the configured chains in json format
func GetChainsHandler(w http.ResponseWriter, r *http.Request) {
	helpers.SuccessJSONResponse(http.StatusOK, config.Chains, w)
}

// GetChainHandler returns the configured chains in json format
func GetChainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, chain, w)
}

// GetChainStatusHandler returns the configured chains in json format
func GetChainStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, chainStatusResponse{}.Populate(chain), w)
}

type chainStatusResponse struct {
	Light   bool `json:"light"`
	Path    bool `json:"path"`
	Key     bool `json:"key"`
	Balance bool `json:"balance"`
}

func (cs chainStatusResponse) Populate(c *relayer.Chain) chainStatusResponse {
	_, err := c.GetAddress()
	if err == nil {
		cs.Key = true
	}

	coins, err := c.QueryBalance(c.Key)
	if err == nil && !coins.Empty() {
		cs.Balance = true
	}

	_, err = c.GetLatestLightHeader()
	if err == nil {
		cs.Light = true
	}

	for _, pth := range config.Paths {
		if pth.Src.ChainID == c.ChainID || pth.Dst.ChainID == c.ChainID {
			cs.Path = true
		}
	}
	return cs
}

type addChainRequest struct {
	Key            string `json:"key"`
	RPCAddr        string `json:"rpc-addr"`
	AccountPrefix  string `json:"account-prefix"`
	GasAdjustment  string `json:"gas-adjustment"`
	GasPrices      string `json:"gas-prices"`
	TrustingPeriod string `json:"trusting-period"`
	// required: false
	FilePath string `json:"file"`
	// required: false
	URL string `json:"url"`
}

// PostChainHandler handles the route
func PostChainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chainID := vars["name"]

	var request addChainRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if request.FilePath != "" && request.URL != "" {
		helpers.WriteErrorResponse(http.StatusBadRequest, errMultipleAddFlags, w)
		return
	}

	var out *Config
	switch {
	case request.FilePath != "":
		if out, err = fileInputAdd(request.FilePath); err != nil {
			helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
			return
		}
	case request.URL != "":
		if out, err = urlInputAdd(request.URL); err != nil {
			helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
			return
		}
	default:
		if out, err = addChainByRequest(request, chainID); err != nil {
			helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
			return
		}
	}

	if err = validateConfig(out); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	if err = overWriteConfig(out); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusCreated, fmt.Sprintf("chain %s added successfully", chainID), w)
}

func addChainByRequest(request addChainRequest, chainID string) (cfg *Config, err error) {
	c := &relayer.Chain{}

	if c, err = c.Update("chain-id", chainID); err != nil {
		return nil, err
	}

	if c, err = c.Update("key", request.Key); err != nil {
		return nil, err
	}

	if c, err = c.Update("rpc-addr", request.RPCAddr); err != nil {
		return nil, err
	}

	if c, err = c.Update("account-prefix", request.AccountPrefix); err != nil {
		return nil, err
	}

	if c, err = c.Update("gas-adjustment", request.GasAdjustment); err != nil {
		return nil, err
	}

	if c, err = c.Update("gas-prices", request.GasPrices); err != nil {
		return nil, err
	}

	if c, err = c.Update("trusting-period", request.TrustingPeriod); err != nil {
		return nil, err
	}

	if err = config.AddChain(c); err != nil {
		return nil, err
	}

	return config, nil
}

type editChainRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PutChainHandler handles the route
func PutChainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	var request editChainRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	c, err := chain.Update(request.Key, request.Value)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = config.DeleteChain(vars["name"]).AddChain(c); err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = overWriteConfig(config); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, fmt.Sprintf("chain %s updated", vars["name"]), w)

}

// DeleteChainHandler handles the route
func DeleteChainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_, err := config.Chains.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	if err := overWriteConfig(config.DeleteChain(vars["name"])); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, fmt.Sprintf("chain %s deleted", vars["name"]), w)
}
