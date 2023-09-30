package cmd

import (
	"fmt"
	"time"

	"github.com/cosmos/relayer/v2/relayer"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	flagHome                    = "home"
	flagURL                     = "url"
	flagSkip                    = "skip"
	flagTimeout                 = "timeout"
	flagJSON                    = "json"
	flagYAML                    = "yaml"
	flagFile                    = "file"
	flagForceAdd                = "force-add"
	flagPath                    = "path"
	flagTestnet                 = "testnet"
	flagMaxTxSize               = "max-tx-size"
	flagMaxMsgLength            = "max-msgs"
	flagIBCDenoms               = "ibc-denoms"
	flagTimeoutHeightOffset     = "timeout-height-offset"
	flagTimeoutTimeOffset       = "timeout-time-offset"
	flagMaxRetries              = "max-retries"
	flagThresholdTime           = "time-threshold"
	flagUpdateAfterExpiry       = "update-after-expiry"
	flagUpdateAfterMisbehaviour = "update-after-misbehaviour"
	flagClientTrustingPeriod    = "client-tp"
	flagOverride                = "override"
	flagSrcPort                 = "src-port"
	flagDstPort                 = "dst-port"
	flagOrder                   = "order"
	flagVersion                 = "version"
	flagDebugAddr               = "debug-addr"
	flagOverwriteConfig         = "overwrite"
	flagLimit                   = "limit"
	flagHeight                  = "height"
	flagPage                    = "page"
	flagPageKey                 = "page-key"
	flagCountTotal              = "count-total"
	flagReverse                 = "reverse"
	flagProcessor               = "processor"
	flagInitialBlockHistory     = "block-history"
	flagFlushInterval           = "flush-interval"
	flagMemo                    = "memo"
	flagFilterRule              = "filter-rule"
	flagFilterChannels          = "filter-channels"
	flagSrcChainID              = "src-chain-id"
	flagDstChainID              = "dst-chain-id"
	flagSrcClientID             = "src-client-id"
	flagDstClientID             = "dst-client-id"
	flagSrcConnID               = "src-connection-id"
	flagDstConnID               = "dst-connection-id"
	flagOutput                  = "output"
)

const (
	// 7597 is "RLYR" on a telephone keypad.
	// It also happens to be unassigned in the IANA port list.
	defaultDebugAddr = "localhost:7597"

	blankValue = "blank"
)

func ibcDenomFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagIBCDenoms, "i", false, "Display IBC denominations for sending tokens back to other chains")
	if err := v.BindPFlag(flagIBCDenoms, cmd.Flags().Lookup(flagIBCDenoms)); err != nil {
		panic(err)
	}
	return cmd
}

func heightFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Int64(flagHeight, 0, "Height of headers to fetch")
	if err := v.BindPFlag(flagHeight, cmd.Flags().Lookup(flagHeight)); err != nil {
		panic(err)
	}
	return cmd
}

func paginationFlags(v *viper.Viper, cmd *cobra.Command, query string) *cobra.Command {
	cmd.Flags().Uint64(flagPage, 1, fmt.Sprintf("pagination page of %s to query for. This sets offset to a multiple of limit", query))
	cmd.Flags().String(flagPageKey, "", fmt.Sprintf("pagination page-key of %s to query for", query))
	cmd.Flags().Uint64(flagLimit, 100, fmt.Sprintf("pagination limit of %s to query for", query))
	cmd.Flags().Bool(flagCountTotal, false, fmt.Sprintf("count total number of records in %s to query for", query))
	cmd.Flags().Bool(flagReverse, false, "results are sorted in descending order")

	if err := v.BindPFlag(flagPage, cmd.Flags().Lookup(flagPage)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagPageKey, cmd.Flags().Lookup(flagPageKey)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagLimit, cmd.Flags().Lookup(flagLimit)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagCountTotal, cmd.Flags().Lookup(flagCountTotal)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagReverse, cmd.Flags().Lookup(flagReverse)); err != nil {
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
	forceAddFlag(v, cmd)
	testnetFlag(v, cmd)
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
	cmd.Flags().Uint64P(flagTimeoutHeightOffset, "y", 0, "set timeout height offset")
	cmd.Flags().DurationP(flagTimeoutTimeOffset, "c", time.Duration(0), "set timeout time offset")
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

func testnetFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool(flagTestnet, false, "fetches testnet data from the chain registry")
	if err := v.BindPFlag(flagTestnet, cmd.Flags().Lookup(flagTestnet)); err != nil {
		panic(err)
	}
	return cmd
}

func forceAddFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool(flagForceAdd, false, "adds chain data even if there are no working RPC's in the chain registry")
	if err := v.BindPFlag(flagForceAdd, cmd.Flags().Lookup(flagForceAdd)); err != nil {
		panic(err)
	}
	return cmd
}

func pathFilterFlags(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	flags := cmd.Flags()
	flags.String(flagFilterRule, blankValue, `filter rule ("allowlist", "denylist", or "" for no filtering)`)
	if err := v.BindPFlag(flagFilterRule, flags.Lookup(flagFilterRule)); err != nil {
		panic(err)
	}
	flags.String(flagFilterChannels, blankValue, "channels from source chain perspective to filter")
	if err := v.BindPFlag(flagFilterRule, flags.Lookup(flagFilterRule)); err != nil {
		panic(err)
	}
	flags.String(flagSrcChainID, "", "chain ID for source chain")
	if err := v.BindPFlag(flagSrcChainID, flags.Lookup(flagSrcChainID)); err != nil {
		panic(err)
	}
	flags.String(flagDstChainID, "", "chain ID for destination chain")
	if err := v.BindPFlag(flagDstChainID, flags.Lookup(flagDstChainID)); err != nil {
		panic(err)
	}
	flags.String(flagSrcClientID, "", "client ID for source chain")
	if err := v.BindPFlag(flagSrcClientID, flags.Lookup(flagSrcClientID)); err != nil {
		panic(err)
	}
	flags.String(flagDstClientID, "", "client ID for destination chain")
	if err := v.BindPFlag(flagDstClientID, flags.Lookup(flagDstClientID)); err != nil {
		panic(err)
	}
	flags.String(flagSrcConnID, "", "connection ID for source chain")
	if err := v.BindPFlag(flagSrcConnID, flags.Lookup(flagSrcConnID)); err != nil {
		panic(err)
	}
	flags.String(flagDstConnID, "", "connection ID for destination chain")
	if err := v.BindPFlag(flagDstConnID, flags.Lookup(flagDstConnID)); err != nil {
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
	cmd.Flags().Uint64P(
		flagMaxMsgLength,
		"l",
		relayer.DefaultMaxMsgLength,
		"maximum number of messages per transaction",
	)
	if err := v.BindPFlag(flagMaxMsgLength, cmd.Flags().Lookup(flagMaxMsgLength)); err != nil {
		panic(err)
	}
	return cmd
}

func getAddInputs(cmd *cobra.Command) (file string, url string, forceAdd bool, testNet bool, err error) {
	file, err = cmd.Flags().GetString(flagFile)
	if err != nil {
		return
	}

	url, err = cmd.Flags().GetString(flagURL)
	if err != nil {
		return
	}

	forceAdd, err = cmd.Flags().GetBool(flagForceAdd)
	if err != nil {
		return
	}

	testNet, err = cmd.Flags().GetBool(flagTestnet)
	if err != nil {
		return
	}

	if file != "" && url != "" {
		return "", "", false, false, errMultipleAddFlags
	}

	if file != "" && testNet || url != "" && testNet {
		return "", "", false, false, errInvalidTestnetFlag
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
	cmd.Flags().Duration(flagThresholdTime, relayer.DefaultClientUpdateThreshold, "time after previous client update before automatic client update")
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
	cmd.Flags().Duration(flagClientTrustingPeriod, 0, "custom light client trusting period ex. 24h (default: 85% of chains reported unbonding time)")
	if err := v.BindPFlag(flagUpdateAfterExpiry, cmd.Flags().Lookup(flagUpdateAfterExpiry)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagUpdateAfterMisbehaviour, cmd.Flags().Lookup(flagUpdateAfterMisbehaviour)); err != nil {
		panic(err)
	}
	if err := v.BindPFlag(flagClientTrustingPeriod, cmd.Flags().Lookup(flagClientTrustingPeriod)); err != nil {
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
	cmd.Flags().String(flagDebugAddr, "", "address to use for debug and metrics server. By default, will be the api-listen-addr parameter in the global config.")
	if err := v.BindPFlag(flagDebugAddr, cmd.Flags().Lookup(flagDebugAddr)); err != nil {
		panic(err)
	}
	return cmd
}

func processorFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagProcessor, "p", relayer.ProcessorEvents, "which relayer processor to use")
	if err := v.BindPFlag(flagProcessor, cmd.Flags().Lookup(flagProcessor)); err != nil {
		panic(err)
	}
	return cmd
}

func initBlockFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Uint64P(flagInitialBlockHistory, "b", 20, "initial block history to query when using 'events' as the processor for relaying")
	if err := v.BindPFlag(flagInitialBlockHistory, cmd.Flags().Lookup(flagInitialBlockHistory)); err != nil {
		panic(err)
	}
	return cmd
}

func flushIntervalFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().DurationP(flagFlushInterval, "i", relayer.DefaultFlushInterval, "how frequently should a flush routine be run")
	if err := v.BindPFlag(flagFlushInterval, cmd.Flags().Lookup(flagFlushInterval)); err != nil {
		panic(err)
	}
	return cmd
}

func memoFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(flagMemo, "", "a memo to include in relayed packets")
	if err := v.BindPFlag(flagMemo, cmd.Flags().Lookup(flagMemo)); err != nil {
		panic(err)
	}
	return cmd
}

func OverwriteConfigFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagOverwriteConfig, "o", false,
		"overwrite already configured paths - will clear channel filter(s)")
	if err := v.BindPFlag(flagOverwriteConfig, cmd.Flags().Lookup(flagOverwriteConfig)); err != nil {
		panic(err)
	}
	return cmd
}

func addOutputFlag(v *viper.Viper, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagOutput, "o", "legacy", "Specify the console output format. Can be 'legacy' or 'json'.")
	if err := v.BindPFlag(flagOutput, cmd.Flags().Lookup(flagOutput)); err != nil {
		panic(err)
	}
	return cmd
}
