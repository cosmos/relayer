package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// queryCmd represents the chain command
func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query",
		Aliases: []string{"q"},
		Short:   "IBC Query Commands",
		Long:    "Commands to query IBC primitives, and other useful data on configured chains.",
	}

	cmd.AddCommand(
		queryUnrelayedPackets(),
		queryUnrelayedAcknowledgements(),
		flags.LineBreak,
		queryAccountCmd(),
		queryBalanceCmd(),
		queryHeaderCmd(),
		queryNodeStateCmd(),
		queryValSetAtHeightCmd(),
		queryTxs(),
		queryTx(),
		flags.LineBreak,
		queryClientCmd(),
		queryClientsCmd(),
		queryConnection(),
		queryConnections(),
		queryConnectionsUsingClient(),
		queryChannel(),
		queryChannels(),
		queryConnectionChannels(),
		queryPacketCommitment(),
		queryIBCDenoms(),
	)

	return cmd
}

func queryIBCDenoms() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ibc-denoms [chain-id]",
		Short: "Query transaction by transaction hash",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query ibc-denoms ibc-0
$ %s q ibc-denoms ibc-0`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}
			h, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}
			res, err := chain.QueryDenomTraces(0, 1000, h)
			if err != nil {
				return err
			}
			return chain.Print(res, false, false)
		},
	}
	return cmd
}

func queryTx() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [chain-id] [tx-hash]",
		Short: "Query transaction by transaction hash",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query tx ibc-0 [tx-hash]
$ %s q tx ibc-0 A5DF8D272F1C451CFF92BA6C41942C4D29B5CF180279439ED6AB038282F956BE`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			txs, err := chain.QueryTx(args[1])
			if err != nil {
				return err
			}

			out, err := json.Marshal(txs)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func queryTxs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "txs [chain-id] [events]",
		Short: "Query transactions by the events they produce",
		Long: strings.TrimSpace(
			`Search for transactions that match the exact given events where results are paginated. Each event 
takes the form of '{eventType}.{eventAttribute}={value}' with multiple events separated by '&'. 
Please refer to each module's documentation for the full set of events to query for. Each module
documents its respective events under 'cosmos-sdk/x/{module}/spec/xx_events.md'.`),
		Args: cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query txs ibc-0 "message.action=transfer" --offset 1 --limit 10
$ %s q txs ibc-0 "message.action=transfer"`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			events, err := relayer.ParseEvents(args[1])
			if err != nil {
				return err
			}

			h, err := chain.UpdateLightWithHeader()
			if err != nil {
				return err
			}

			offset, err := cmd.Flags().GetUint64(flags.FlagOffset)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flags.FlagLimit)
			if err != nil {
				return err
			}

			txs, err := chain.QueryTxs(relayer.MustGetHeight(h.GetHeight()), int(offset), int(limit), events)
			if err != nil {
				return err
			}

			out, err := json.Marshal(txs)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}
	return paginationFlags(cmd)
}

func queryAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account [chain-id]",
		Aliases: []string{"acc"},
		Short:   "Query the account data",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query account ibc-0
$ %s q acc ibc-1`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			addr, err := chain.GetAddress()
			if err != nil {
				return err
			}

			res, err := types.NewQueryClient(chain.CLIContext(0)).Account(
				context.Background(),
				&types.QueryAccountRequest{
					Address: addr.String(),
				})
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}
	return cmd
}

func queryBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "balance [chain-id] [[key-name]]",
		Aliases: []string{"bal"},
		Short:   "Query the account balances",
		Args:    cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query balance ibc-0
$ %s query balance ibc-0 testkey
$ %s q bal ibc-1 --ibc-denoms`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			showDenoms, err := cmd.Flags().GetBool(flagIBCDenoms)
			if err != nil {
				return err
			}

			var coins sdk.Coins
			if len(args) == 2 {
				coins, err = chain.QueryBalance(args[1])
			} else {
				coins, err = chain.QueryBalance(chain.Key)
			}
			if err != nil {
				return err
			}

			if showDenoms {
				fmt.Println(coins)
				return nil
			}

			h, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			dts, err := chain.QueryDenomTraces(0, 1000, h)
			if err != nil {
				return err
			}

			if len(dts.DenomTraces) > 0 {
				out := sdk.Coins{}
				for _, c := range coins {
					for _, d := range dts.DenomTraces {
						switch {
						case c.Amount.Equal(sdk.NewInt(0)):
						case c.Denom == d.IBCDenom():
							out = append(out, sdk.NewCoin(d.GetFullDenomPath(), c.Amount))
						default:
							out = append(out, c)
						}
					}
				}
				fmt.Println(out)
				return nil
			}

			fmt.Println(coins)
			return nil
		},
	}
	return ibcDenomFlags(cmd)
}

func queryHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "header [chain-id] [[height]]",
		Aliases: []string{"hdr"},
		Short:   "Query the header of a chain at a given height",
		Args:    cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query header ibc-0
$ %s query header ibc-0 1400
$ %s q hdr ibc-1`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			var header *tmclient.Header

			switch len(args) {
			case 1:
				header, err = chain.QueryLatestHeader()
				if err != nil {
					return err
				}
			case 2:
				var height int64
				height, err = strconv.ParseInt(args[1], 10, 64) //convert to int64
				if err != nil {
					return err
				}

				if height == 0 {
					height, err = chain.QueryLatestHeight()
					if err != nil {
						return err
					}

					if height == -1 {
						return relayer.ErrLightNotInitialized
					}
				}

				header, err = chain.QueryHeaderAtHeight(height)
				if err != nil {
					return err
				}

			}

			return chain.Print(header, false, false)
		},
	}

	return cmd
}

// GetCmdQueryConsensusState defines the command to query the consensus state of
// the chain as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#query
func queryNodeStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-state [chain-id]",
		Short: "Query the consensus state of a node",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query node-state ibc-0
$ %s q node-state ibc-1`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			csRes, _, err := chain.QueryConsensusState(0)
			if err != nil {
				return err
			}

			return chain.Print(csRes, false, false)
		},
	}

	return cmd
}

func queryClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client [chain-id] [client-id]",
		Aliases: []string{"clnt"},
		Short:   "Query the state of a client given it's client-id",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client ibc-0 ibczeroclient
$ %s query client ibc-0 ibczeroclient --height 1205
$ %s q clnt ibc-1 ibconeclient`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			if height == 0 {
				height, err = chain.QueryLatestHeight()
				if err != nil {
					return err
				}
			}

			if err = chain.AddPath(args[1], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			res, err := chain.QueryClientState(height)
			if err != nil {
				return err
			}

			return chain.CLIContext(height).PrintProto(res)
		},
	}

	return heightFlag(cmd)
}

func queryClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients [chain-id]",
		Aliases: []string{"clnts"},
		Short:   "Query for all client states on a chain",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query clients ibc-0
$ %s query clients ibc-2 --offset 2 --limit 30
$ %s q clnts ibc-1`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			offset, err := cmd.Flags().GetUint64(flags.FlagOffset)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flags.FlagLimit)
			if err != nil {
				return err
			}

			res, err := chain.QueryClients(offset, limit)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryValSetAtHeightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "valset [chain-id]",
		Aliases: []string{"vs"},
		Short:   "Query validator set at particular height",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query valset ibc-0
$ %s q vs ibc-1`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			h, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			version := clienttypes.ParseChainID(args[0])

			res, err := chain.QueryValsetAtHeight(clienttypes.NewHeight(version, uint64(h)))
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return cmd
}

func queryConnections() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections [chain-id]",
		Aliases: []string{"conns"},
		Short:   "Query for all connections on a chain",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connections ibc-0
$ %s query connections ibc-2 --offset 2 --limit 30
$ %s q conns ibc-1`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			page, err := cmd.Flags().GetUint64(flags.FlagOffset)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flags.FlagLimit)
			if err != nil {
				return err
			}

			res, err := chain.QueryConnections(page, limit)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryConnectionsUsingClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client-connections [chain-id] [client-id]",
		Aliases: []string{"clnt-conns"},
		Short:   "Query for all connections on a given client",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client-connections ibc-0 ibczeroclient
$ %s query client-connections ibc-0 ibczeroclient --height 1205
$ %s q clnt-conns ibc-1 ibconeclient`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(args[1], dcon, dcha, dpor, dord); err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			if height == 0 {
				height, err = chain.QueryLatestHeight()
				if err != nil {
					return err
				}
			}

			res, err := chain.QueryConnectionsUsingClient(height)
			if err != nil {
				return err
			}

			return chain.CLIContext(height).PrintProto(res)
		},
	}

	return heightFlag(cmd)
}

func queryConnection() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [chain-id] [connection-id]",
		Aliases: []string{"conn"},
		Short:   "Query the connection state for the given connection id",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection ibc-0 ibconnection0
$ %s q conn ibc-1 ibconeconn`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err := chain.AddPath(dcli, args[1], dcon, dpor, dord); err != nil {
				return err
			}

			height, err := chain.QueryLatestHeight()
			if err != nil {
				return err
			}

			res, err := chain.QueryConnection(height)
			if err != nil {
				return err
			}

			return chain.CLIContext(height).PrintProto(res)
		},
	}

	return cmd
}

func queryConnectionChannels() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection-channels [chain-id] [connection-id]",
		Aliases: []string{"conn-chans"},
		Short:   "Query any channels associated with a given connection",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection-channels ibc-0 ibcconnection1
$ %s query connection-channels ibc-2 ibcconnection2 --offset 2 --limit 30
$ %s q conn-chans ibc-0 ibcconnection1`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}
			if err = chain.AddPath(dcli, args[1], dcha, dpor, dord); err != nil {
				return err
			}

			page, err := cmd.Flags().GetUint64(flags.FlagOffset)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flags.FlagLimit)
			if err != nil {
				return err
			}

			chans, err := chain.QueryConnectionChannels(args[1], page, limit)
			if err != nil {
				return err
			}

			return chain.Print(chans, false, false)
		},
	}
	return paginationFlags(cmd)
}

func queryChannel() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel [chain-id] [channel-id] [port-id]",
		Aliases: []string{"chan"},
		Short:   "Query a channel given it's channel and port ids",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channel ibc-0 ibczerochannel transfer
$ %s query channel ibc-2 ibctwochannel transfer --height 1205
$ %s q chan ibc-1 ibconechannel transfer`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			if height == 0 {
				height, err = chain.QueryLatestHeight()
				if err != nil {
					return err
				}
			}

			res, err := chain.QueryChannel(height)
			if err != nil {
				return err
			}

			return chain.CLIContext(height).PrintProto(res)
		},
	}

	return heightFlag(cmd)
}

func queryChannels() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channels [chain-id]",
		Aliases: []string{"chans"},
		Short:   "Query for all channels on a chain",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channels ibc-0
$ %s query channels ibc-2 --offset 2 --limit 30
$ %s q chans ibc-1`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			page, err := cmd.Flags().GetUint64(flags.FlagOffset)
			if err != nil {
				return err
			}

			limit, err := cmd.Flags().GetUint64(flags.FlagLimit)
			if err != nil {
				return err
			}

			res, err := chain.QueryChannels(page, limit)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return paginationFlags(cmd)
}

func queryPacketCommitment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packet-commit [chain-id] [channel-id] [port-id] [seq]",
		Short: "Query for the packet commitment given it's sequence and channel ids",
		Args:  cobra.ExactArgs(4),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query packet-commit ibc-0 ibczerochannel transfer 32
$ %s q packet-commit ibc-1 ibconechannel transfer 31`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			if err = chain.AddPath(dcli, dcon, args[1], args[2], dord); err != nil {
				return err
			}

			seq, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return err
			}

			res, err := chain.QueryPacketCommitment(0, seq)
			if err != nil {
				return err
			}

			return chain.Print(res, false, false)
		},
	}

	return cmd
}

func queryUnrelayedPackets() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unrelayed-packets [path]",
		Aliases: []string{"unrelayed", "pkts"},
		Short:   "Query for the packet sequence numbers that remain to be relayed on a given path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query unrelayed-packets demo-path
$ %s query unrelayed demo-path
$ %s q pkts demo-path`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			src, dst := path.Src.ChainID, path.Dst.ChainID

			c, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = c[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = c[dst].SetPath(path.Dst); err != nil {
				return err
			}

			sh, err := relayer.NewSyncHeaders(c[src], c[dst])
			if err != nil {
				return err
			}

			strategy, err := path.GetStrategy()
			if err != nil {
				return err
			}

			sp, err := strategy.UnrelayedSequences(c[src], c[dst], sh)
			if err != nil {
				return err
			}

			out, err := json.Marshal(sp)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}

	return cmd
}

func queryUnrelayedAcknowledgements() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unrelayed-acknowledgements [path]",
		Aliases: []string{"acks"},
		Short:   "Query for the packet sequence numbers that remain to be relayed on a given path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query unrelayed-acknowledgements demo-path
$ %s q acks demo-path`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			src, dst := path.Src.ChainID, path.Dst.ChainID

			c, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if err = c[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = c[dst].SetPath(path.Dst); err != nil {
				return err
			}

			sh, err := relayer.NewSyncHeaders(c[src], c[dst])
			if err != nil {
				return err
			}

			strategy, err := path.GetStrategy()
			if err != nil {
				return err
			}

			sp, err := strategy.UnrelayedAcknowledgements(c[src], c[dst], sh)
			if err != nil {
				return err
			}

			out, err := json.Marshal(sp)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}

	return cmd
}
