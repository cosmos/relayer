package cmd

import (
	"strconv"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// GetStrategyWithOptions sets strategy specific fields.
func GetStrategyWithOptions(cmd *cobra.Command, strategy relayer.Strategy) (relayer.Strategy, error) {
	switch start := strategy.(type) {
	case *relayer.NaiveStrategy:
		maxTxSize, err := cmd.Flags().GetString(flagMaxTxSize)
		if err != nil {
			return start, err
		}

		txSize, err := strconv.ParseUint(maxTxSize, 10, 64)
		if err != nil {
			return start, err
		}

		// set max size of messages in a relay transaction
		start.MaxTxSize = txSize * MB // in MB

		maxMsgLength, err := cmd.Flags().GetString(flagMaxMsgLength)
		if err != nil {
			return start, err
		}

		msgLen, err := strconv.ParseUint(maxMsgLength, 10, 64)
		if err != nil {
			return start, err
		}

		// set max length messages in relay transaction
		start.MaxMsgLength = msgLen

		return start, nil
	default:
		return strategy, nil
	}
}
