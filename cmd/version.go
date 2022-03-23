package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	// Version defines the application version (defined at compile time)
	Version = ""
	// Commit defines the application commit hash (defined at compile time)
	Commit = ""
)

type versionInfo struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	CosmosSDK string `json:"cosmos-sdk" yaml:"cosmos-sdk"`
	Go        string `json:"go" yaml:"go"`
}

func getVersionCmd(a *appState) *cobra.Command {
	versionCmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Print the relayer version info",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s version --json
$ %s v`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			cosmosSDK := "(unable to determine)"
			if bi, ok := debug.ReadBuildInfo(); ok {
				for _, dep := range bi.Deps {
					if dep.Path == "github.com/cosmos/cosmos-sdk" {
						cosmosSDK = dep.Version
						break
					}
				}
			}

			verInfo := versionInfo{
				Version:   Version,
				Commit:    Commit,
				CosmosSDK: cosmosSDK,
				Go:        fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
			}

			var bz []byte
			if jsn {
				bz, err = json.Marshal(verInfo)
			} else {
				bz, err = yaml.Marshal(&verInfo)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(bz))
			return err
		},
	}

	return jsonFlag(a.Viper, versionCmd)
}
