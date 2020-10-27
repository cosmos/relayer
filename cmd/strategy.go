package cmd

import (
	"fmt"
	"strconv"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// GetStrategyWithOptions sets strategy specific fields.
func GetStrategyWithOptions(cmd *cobra.Command, strategy relayer.Strategy) (relayer.Strategy, error) {
	switch strategy.GetType() {
	case (&relayer.NaiveStrategy{}).GetType():
		ns, ok := strategy.(*relayer.NaiveStrategy)
		if !ok {
			return strategy,
				fmt.Errorf("strategy.GetType() returns naive, but strategy type (%T) is not type NaiveStrategy", strategy)

		}

		maxTxSize, err := cmd.Flags().GetString(flagMaxTxSize)
		if err != nil {
			return ns, err
		}

		txSize, err := strconv.ParseUint(maxTxSize, 10, 64)
		if err != nil {
			return ns, err
		}

		// set max size of messages in a relay transaction
		ns.MaxTxSize = txSize * MB // in MB

		maxMsgLength, err := cmd.Flags().GetString(flagMaxMsgLength)
		if err != nil {
			return ns, err
		}

		msgLen, err := strconv.ParseUint(maxMsgLength, 10, 64)
		if err != nil {
			return ns, err
		}

		// set max length messages in relay transaction
		ns.MaxMsgLength = msgLen

		return ns, nil
	default:
		return strategy, nil
	}
}
