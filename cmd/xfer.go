package cmd

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

type stringStringer struct {
	str string
}

func (ss stringStringer) String() string {
	return ss.str
}

func xfersend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [src-chain-id] [dst-chain-id] [amount] [dst-addr]",
		Short: "initiate a transfer from one network to another",
		Long: `Initiate a token transfer via IBC between two networks. The created packet
must be relayed to the destination chain.`,
		Args: cobra.ExactArgs(4),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx transfer ibc-0 ibc-1 100000stake cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk --path demo-path
$ %s tx transfer ibc-0 ibc-1 100000stake cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk --path demo -y 2 -c 10
$ %s tx transfer ibc-0 ibc-1 100000stake raw:non-bech32-address --path demo
$ %s tx raw send ibc-0 ibc-1 100000stake cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk --path demo -c 5
`, appName, appName, appName, appName)),
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

			amount, err := sdk.ParseCoinNormalized(args[2])
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

			toHeightOffset, err := cmd.Flags().GetUint64(flagTimeoutHeightOffset)
			if err != nil {
				return err
			}

			toTimeOffset, err := cmd.Flags().GetDuration(flagTimeoutTimeOffset)
			if err != nil {
				return err
			}

			// If the argument begins with "raw:" then use the suffix directly.
			rawDstAddr := strings.TrimPrefix(args[3], "raw:")
			var dstAddr fmt.Stringer
			if rawDstAddr == args[3] {
				// not "raw:", so we treat the dstAddr as bech32
				done := c[dst].UseSDKContext()

				dstAddr, err = sdk.AccAddressFromBech32(args[3])
				if err != nil {
					return err
				}

				done()
			} else {
				// Don't parse the rest of the dstAddr... it's raw.
				dstAddr = stringStringer{str: rawDstAddr}
			}

			return c[src].SendTransferMsg(c[dst], amount, dstAddr.String(), toHeightOffset, toTimeOffset)
		},
	}

	return timeoutFlags(pathFlag(cmd))
}

func setPathsFromArgs(src, dst *relayer.Chain, name string) (*relayer.Path, error) {
	// find any configured paths between the chains
	paths, err := config.Paths.PathsFromChains(src.ChainID, dst.ChainID)
	if err != nil {
		return nil, err
	}

	// Given the number of args and the number of paths, work on the appropriate
	// path.
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
