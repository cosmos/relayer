package cmd

import (
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagHash       = "hash"
	flagURL        = "url"
	flagForce      = "force"
	flagFlags      = "flags"
	flagTimeout    = "timeout"
	flagConfig     = "config"
	flagJSON       = "json"
	flagYAML       = "yaml"
	flagFile       = "file"
	flagPath       = "path"
	flagListenAddr = "listen"
	flagTx         = "no-tx"
	flagBlock      = "no-block"
	flagData       = "data"
	flagOrder      = "unordered"
)

func liteFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Int64(flags.FlagHeight, -1, "Trusted header's height")
	cmd.Flags().BytesHexP(flagHash, "x", []byte{}, "Trusted header's hash")
	cmd.Flags().StringP(flagURL, "u", "", "Optional URL to fetch trusted-hash and trusted-height")
	if err := viper.BindPFlag(flags.FlagHeight, cmd.Flags().Lookup(flags.FlagHeight)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag(flagHash, cmd.Flags().Lookup(flagHash)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag(flagURL, cmd.Flags().Lookup(flagURL)); err != nil {
		panic(err)
	}
	return cmd
}

func paginationFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().IntP(flags.FlagPage, "p", 1, "pagination page of light clients to to query for")
	cmd.Flags().IntP(flags.FlagLimit, "l", 100, "pagination limit of light clients to query for")
	if err := viper.BindPFlag(flags.FlagPage, cmd.Flags().Lookup(flags.FlagPage)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag(flags.FlagLimit, cmd.Flags().Lookup(flags.FlagLimit)); err != nil {
		panic(err)
	}
	return cmd
}

func yamlFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagYAML, "y", false, "output using yaml")
	if err := viper.BindPFlag(flagYAML, cmd.Flags().Lookup(flagYAML)); err != nil {
		panic(err)
	}
	return cmd
}

func orderFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagOrder, "o", false, "create an unordered channel")
	if err := viper.BindPFlag(flagOrder, cmd.Flags().Lookup(flagOrder)); err != nil {
		panic(err)
	}
	return cmd
}

func listenFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagTx, "t", false, "don't output transaction events")
	cmd.Flags().BoolP(flagBlock, "b", false, "don't output block events")
	cmd.Flags().Bool(flagData, false, "output full event data")
	if err := viper.BindPFlag(flagTx, cmd.Flags().Lookup(flagTx)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag(flagBlock, cmd.Flags().Lookup(flagBlock)); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag(flagData, cmd.Flags().Lookup(flagData)); err != nil {
		panic(err)
	}
	return cmd
}

func chainsAddFlags(cmd *cobra.Command) *cobra.Command {
	fileFlag(cmd)
	urlFlag(cmd)
	return cmd
}

func listenFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagListenAddr, "l", "0.0.0.0:8000", "sets the faucet listener addresss")
	if err := viper.BindPFlag(flagListenAddr, cmd.Flags().Lookup(flagListenAddr)); err != nil {
		panic(err)
	}
	return cmd
}

func pathFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagPath, "p", "", "specify the path to relay over")
	if err := viper.BindPFlag(flagPath, cmd.Flags().Lookup(flagPath)); err != nil {
		panic(err)
	}
	return cmd
}

func jsonFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagJSON, "j", false, "returns the response in json format")
	if err := viper.BindPFlag(flagJSON, cmd.Flags().Lookup(flagJSON)); err != nil {
		panic(err)
	}
	return cmd
}

func fileFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagFile, "f", "", "fetch json data from specified file")
	if err := viper.BindPFlag(flagFile, cmd.Flags().Lookup(flagFile)); err != nil {
		panic(err)
	}
	return cmd
}

func timeoutFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagTimeout, "o", "10s", "timeout between relayer runs")
	if err := viper.BindPFlag(flagTimeout, cmd.Flags().Lookup(flagTimeout)); err != nil {
		panic(err)
	}
	return cmd
}

func forceFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagForce, "f", false, "option to force initialization of lite client from configured chain")
	if err := viper.BindPFlag(flagForce, cmd.Flags().Lookup(flagForce)); err != nil {
		panic(err)
	}
	return cmd
}

func flagsFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagFlags, "f", false, "pass flag to output the flags for lite init/update")
	if err := viper.BindPFlag(flagFlags, cmd.Flags().Lookup(flagFlags)); err != nil {
		panic(err)
	}
	return cmd
}

func getTimeout(cmd *cobra.Command) (time.Duration, error) {
	to, err := cmd.Flags().GetString(flagTimeout)
	if err != nil {
		return 0, err
	}
	return time.ParseDuration(to)
}

func urlFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagURL, "u", "", "url to fetch data from")
	if err := viper.BindPFlag(flagURL, cmd.Flags().Lookup(flagURL)); err != nil {
		panic(err)
	}
	return cmd
}

func getAddInputs(cmd *cobra.Command) (file string, url string, err error) {
	file, err = cmd.Flags().GetString(flagFile)
	if err != nil {
		return
	}

	url, err = cmd.Flags().GetString(flagURL)
	if err != nil {
		return
	}

	if file != "" && url != "" {
		return "", "", errMultipleAddFlags
	}

	return
}
