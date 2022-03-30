package cmd

import (
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	flagURL                     = "url"
	flagSkip                    = "skip"
	flagTimeout                 = "timeout"
	flagJSON                    = "json"
	flagYAML                    = "yaml"
	flagFile                    = "file"
	flagPath                    = "path"
	flagMaxTxSize               = "max-tx-size"
	flagMaxMsgLength            = "max-msgs"
	flagIBCDenoms               = "ibc-denoms"
	flagTimeoutHeightOffset     = "timeout-height-offset"
	flagTimeoutTimeOffset       = "timeout-time-offset"
	flagMaxRetries              = "max-retries"
	flagThresholdTime           = "time-threshold"
	flagUpdateAfterExpiry       = "update-after-expiry"
	flagUpdateAfterMisbehaviour = "update-after-misbehaviour"
	flagOverride                = "override"
	flagSrcPort                 = "src-port"
	flagDstPort                 = "dst-port"
	flagOrder                   = "order"
	flagVersion                 = "version"
	flagDebugAddr               = "debug-addr"
)

const (
	// 7597 is "RLYR" on a telephone keypad.
	// It also happens to be unassigned in the IANA port list.
	defaultDebugAddr = "localhost:7597"
)

func ibcDenomFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagIBCDenoms, "i", false, "Display IBC denominations for sending tokens back to other chains")
	if err := v.BindPFlag(flagIBCDenoms, cmd.Flags().Lookup(flagIBCDenoms)); err != nil {
		panic(err)
	}
	return cmd
}

func heightFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Int64(flags.FlagHeight, 0, "Height of headers to fetch")
	if err := v.BindPFlag(flags.FlagHeight, cmd.Flags().Lookup(flags.FlagHeight)); err != nil {
		panic(err)
	}
	return cmd
}

func paginationFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Uint64P(flags.FlagOffset, "o", 0, "pagination offset for query")
	cmd.Flags().Uint64P(flags.FlagLimit, "l", 10, "pagination limit for query")
	if err := v.BindPFlag(flags.FlagOffset, cmd.Flags().Lookup(flags.FlagOffset)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flags.FlagLimit, cmd.Flags().Lookup(flags.FlagLimit)); err != nil {
		panic(err)
	}
	return cmd
}

func yamlFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagYAML, "y", false, "output using yaml")
	if err := v.BindPFlag(flagYAML, cmd.Flags().Lookup(flagYAML)); err != nil {
		panic(err)
	}
	return cmd
}

func skipConfirm(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagSkip, "y", false, "output using yaml")
	if err := v.BindPFlag(flagSkip, cmd.Flags().Lookup(flagSkip)); err != nil {
		panic(err)
	}
	return cmd
}

func chainsAddFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	fileFlag(v, cmd)
	urlFlag(v, cmd)
	return cmd
}

func pathFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagPath, "p", "", "specify the path to relay over")
	if err := v.BindPFlag(flagPath, cmd.Flags().Lookup(flagPath)); err != nil {
		panic(err)
	}
	return cmd
}

func timeoutFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Uint64P(flagTimeoutHeightOffset, "y", 0, "set timeout height offset for ")
	cmd.Flags().DurationP(flagTimeoutTimeOffset, "c", time.Duration(0), "specify the path to relay over")
	if err := v.BindPFlag(flagTimeoutHeightOffset, cmd.Flags().Lookup(flagTimeoutHeightOffset)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagTimeoutTimeOffset, cmd.Flags().Lookup(flagTimeoutTimeOffset)); err != nil {
		panic(err)
	}
	return cmd
}

func jsonFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagJSON, "j", false, "returns the response in json format")
	if err := v.BindPFlag(flagJSON, cmd.Flags().Lookup(flagJSON)); err != nil {
		panic(err)
	}
	return cmd
}

func fileFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagFile, "f", "", "fetch json data from specified file")
	if err := v.BindPFlag(flagFile, cmd.Flags().Lookup(flagFile)); err != nil {
		panic(err)
	}
	return cmd
}

func timeoutFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagTimeout, "t", "10s", "timeout between relayer runs")
	if err := v.BindPFlag(flagTimeout, cmd.Flags().Lookup(flagTimeout)); err != nil {
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

func urlFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagURL, "u", "", "url to fetch data from")
	if err := v.BindPFlag(flagURL, cmd.Flags().Lookup(flagURL)); err != nil {
		panic(err)
	}
	return cmd
}

func strategyFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagMaxTxSize, "s", "2", "strategy of path to generate of the messages in a relay transaction")
	cmd.Flags().StringP(flagMaxMsgLength, "l", "5", "maximum number of messages in a relay transaction")
	if err := v.BindPFlag(flagMaxTxSize, cmd.Flags().Lookup(flagMaxTxSize)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagMaxMsgLength, cmd.Flags().Lookup(flagMaxMsgLength)); err != nil {
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

func retryFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Uint64P(flagMaxRetries, "r", 3, "maximum retries after failed message send")
	if err := v.BindPFlag(flagMaxRetries, cmd.Flags().Lookup(flagMaxRetries)); err != nil {
		panic(err)
	}
	return cmd
}

func updateTimeFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Duration(flagThresholdTime, 6*time.Hour, "time before to expiry time to update client")
	if err := v.BindPFlag(flagThresholdTime, cmd.Flags().Lookup(flagThresholdTime)); err != nil {
		panic(err)
	}
	return cmd
}

func clientParameterFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagUpdateAfterExpiry, "e", true,
		"allow governance to update the client if expiry occurs")
	cmd.Flags().BoolP(flagUpdateAfterMisbehaviour, "m", true,
		"allow governance to update the client if misbehaviour freezing occurs")
	if err := v.BindPFlag(flagUpdateAfterExpiry, cmd.Flags().Lookup(flagUpdateAfterExpiry)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagUpdateAfterMisbehaviour, cmd.Flags().Lookup(flagUpdateAfterMisbehaviour)); err != nil {
		panic(err)
	}
	return cmd
}

func channelParameterFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	return srcPortFlag(v, dstPortFlag(v, versionFlag(v, orderFlag(v, cmd))))
}

func overrideFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool(flagOverride, false, "option to not reuse existing client or channel")
	if err := v.BindPFlag(flagOverride, cmd.Flags().Lookup(flagOverride)); err != nil {
		panic(err)
	}
	return cmd
}

func orderFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagOrder, "o", "unordered", "order of channel to create (ordered or unordered)")
	if err := v.BindPFlag(flagOrder, cmd.Flags().Lookup(flagOrder)); err != nil {
		panic(err)
	}
	return cmd
}

func versionFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagVersion, "v", "ics20-1", "version of channel to create")
	if err := v.BindPFlag(flagVersion, cmd.Flags().Lookup(flagVersion)); err != nil {
		panic(err)
	}
	return cmd
}

func srcPortFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(flagSrcPort, "transfer", "port on src chain to use when generating path")
	if err := v.BindPFlag(flagSrcPort, cmd.Flags().Lookup(flagSrcPort)); err != nil {
		panic(err)
	}
	return cmd
}

func dstPortFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(flagDstPort, "transfer", "port on dst chain to use when generating path")
	if err := v.BindPFlag(flagDstPort, cmd.Flags().Lookup(flagDstPort)); err != nil {
		panic(err)
	}
	return cmd
}

func debugServerFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(flagDebugAddr, defaultDebugAddr, "address to use for debug server. Set empty to disable debug server.")
	if err := v.BindPFlag(flagDebugAddr, cmd.Flags().Lookup(flagDebugAddr)); err != nil {
		panic(err)
	}
	return cmd
}
