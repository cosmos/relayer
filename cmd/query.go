package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/helpers"
	"github.com/spf13/cobra"
)

// queryCmd represents the chain command
func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query",
		Aliases: []string{"q"},
		Short:   "IBC query commands",
		Long:    "Commands to query IBC primitives and other useful data on configured chains.",
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
		Short: "query denomination traces for a given network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query ibc-denoms ibc-0
$ %s q ibc-denoms ibc-0`,
			appName, appName,
		)),
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
		Short: "query for a transaction on a given network by transaction hash and chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query tx ibc-0 [tx-hash]
$ %s q tx ibc-0 A5DF8D272F1C451CFF92BA6C41942C4D29B5CF180279439ED6AB038282F956BE`,
			appName, appName,
		)),
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
		Short: "query for transactions on a given network by chain ID and a set of transaction events",
		Long: strings.TrimSpace(`Search for a paginated list of transactions that match the given set of
events. Each event takes the form of '{eventType}.{eventAttribute}={value}' with multiple events
separated by '&'.

Please refer to each module's documentation for the full set of events to query for. Each module
documents its respective events under 'cosmos-sdk/x/{module}/spec/xx_events.md'.`,
		),
		Args: cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query txs ibc-0 "message.action=transfer" --offset 1 --limit 10
$ %s q txs ibc-0 "message.action=transfer"`,
			appName, appName,
		)),
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

			txs, err := helpers.QueryTxs(chain, args[1], offset, limit)
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
		Short:   "query the relayer's account on a given network by chain ID",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query account ibc-0
$ %s q acc ibc-1`,
			appName, appName,
		)),
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
				},
			)
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
		Use:   "balance [chain-id] [[key-name]]",
		Short: "query the relayer's account balance on a given network by chain-ID",
		Args:  cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query balance ibc-0
$ %s query balance ibc-0 testkey`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			showDenoms, err := cmd.Flags().GetBool(flagIBCDenoms)
			if err != nil {
				return err
			}

			keyName := chain.Key
			if len(args) == 2 {
				keyName = args[1]
			}

			if !chain.KeyExists(keyName) {
				return errKeyDoesntExist(keyName)
			}

			info, err := chain.Keybase.Key(keyName)
			if err != nil {
				return err
			}

			coins, err := helpers.QueryBalance(chain, info.GetAddress().String(), showDenoms)
			if err != nil {
				return err
			}

			fmt.Println(coins)
			return nil
		},
	}

	return ibcDenomFlags(cmd)
}

func queryHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "header [chain-id] [[height]]",
		Short: "query the header of a network by chain ID at a given height or the latest height",
		Args:  cobra.RangeArgs(1, 2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query header ibc-0
$ %s query header ibc-0 1400`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			var header *tmclient.Header

			switch len(args) {
			case 1:
				header, err = chain.GetLightSignedHeaderAtHeight(0)
				if err != nil {
					return err
				}

			case 2:
				header, err = helpers.QueryHeader(chain, args[1])
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
		Short: "query the consensus state of a network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query node-state ibc-0
$ %s q node-state ibc-1`,
			appName, appName,
		)),
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
		Use:   "client [chain-id] [client-id]",
		Short: "query the state of a light client on a network by chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client ibc-0 ibczeroclient
$ %s query client ibc-0 ibczeroclient --height 1205`,
			appName, appName,
		)),
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
		Short:   "query for all light client states on a network by chain ID",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query clients ibc-0
$ %s query clients ibc-2 --offset 2 --limit 30`,
			appName, appName,
		)),
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
		Use:   "valset [chain-id]",
		Short: "query the validator set at particular height for a network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query valset ibc-0
$ %s q valset ibc-1`,
			appName, appName,
		)),
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
		Short:   "query for all connections on a network by chain ID",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connections ibc-0
$ %s query connections ibc-2 --offset 2 --limit 30
$ %s q conns ibc-1`,
			appName, appName, appName,
		)),
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
		Use:   "client-connections [chain-id] [client-id]",
		Short: "query for all connections for a given client on a network by chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query client-connections ibc-0 ibczeroclient
$ %s query client-connections ibc-0 ibczeroclient --height 1205`,
			appName, appName,
		)),
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
		Short:   "query the connection state for a given connection id on a network by chain ID",
		Args:    cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection ibc-0 ibconnection0
$ %s q conn ibc-1 ibconeconn`,
			appName, appName,
		)),
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
		Use:   "connection-channels [chain-id] [connection-id]",
		Short: "query all channels associated with a given connection on a network by chain ID",
		Args:  cobra.ExactArgs(2),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query connection-channels ibc-0 ibcconnection1
$ %s query connection-channels ibc-2 ibcconnection2 --offset 2 --limit 30`,
			appName, appName,
		)),
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
		Use:   "channel [chain-id] [channel-id] [port-id]",
		Short: "query a channel by channel and port ID on a network by chain ID",
		Args:  cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channel ibc-0 ibczerochannel transfer
$ %s query channel ibc-2 ibctwochannel transfer --height 1205`,
			appName, appName,
		)),
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
		Use:   "channels [chain-id]",
		Short: "query for all channels on a network by chain ID",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query channels ibc-0
$ %s query channels ibc-2 --offset 2 --limit 30`,
			appName, appName,
		)),
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
		Short: "query for the packet commitment given a sequence and channel ID on a network by chain ID",
		Args:  cobra.ExactArgs(4),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s query packet-commit ibc-0 ibczerochannel transfer 32
$ %s q packet-commit ibc-1 ibconechannel transfer 31`,
			appName, appName,
		)),
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
		Aliases: []string{"unrelayed-pkts"},
		Short:   "query for the packet sequence numbers that remain to be relayed on a given path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s q unrelayed-packets demo-path
$ %s query unrelayed-packets demo-path
$ %s query unrelayed-pkts demo-path`,
			appName, appName, appName,
		)),
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

			strategy, err := path.GetStrategy()
			if err != nil {
				return err
			}

			sp, err := strategy.UnrelayedSequences(c[src], c[dst])
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
		Aliases: []string{"unrelayed-acks"},
		Short:   "query for unrelayed acknowledgement sequence numbers that remain to be relayed on a given path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s q unrelayed-acknowledgements demo-path
$ %s query unrelayed-acknowledgements demo-path
$ %s query unrelayed-acks demo-path`,
			appName, appName, appName,
		)),
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

			strategy, err := path.GetStrategy()
			if err != nil {
				return err
			}

			sp, err := strategy.UnrelayedAcknowledgements(c[src], c[dst])
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
