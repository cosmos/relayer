package cmd

import (
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

// NOTE: These commands are registered over in cmd/raw.go

func xfersend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer-send [src-chain-id] [dst-chain-id] [amount] [source] [dst-addr]",
		Short: "xfer-send",
		Long:  "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
		Args:  cobra.ExactArgs(5),
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

			// If there is a path seperator in the denom of the coins being sent,
			// then src is not the source, otherwise it is
			// NOTE: this will not work in the case where tokens are sent from A -> B -> C
			// Need a function in the SDK to determine from a denom if the tokens are from this chain
			// TODO: Refactor this in the SDK.
			source, err := strconv.ParseBool(args[3])
			if err != nil {
				return err
			}

			if source {
				amount.Denom = fmt.Sprintf("%s/%s/%s", c[dst].PathEnd.PortID, c[dst].PathEnd.ChannelID, amount.Denom)
			} else {
				amount.Denom = fmt.Sprintf("%s/%s/%s", c[src].PathEnd.PortID, c[src].PathEnd.ChannelID, amount.Denom)
			}

			dstAddr, err := sdk.AccAddressFromBech32(args[4])
			if err != nil {
				return err
			}

			dstHeader, err := c[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			// MsgTransfer will call SendPacket on src chain
			txs := relayer.RelayMsgs{
				Src: []sdk.Msg{c[src].PathEnd.MsgTransfer(c[dst].PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, source, c[src].MustGetAddress())},
				Dst: []sdk.Msg{},
			}

			if txs.Send(c[src], c[dst]); !txs.Success() {
				return fmt.Errorf("failed to send first transaction")
			}

			return nil
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
		return nil, fmt.Errorf("more than one path between %s and %s exists, pass in path name", src, dst)
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
