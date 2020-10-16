package cmd

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/ovrclk/relayer/relayer"
)

// NOTE: These commands are registered over in cmd/raw.go

func xfersend() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transfer [src-chain-id] [dst-chain-id] [amount] [dst-addr]",
		Short:   "Initiate a transfer from one chain to another",
		Aliases: []string{"xfer", "txf", "send"},
		Long: "Sends the first step to transfer tokens in an IBC transfer." +
			" The created packet must be relayed to another chain",
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			c, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			pth, err := cmd.Flags().GetString(flagPath)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(c[src], c[dst], pth); err != nil {
				return err
			}

			amount, err := sdk.ParseCoin(args[2])
			if err != nil {
				return err
			}

			srch, err := c[src].QueryLatestHeight()
			if err != nil {
				return err
			}

			dts, err := c[src].QueryDenomTraces(0, 1000, srch)
			if err != nil {
				return err
			}

			for _, d := range dts.DenomTraces {
				if amount.Denom == d.GetFullDenomPath() {
					amount = sdk.NewCoin(d.IBCDenom(), amount.Amount)
				}
			}

			// TODO: add ability to set timeout height and time from flags
			// Should be relative to current time and block height
			// --timeout-height-offset=1000
			// --timeout-time-offset=2h
			unlock := relayer.SDKConfig.SetLock(c[dst])
			dstAddr, err := sdk.AccAddressFromBech32(args[3])
			if err != nil {
				return err
			}
			unlock()

			return c[src].SendTransferMsg(c[dst], amount, dstAddr)
		},
	}
	return pathFlag(cmd)
}

func setPathsFromArgs(src, dst *relayer.Chain, name string) (*relayer.Path, error) {
	// Find any configured paths between the chains
	paths, err := config.Paths.PathsFromChains(src.ChainID, dst.ChainID)
	if err != nil {
		return nil, err
	}

	// Given the number of args and the number of paths,
	// work on the appropriate path
	var path *relayer.Path
	switch {
	case name != "" && len(paths) > 1:
		if path, err = paths.Get(name); err != nil {
			return path, err
		}
	case name != "" && len(paths) == 1:
		if path, err = paths.Get(name); err != nil {
			return path, err
		}
	case name == "" && len(paths) > 1:
		return nil, fmt.Errorf("more than one path between %s and %s exists, pass in path name", src.ChainID, dst.ChainID)
	case name == "" && len(paths) == 1:
		for _, v := range paths {
			path = v
		}
	}

	if err = src.SetPath(path.End(src.ChainID)); err != nil {
		return nil, err
	}

	if err = dst.SetPath(path.End(dst.ChainID)); err != nil {
		return nil, err
	}

	return path, nil
}
