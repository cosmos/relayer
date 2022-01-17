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

	"github.com/cosmos/relayer/relayer"
	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
)

const (
	jsonURL = "https://raw.githubusercontent.com/cosmos/relayer/master/interchain/chains/"
	repoURL = "https://github.com/cosmos/relayer"
)

// fetchCmd bootstraps the necessary commands for fetching canonical chain and path metadata from GitHub
func fetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch",
		Aliases: []string{"fch"},
		Short:   "Fetch canonical path info for configured chains",
	}

	cmd.AddCommand(
		fetchPathsCmd(),
	)

	return cmd
}

// fetchPathsCmd attempts to fetch the json files containing the path metadata, for each configured chain, from GitHub
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
				ReferenceName: "refs/heads/master",
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
