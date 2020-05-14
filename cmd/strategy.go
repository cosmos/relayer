package cmd

import (
	"fmt"
	"strconv"

	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

// GetStrategyWithOptions sets strategy specific fields.
func GetStrategyWithOptions(cmd *cobra.Command, strategy relayer.Strategy) (relayer.Strategy, error) {
	switch strategy.GetType() {
	case (relayer.NaiveStrategy{}).GetType():
		ns, ok := strategy.(relayer.NaiveStrategy)
		if !ok {
			return strategy, fmt.Errorf("strategy.GetType() returns naive, but strategy type (%T) is not type NaiveStrategy", strategy)

		}

		if maxTxSize, err := cmd.Flags().GetString(flagMaxTxSize); err != nil {
			return ns, err
		} else if maxTxSize != "" {
			txSize, err := strconv.ParseUint(maxTxSize, 10, 64)
			if err != nil {
				return ns, err
			}
			ns.MaxTxSize = txSize * MB
		}

		if maxMsgLength, err := cmd.Flags().GetString(flagMaxMsgLength); err != nil {
			return ns, err
		} else if maxMsgLength != "" {
			msgLen, err := strconv.ParseUint(maxMsgLength, 10, 64)
			if err != nil {
				return ns, err
			}
			ns.MaxMsgLength = msgLen
		}

		return ns, nil
	default:
		return strategy, nil
	}
}
