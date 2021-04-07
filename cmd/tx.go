package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// transactionCmd returns a parent transaction command handler, where all child
// commands can submit transactions on IBC-connected networks.
func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transact",
		Aliases: []string{"tx"},
		Short:   "IBC transaction commands",
		Long: strings.TrimSpace(`Commands to create IBC transactions on pre-configured chains.
Most of these commands take a [path] argument. Make sure:
  1. Chains are properly configured to relay over by using the 'rly chains list' command
  2. Path is properly configured to relay over by using the 'rly paths list' command`,
		),
	}

	cmd.AddCommand(
		linkCmd(),
		linkThenStartCmd(),
		relayMsgsCmd(),
		relayAcksCmd(),
		xfersend(),
		flags.LineBreak,
		createClientsCmd(),
		updateClientsCmd(),
		upgradeClientsCmd(),
		upgradeChainCmd(),
		createConnectionCmd(),
		closeChannelCmd(),
		flags.LineBreak,
		rawTransactionCmd(),
		flags.LineBreak,
		sendCmd(),
	)

	return cmd
}

func sendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [chain-id] [from-key] [to-address] [amount]",
		Short: "send funds to a different address on the same chain",
		Args:  cobra.ExactArgs(4),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx send testkey cosmos10yft4nc8tacpngwlpyq3u4t88y7qzc9xv0q4y8 10000uatom`,
			appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			key, err := c.Keybase.Key(args[1])
			if err != nil {
				return err
			}

			to, err := sdk.AccAddressFromBech32(args[2])
			if err != nil {
				return err
			}

			amt, err := sdk.ParseCoinsNormalized(args[3])
			if err != nil {
				return err
			}

			msg := banktypes.NewMsgSend(key.GetAddress(), to, amt)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			res, _, err := c.SendMsg(msg)
			if err != nil {
				return err
			}

			return c.Print(res, false, true)
		},
	}

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [path-name]",
		Short: "create a clients between two configured chains with a configured path",
		Long: "Creates a working ibc client for chain configured on each end of the" +
			" path by querying headers from each chain and then sending the corresponding create-client messages",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s transact clients demo-path`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			modified, err := c[src].CreateClients(c[dst])
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}

			return err
		},
	}

	return cmd
}

func updateClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-clients [path-name]",
		Short: "update IBC clients between two configured chains with a configured path",
		Long: `Updates IBC client for chain configured on each end of the supplied path.
Clients are updated by querying headers from each chain and then sending the
corresponding update-client messages.`,
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s transact update-clients demo-path`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			return c[src].UpdateClients(c[dst])
		},
	}

	return cmd
}

func upgradeClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-clients [path-name] [chain-id]",
		Short: "upgrades IBC clients between two configured chains with a configured path and chain-id",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flags.FlagHeight)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			targetChainID := args[1]

			// send the upgrade message on the targetChainID
			if src == targetChainID {
				return c[src].UpgradeClients(c[dst], height)
			}

			return c[dst].UpgradeClients(c[src], height)
		},
	}

	return heightFlag(cmd)
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection [path-name]",
		Aliases: []string{"conn"},
		Short:   "create a connection between two configured chains with a configured path",
		Long: strings.TrimSpace(`Create or repair a connection between two IBC-connected networks
along a specific path.`,
		),
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact connection demo-path
$ %s tx conn demo-path --timeout 5s`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			retries, err := cmd.Flags().GetUint64(flagMaxRetries)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// ensure that the clients exist
			modified, err := c[src].CreateClients(c[dst])
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}
			if err != nil {
				return err
			}

			modified, err = c[src].CreateOpenConnections(c[dst], retries, to)
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}

			return err
		},
	}

	return retryFlag(timeoutFlag(cmd))
}

func closeChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel-close [path-name]",
		Short: "close a channel between two configured chains with a configured path",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact channel-close demo-path
$ %s tx channel-close demo-path --timeout 5s
$ %s tx channel-close demo-path
$ %s tx channel-close demo-path -o 3s`,
			appName, appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			return c[src].CloseChannel(c[dst], to)
		},
	}

	return timeoutFlag(cmd)
}

func linkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "link [path-name]",
		Aliases: []string{"connect"},
		Short:   "create clients, connection, and channel between two configured chains with a configured path",
		Long: strings.TrimSpace(`Create an IBC client between two IBC-enabled networks, in addition
to creating a connection and a channel between the two networks on a configured path.`,
		),
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact link demo-path
$ %s tx link demo-path
$ %s tx connect demo-path`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			retries, err := cmd.Flags().GetUint64(flagMaxRetries)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// create clients if they aren't already created
			modified, err := c[src].CreateClients(c[dst])
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}

			if err != nil {
				return err
			}

			// create connection if it isn't already created
			modified, err = c[src].CreateOpenConnections(c[dst], retries, to)
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}
			if err != nil {
				return err
			}

			// create channel if it isn't already created
			modified, err = c[src].CreateOpenChannels(c[dst], 3, to)
			if modified {
				if err := overWriteConfig(config); err != nil {
					return err
				}
			}
			return err
		},
	}

	return retryFlag(timeoutFlag(cmd))
}

func linkThenStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "link-then-start [path-name]",
		Aliases: []string{"connect-then-start"},
		Short:   "a shorthand command to execute 'link' followed by 'start'",
		Long: strings.TrimSpace(`Create IBC clients, connection, and channel between two configured IBC
networks with a configured path and then start the relayer on that path.`,
		),
		Args: cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact link-then-start demo-path
$ %s tx link-then-start demo-path --timeout 5s`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			lCmd := linkCmd()

			for err := lCmd.RunE(cmd, args); err != nil; err = lCmd.RunE(cmd, args) {
				fmt.Printf("retrying link: %s\n", err)
				time.Sleep(1 * time.Second)
			}

			sCmd := startCmd()
			return sCmd.RunE(cmd, args)
		},
	}

	return strategyFlag(retryFlag(timeoutFlag(cmd)))
}

func relayMsgsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay-packets [path-name]",
		Aliases: []string{"relay-pkts"},
		Short:   "relay any remaining non-relayed packets on a given path, in both directions",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact relay-packets demo-path
$ %s tx relay-pkts demo-path`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = ensureKeysExist(c); err != nil {
				return err
			}

			strategy, err := GetStrategyWithOptions(cmd, config.Paths.MustGet(args[0]).MustGetStrategy())
			if err != nil {
				return err
			}

			sp, err := strategy.UnrelayedSequences(c[src], c[dst])
			if err != nil {
				return err
			}

			if err = strategy.RelayPackets(c[src], c[dst], sp); err != nil {
				return err
			}

			return nil
		},
	}

	return strategyFlag(cmd)
}

func relayAcksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay-acknowledgements [path-name]",
		Aliases: []string{"relay-acks"},
		Short:   "relay any remaining non-relayed acknowledgements on a given path, in both directions",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact relay-acknowledgements demo-path
$ %s tx relay-acks demo-path -l 3 -s 6`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = ensureKeysExist(c); err != nil {
				return err
			}

			strategy, err := GetStrategyWithOptions(cmd, config.Paths.MustGet(args[0]).MustGetStrategy())
			if err != nil {
				return err
			}

			// sp.Src contains all sequences acked on SRC but acknowledgement not processed on DST
			// sp.Dst contains all sequences acked on DST but acknowledgement not processed on SRC
			sp, err := strategy.UnrelayedAcknowledgements(c[src], c[dst])
			if err != nil {
				return err
			}

			if err = strategy.RelayAcknowledgements(c[src], c[dst], sp); err != nil {
				return err
			}

			return nil
		},
	}

	return strategyFlag(cmd)
}

func upgradeChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-chain [path-name] [chain-id] [new-unbonding-period] [deposit] [path/to/upgradePlan.json]",
		Short: "upgrade an IBC-enabled network with a given upgrade plan",
		Long: strings.TrimSpace(`Upgrade an IBC-enabled network by providing the chain-id of the
network being upgraded, the new unbonding period, the proposal deposit and the JSN file of the
upgrade plan without the upgrade client state.`,
		),
		Args: cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			targetChainID := args[1]

			unbondingPeriod, err := time.ParseDuration(args[2])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = c[src].GetAddress(); err != nil {
				return err
			}
			if _, err = c[dst].GetAddress(); err != nil {
				return err
			}

			// parse deposit
			deposit, err := sdk.ParseCoinNormalized(args[3])
			if err != nil {
				return err
			}

			// parse plan
			plan := &upgradetypes.Plan{}
			path := args[4]
			if _, err := os.Stat(path); err != nil {
				return err
			}

			byt, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			if err = json.Unmarshal(byt, plan); err != nil {
				return err
			}

			// send the upgrade message on the targetChainID
			if src == targetChainID {
				return c[src].UpgradeChain(c[dst], plan, deposit, unbondingPeriod)
			}

			return c[dst].UpgradeChain(c[src], plan, deposit, unbondingPeriod)
		},
	}

	return cmd
}

// ensureKeysExist returns an error if a configured key for a given chain does
// not exist.
func ensureKeysExist(chains map[string]*relayer.Chain) error {
	for _, v := range chains {
		if _, err := v.GetAddress(); err != nil {
			return err
		}
	}

	return nil
}
