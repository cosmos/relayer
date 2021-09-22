package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cosmos/relayer/relayer"
	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	jsonURL = "https://raw.githubusercontent.com/strangelove-ventures/relayer/main/interchain/chains/"
	repoURL = "https://github.com/strangelove-ventures/relayer"
)

func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch",
		Aliases: []string{"fch"},
		Short:   "Fetch canonical chain and path info for bootstrapping the relayer",
	}

	cmd.AddCommand(
		fetchChainCmd(),
		fetchPathsCmd(),
	)

	return cmd
}

func fetchChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chain [chain-id]",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"chn", "c"},
		Short:   "Fetches the json file necessary to configure the specified chain",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s fetch chain osmosis-1 --home %s
$ %s fch chn cosmoshub-4`, appName, defaultHome, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			fName := fmt.Sprintf("%s.json", chainID)

			// Check that the constructed URL is valid
			u, err := url.Parse(fmt.Sprintf("%s%s", jsonURL, fName))
			if err != nil || u.Scheme == "" || u.Host == "" {
				return errors.New("invalid URL")
			}

			// Fetch the json file via HTTP request
			resp, err := http.Get(u.String())
			if err != nil {
				return fmt.Errorf("Error fetching data for %s be sure it's data is in %s. err: %s \n", chainID, repoURL, err)
			}
			defer resp.Body.Close()

			// Decode the HTTP response and attempt to add chain to the config file
			var c = &relayer.Chain{
				Key:            "",
				ChainID:        "",
				RPCAddr:        "",
				AccountPrefix:  "",
				GasAdjustment:  0,
				GasPrices:      "",
				TrustingPeriod: "",
			}
			d := json.NewDecoder(resp.Body)
			d.DisallowUnknownFields()

			if err = d.Decode(c); err != nil {
				return fmt.Errorf("Error fetching data for %s be sure it's data is in %s. err: %s \n", chainID, repoURL, err)
			}

			if err = config.AddChain(c); err != nil {
				return fmt.Errorf("Error fetching data for %s be sure it's data is in %s. err: %s \n", chainID, repoURL, err)
			}

			err = overWriteConfig(config)
			if err != nil {
				return fmt.Errorf("Be sure you have initialized the relayer config with `rly config init` err: %s \n", err)
			}

			fmt.Printf("Successfully added %s to the relayer configuration. \n", chainID)
			fmt.Printf("Be sure to change default key & rpc-addr values in %s. \n", homePath)

			return nil
		},
	}
	return cmd
}

func fetchPathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "paths",
		Aliases: []string{"pths"},
		Short:   "Fetches the json files necessary to setup the paths for the configured chains",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s fetch paths --home %s
$ %s fch pths`, appName, defaultHome, appName)),
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

			// Try to fetch path info for each configured chain that has canonical chain/path info in GH localRepo
			for _, c := range config.Chains {
				fName := fmt.Sprintf("%s.json", c.ChainID)

				// Check that the constructed URL is valid
				u, err := url.Parse(fmt.Sprintf("%s%s", jsonURL, fName))
				if err != nil || u.Scheme == "" || u.Host == "" {
					cleanupDir(localRepo)
					return errors.New("invalid URL")
				}

				// Check that the chain c, has provided canonical chain/path info in GH localRepo
				resp, err := http.Get(u.String())
				if err != nil || resp.StatusCode == 404 {
					fmt.Printf("Chain %s is not currently supported by fetch. Consider adding it's info to %s \n", c.ChainID, repoURL)
					continue
				}

				// Add paths to rly config from {localRepo}/interchain/chaind-id
				pathsDir := path.Join(localRepo, "interchain", c.ChainID)

				var cfg *Config
				if cfg, err = cfgFilesAddPaths(pathsDir); err != nil {
					fmt.Printf("Failed to add files from %s for chain %s. \n", pathsDir, c.ChainID)
					continue
				}

				err = overWriteConfig(cfg)
				if err != nil {
					return err
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
