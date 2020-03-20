package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"

	"github.com/sirkon/goproxy/gomod"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
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

func getVersionCmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Print relayer version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			modBz, err := ioutil.ReadFile("go.mod")
			if err != nil {
				return err
			}

			mod, err := gomod.Parse("go.mod", modBz)
			if err != nil {
				return err
			}

			jsn, err := cmd.Flags().GetBool(flagJSON)
			if err != nil {
				return err
			}

			verInfo := versionInfo{
				Version:   Version,
				Commit:    Commit,
				CosmosSDK: mod.Require["github.com/cosmos/cosmos-sdk"],
				Go:        fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
			}

			var bz []byte
			if jsn {
				bz, err = json.Marshal(verInfo)
			} else {
				bz, err = yaml.Marshal(&verInfo)
			}

			fmt.Println(string(bz))
			return err
		},
	}
	return jsonFlag(versionCmd)
}
