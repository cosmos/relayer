package cmd

import (
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var (
	flagText    = "text"
	flagAddress = "address"
	flagHash    = "hash"
	flagURL     = "url"
	flagForce   = "force"
	flagFlags   = "flags"
	flagTimeout = "timeout"
	flagConfig  = "config"
	flagPrintTx = "print-tx"
)

func liteFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Int64(flags.FlagHeight, -1, "Trusted header's height")
	cmd.Flags().BytesHexP(flagHash, "x", []byte{}, "Trusted header's hash")
	cmd.Flags().StringP(flagURL, "u", "", "Optional URL to fetch trusted-hash and trusted-height")
	viper.BindPFlag(flags.FlagHeight, cmd.Flags().Lookup(flags.FlagHeight))
	viper.BindPFlag(flagHash, cmd.Flags().Lookup(flagHash))
	viper.BindPFlag(flagURL, cmd.Flags().Lookup(flagURL))
	return cmd
}

func paginationFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().IntP(flags.FlagPage, "p", 1, "pagination page of light clients to to query for")
	cmd.Flags().IntP(flags.FlagLimit, "l", 100, "pagination limit of light clients to query for")
	viper.BindPFlag(flags.FlagPage, cmd.Flags().Lookup(flags.FlagPage))
	viper.BindPFlag(flags.FlagLimit, cmd.Flags().Lookup(flags.FlagLimit))
	return cmd
}

func transactionFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagPrintTx, "p", false, "pass flag to print transactions before sending")
	viper.BindPFlag(flagPrintTx, cmd.Flags().Lookup(flagPrintTx))
	return outputFlags(cmd)
}

func outputFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagText, "t", false, "pass flag to force text output")
	cmd.Flags().BoolP(flags.FlagIndentResponse, "i", false, "indent json output")
	viper.BindPFlag(flagText, cmd.Flags().Lookup(flagText))
	viper.BindPFlag(flags.FlagIndentResponse, cmd.Flags().Lookup(flags.FlagIndentResponse))
	return cmd
}

func addressFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP(flagAddress, "a", false, "returns just the address of the flag, useful for scripting")
	viper.BindPFlag(flagAddress, cmd.Flags().Lookup(flagAddress))
	return cmd
}

func timeoutFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP(flagTimeout, "o", "8s", "timeout between relayer runs")
	viper.BindPFlag(flagTimeout, cmd.Flags().Lookup(flagTimeout))
	return cmd
}

func getTimeout(cmd *cobra.Command) (out time.Duration, err error) {
	var to string
	if to, err = cmd.Flags().GetString(flagTimeout); err != nil {
		return
	}
	if out, err = time.ParseDuration(to); err != nil {
		return
	}
	return
}

// PrintOutput fmt.Printlns the json or yaml representation of whatever is passed in
// CONTRACT: The cmd calling this function needs to have the "json" and "indent" flags set
func PrintOutput(toPrint interface{}, cmd *cobra.Command) error {
	var (
		out          []byte
		err          error
		text, indent bool
	)

	text, err = cmd.Flags().GetBool(flagText)
	if err != nil {
		return err
	}
	indent, err = cmd.Flags().GetBool(flags.FlagIndentResponse)
	if err != nil {
		return err
	}

	switch {
	case indent:
		out, err = cdc.MarshalJSONIndent(toPrint, "", "  ")
	case text:
		out, err = yaml.Marshal(&toPrint)
	default:
		out, err = cdc.MarshalJSON(toPrint)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

// PrintTxs prints transactions prior to sending if the flag has been passed in
func PrintTxs(toPrint interface{}, cmd *cobra.Command) error {
	print, err := cmd.Flags().GetBool(flagPrintTx)
	if err != nil {
		return err
	}

	if print {
		err = PrintOutput(toPrint, cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// SendAndPrint sends the transaction with printing options from the CLI
func SendAndPrint(txs []sdk.Msg, chain *relayer.Chain, cmd *cobra.Command) (err error) {
	if err = PrintTxs(txs, cmd); err != nil {
		return err
	}

	res, err := chain.SendMsgs(txs)
	if err != nil {
		return err
	}

	return PrintOutput(res, cmd)
}
