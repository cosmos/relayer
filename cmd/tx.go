package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const flushTimeout = 10 * time.Minute

// transactionCmd returns a parent transaction command handler, where all child
// commands can submit transactions on IBC-connected networks.
func transactionCmd(a *appState) *cobra.Command {
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
		linkCmd(a),
		linkThenStartCmd(a),
		flushCmd(a),
		relayMsgsCmd(a),
		relayAcksCmd(a),
		xfersend(a),
		lineBreakCommand(),
		createClientsCmd(a),
		createClientCmd(a),
		updateClientsCmd(a),
		upgradeClientsCmd(a),
		createConnectionCmd(a),
		createChannelCmd(a),
		closeChannelCmd(a),
		lineBreakCommand(),
		registerCounterpartyCmd(a),
	)

	return cmd
}

func createClientsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients path_name",
		Short: "create a clients between two configured chains with a configured path",
		Long: "Creates a working ibc client for chain configured on each end of the" +
			" path by querying headers from each chain and then sending the corresponding create-client messages",
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s transact clients demo-path`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			allowUpdateAfterExpiry, err := cmd.Flags().GetBool(flagUpdateAfterExpiry)
			if err != nil {
				return err
			}

			allowUpdateAfterMisbehaviour, err := cmd.Flags().GetBool(flagUpdateAfterMisbehaviour)
			if err != nil {
				return err
			}

			customClientTrustingPeriod, err := cmd.Flags().GetDuration(flagClientTrustingPeriod)
			if err != nil {
				return err
			}

			override, err := cmd.Flags().GetBool(flagOverride)
			if err != nil {
				return err
			}

			path := args[0]

			c, src, dst, err := a.config.ChainsFromPath(path)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			clientSrc, clientDst, err := c[src].CreateClients(cmd.Context(), c[dst], allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override, customClientTrustingPeriod, a.config.memo(cmd))
			if err != nil {
				return err
			}
			if clientSrc != "" || clientDst != "" {
				if err := a.updatePathConfig(cmd.Context(), path, clientSrc, clientDst, "", ""); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd = clientParameterFlags(a.viper, cmd)
	cmd = overrideFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func createClientCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client src_chain_name dst_chain_name path_name",
		Short: "create a client between two configured chains with a configured path",
		Long: "Creates a working ibc client for chain configured on each end of the" +
			" path by querying headers from each chain and then sending the corresponding create-client messages",
		Args:    withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s transact client demo-path`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			allowUpdateAfterExpiry, err := cmd.Flags().GetBool(flagUpdateAfterExpiry)
			if err != nil {
				return err
			}

			allowUpdateAfterMisbehaviour, err := cmd.Flags().GetBool(flagUpdateAfterMisbehaviour)
			if err != nil {
				return err
			}

			customClientTrustingPeriod, err := cmd.Flags().GetDuration(flagClientTrustingPeriod)
			if err != nil {
				return err
			}

			override, err := cmd.Flags().GetBool(flagOverride)
			if err != nil {
				return err
			}

			src, ok := a.config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}
			dst, ok := a.config.Chains[args[1]]
			if !ok {
				return errChainNotFound(args[1])
			}

			pathName := args[2]
			path, err := a.config.Paths.Get(pathName)
			if err != nil {
				return err
			}

			src.PathEnd = path.End(src.ChainID())
			dst.PathEnd = path.End(dst.ChainID())

			// ensure that keys exist
			if exists := src.ChainProvider.KeyExists(src.ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", src.ChainProvider.Key(), src.ChainID())
			}
			if exists := dst.ChainProvider.KeyExists(dst.ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", dst.ChainProvider.Key(), dst.ChainID())
			}

			// Query the latest heights on src and dst and retry if the query fails
			var srch, dsth int64
			if err = retry.Do(func() error {
				srch, dsth, err = relayer.QueryLatestHeights(cmd.Context(), src, dst)
				if srch == 0 || dsth == 0 || err != nil {
					return fmt.Errorf("failed to query latest heights: %w", err)
				}
				return err
			}, retry.Context(cmd.Context()), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr); err != nil {
				return err
			}

			// Query the light signed headers for src & dst at the heights srch & dsth, retry if the query fails
			var srcUpdateHeader, dstUpdateHeader provider.IBCHeader
			if err = retry.Do(func() error {
				srcUpdateHeader, dstUpdateHeader, err = relayer.QueryIBCHeaders(cmd.Context(), src, dst, srch, dsth)
				if err != nil {
					return err
				}
				return nil
			}, retry.Context(cmd.Context()), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
				a.log.Info(
					"Failed to get light signed header",
					zap.String("src_chain_id", src.ChainID()),
					zap.Int64("src_height", srch),
					zap.String("dst_chain_id", dst.ChainID()),
					zap.Int64("dst_height", dsth),
					zap.Uint("attempt", n+1),
					zap.Uint("max_attempts", relayer.RtyAttNum),
					zap.Error(err),
				)
				srch, dsth, _ = relayer.QueryLatestHeights(cmd.Context(), src, dst)
			})); err != nil {
				return err
			}

			clientID, err := relayer.CreateClient(cmd.Context(), src, dst, srcUpdateHeader, dstUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override, customClientTrustingPeriod, a.config.memo(cmd))
			if err != nil {
				return err
			}
			var clientSrc, clientDst string
			if path.Src.ChainID == src.ChainID() {
				clientSrc = clientID
			} else {
				clientDst = clientID
			}
			if clientID != "" {
				if err = a.updatePathConfig(cmd.Context(), pathName, clientSrc, clientDst, "", ""); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd = clientParameterFlags(a.viper, cmd)
	cmd = overrideFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func updateClientsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-clients path_name",
		Short: "update IBC clients between two configured chains with a configured path",
		Long: `Updates IBC client for chain configured on each end of the supplied path.
Clients are updated by querying headers from each chain and then sending the
corresponding update-client messages.`,
		Args:    withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`$ %s transact update-clients demo-path`, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := a.config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			// ensure that keys exist
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			return relayer.UpdateClients(cmd.Context(), c[src], c[dst], a.config.memo(cmd))
		},
	}

	return memoFlag(a.viper, cmd)
}

func upgradeClientsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-clients path_name chain_id",
		Short: "upgrades IBC clients between two configured chains with a configured path and chain-id",
		Args:  withUsage(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := a.config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			height, err := cmd.Flags().GetInt64(flagHeight)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			targetChainID := args[1]

			memo := a.config.memo(cmd)

			// send the upgrade message on the targetChainID
			if src == targetChainID {
				return relayer.UpgradeClient(cmd.Context(), c[dst], c[src], height, memo)
			}

			return relayer.UpgradeClient(cmd.Context(), c[src], c[dst], height, memo)
		},
	}

	cmd = heightFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func createConnectionCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connection path_name",
		Aliases: []string{"conn"},
		Short:   "create a connection between two configured chains with a configured path",
		Long: strings.TrimSpace(`Create or repair a connection between two IBC-connected networks
along a specific path.`,
		),
		Args: withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact connection demo-path
$ %s tx conn demo-path --timeout 5s`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			allowUpdateAfterExpiry, err := cmd.Flags().GetBool(flagUpdateAfterExpiry)
			if err != nil {
				return err
			}

			allowUpdateAfterMisbehaviour, err := cmd.Flags().GetBool(flagUpdateAfterMisbehaviour)
			if err != nil {
				return err
			}

			customClientTrustingPeriod, err := cmd.Flags().GetDuration(flagClientTrustingPeriod)
			if err != nil {
				return err
			}

			pathName := args[0]

			c, src, dst, err := a.config.ChainsFromPath(pathName)
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

			override, err := cmd.Flags().GetBool(flagOverride)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			memo := a.config.memo(cmd)

			initialBlockHistory, err := cmd.Flags().GetUint64(flagInitialBlockHistory)
			if err != nil {
				return err
			}

			// ensure that the clients exist
			clientSrc, clientDst, err := c[src].CreateClients(cmd.Context(), c[dst], allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override, customClientTrustingPeriod, memo)
			if err != nil {
				return err
			}
			if clientSrc != "" || clientDst != "" {
				if err := a.updatePathConfig(cmd.Context(), pathName, clientSrc, clientDst, "", ""); err != nil {
					return err
				}
			}

			connectionSrc, connectionDst, err := c[src].CreateOpenConnections(cmd.Context(), c[dst], retries, to, memo, initialBlockHistory, pathName)
			if err != nil {
				return err
			}
			if connectionSrc != "" || connectionDst != "" {
				if err := a.updatePathConfig(cmd.Context(), pathName, "", "", connectionSrc, connectionDst); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd = timeoutFlag(a.viper, cmd)
	cmd = retryFlag(a.viper, cmd)
	cmd = clientParameterFlags(a.viper, cmd)
	cmd = overrideFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	cmd = initBlockFlag(a.viper, cmd)
	return cmd
}

func createChannelCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "channel path_name",
		Aliases: []string{"chan"},
		Short:   "create a channel between two configured chains with a configured path using specified or default channel identifiers",
		Long: strings.TrimSpace(`Create or repair a channel between two IBC-connected networks
along a specific path.`,
		),
		Args: withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact channel demo-path --src-port transfer --dst-port transfer --order unordered --version ics20-1
$ %s tx chan demo-path --timeout 5s --max-retries 10`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {

			pathName := args[0]

			c, src, dst, err := a.config.ChainsFromPath(pathName)
			if err != nil {
				return err
			}

			override, err := cmd.Flags().GetBool(flagOverride)
			if err != nil {
				return err
			}

			srcPort, err := cmd.Flags().GetString(flagSrcPort)
			if err != nil {
				return err
			}

			dstPort, err := cmd.Flags().GetString(flagDstPort)
			if err != nil {
				return err
			}

			order, err := cmd.Flags().GetString(flagOrder)
			if err != nil {
				return err
			}

			version, err := cmd.Flags().GetString(flagVersion)
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
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			// create channel if it isn't already created
			return c[src].CreateOpenChannels(cmd.Context(), c[dst], retries, to, srcPort, dstPort, order, version, override, a.config.memo(cmd), pathName)
		},
	}

	cmd = timeoutFlag(a.viper, cmd)
	cmd = retryFlag(a.viper, cmd)
	cmd = overrideFlag(a.viper, cmd)
	cmd = channelParameterFlags(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func closeChannelCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel-close path_name src_channel_id src_port_id",
		Short: "close a channel between two configured chains with a configured path",
		Args:  withUsage(cobra.ExactArgs(3)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact channel-close demo-path channel-0 transfer
$ %s tx channel-close demo-path channel-0 transfer --timeout 5s
$ %s tx channel-close demo-path channel-0 transfer
$ %s tx channel-close demo-path channel-0 transfer -o 3s`,
			appName, appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			pathName := args[0]

			c, src, dst, err := a.config.ChainsFromPath(pathName)
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

			channelID := args[1]
			portID := args[2]

			// ensure that keys exist
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			srch, err := c[src].ChainProvider.QueryLatestHeight(cmd.Context())
			if err != nil {
				return err
			}

			_, err = c[src].ChainProvider.QueryChannel(cmd.Context(), srch, channelID, portID)
			if err != nil {
				return err
			}

			return c[src].CloseChannel(cmd.Context(), c[dst], retries, to, channelID, portID, a.config.memo(cmd), pathName)
		},
	}

	cmd = timeoutFlag(a.viper, cmd)
	cmd = retryFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func linkCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "link path_name",
		Aliases: []string{"connect"},
		Short:   "create clients, connection, and channel between two configured chains with a configured path",
		Long: strings.TrimSpace(`Create an IBC client between two IBC-enabled networks, in addition
to creating a connection and a channel between the two networks on a configured path.`,
		),
		Args: withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact link demo-path --src-port transfer --dst-port transfer
$ %s tx link demo-path
$ %s tx connect demo-path --src-port transfer --dst-port transfer --order unordered --version ics20-1`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			allowUpdateAfterExpiry, err := cmd.Flags().GetBool(flagUpdateAfterExpiry)
			if err != nil {
				return err
			}

			allowUpdateAfterMisbehaviour, err := cmd.Flags().GetBool(flagUpdateAfterMisbehaviour)
			if err != nil {
				return err
			}

			customClientTrustingPeriod, err := cmd.Flags().GetDuration(flagClientTrustingPeriod)
			if err != nil {
				return err
			}

			pathName := args[0]

			pth, err := a.config.Paths.Get(pathName)
			if err != nil {
				return err
			}

			src, dst := pth.Src.ChainID, pth.Dst.ChainID
			c, err := a.config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			c[src].PathEnd = pth.Src
			c[dst].PathEnd = pth.Dst

			srcPort, err := cmd.Flags().GetString(flagSrcPort)
			if err != nil {
				return err
			}

			dstPort, err := cmd.Flags().GetString(flagDstPort)
			if err != nil {
				return err
			}

			order, err := cmd.Flags().GetString(flagOrder)
			if err != nil {
				return err
			}

			version, err := cmd.Flags().GetString(flagVersion)
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

			override, err := cmd.Flags().GetBool(flagOverride)
			if err != nil {
				return err
			}

			// ensure that keys exist
			if exists := c[src].ChainProvider.KeyExists(c[src].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on src chain %s", c[src].ChainProvider.Key(), c[src].ChainID())
			}
			if exists := c[dst].ChainProvider.KeyExists(c[dst].ChainProvider.Key()); !exists {
				return fmt.Errorf("key %s not found on dst chain %s", c[dst].ChainProvider.Key(), c[dst].ChainID())
			}

			memo := a.config.memo(cmd)

			initialBlockHistory, err := cmd.Flags().GetUint64(flagInitialBlockHistory)
			if err != nil {
				return err
			}

			// create clients if they aren't already created
			clientSrc, clientDst, err := c[src].CreateClients(cmd.Context(), c[dst], allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override, customClientTrustingPeriod, memo)
			if err != nil {
				return fmt.Errorf("error creating clients: %w", err)
			}
			if clientSrc != "" || clientDst != "" {
				if err := a.updatePathConfig(cmd.Context(), pathName, clientSrc, clientDst, "", ""); err != nil {
					return err
				}
			}

			// create connection if it isn't already created
			connectionSrc, connectionDst, err := c[src].CreateOpenConnections(cmd.Context(), c[dst], retries, to, memo, initialBlockHistory, pathName)
			if err != nil {
				return fmt.Errorf("error creating connections: %w", err)
			}
			if connectionSrc != "" || connectionDst != "" {
				if err := a.updatePathConfig(cmd.Context(), pathName, "", "", connectionSrc, connectionDst); err != nil {
					return err
				}
			}

			// create channel if it isn't already created
			return c[src].CreateOpenChannels(cmd.Context(), c[dst], retries, to, srcPort, dstPort, order, version, override, memo, pathName)
		},
	}
	cmd = timeoutFlag(a.viper, cmd)
	cmd = retryFlag(a.viper, cmd)
	cmd = clientParameterFlags(a.viper, cmd)
	cmd = channelParameterFlags(a.viper, cmd)
	cmd = overrideFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	cmd = initBlockFlag(a.viper, cmd)
	return cmd
}

func linkThenStartCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "link-then-start path_name",
		Aliases: []string{"connect-then-start"},
		Short:   "a shorthand command to execute 'link' followed by 'start'",
		Long: strings.TrimSpace(`Create IBC clients, connection, and channel between two configured IBC
networks with a configured path and then start the relayer on that path.`,
		),
		Args: withUsage(cobra.ExactArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact link-then-start demo-path
$ %s tx link-then-start demo-path --timeout 5s`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			lCmd := linkCmd(a)

			for err := lCmd.RunE(cmd, args); err != nil; err = lCmd.RunE(cmd, args) {
				a.log.Info("Error running link; retrying", zap.Error(err))
				select {
				case <-time.After(time.Second):
					// Keep going.
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				}
			}

			sCmd := startCmd(a)
			return sCmd.RunE(cmd, args)
		},
	}

	cmd = timeoutFlag(a.viper, cmd)
	cmd = retryFlag(a.viper, cmd)
	cmd = strategyFlag(a.viper, cmd)
	cmd = clientParameterFlags(a.viper, cmd)
	cmd = channelParameterFlags(a.viper, cmd)
	cmd = overrideFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	cmd = debugServerFlags(a.viper, cmd)
	cmd = initBlockFlag(a.viper, cmd)
	cmd = processorFlag(a.viper, cmd)
	cmd = updateTimeFlags(a.viper, cmd)
	cmd = flushIntervalFlag(a.viper, cmd)
	return cmd
}

func flushCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "flush [path_name]? [src_channel_id]?",
		Aliases: []string{"relay-pkts"},
		Short:   "flush any pending MsgRecvPacket and MsgAcknowledgement messages on a given path, in both directions",
		Args:    withUsage(cobra.RangeArgs(0, 2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx flush
$ %s tx flush demo-path
$ %s tx flush demo-path channel-0`,
			appName, appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chains := make(map[string]*relayer.Chain)
			var paths []relayer.NamedPath

			if len(args) > 0 {
				pathName := args[0]
				path := a.config.Paths.MustGet(pathName)
				paths = append(paths, relayer.NamedPath{
					Name: pathName,
					Path: path,
				})

				// collect unique chain IDs
				chains[path.Src.ChainID] = nil
				chains[path.Dst.ChainID] = nil
			} else {
				for n, path := range a.config.Paths {
					paths = append(paths, relayer.NamedPath{
						Name: n,
						Path: path,
					})

					// collect unique chain IDs
					chains[path.Src.ChainID] = nil
					chains[path.Dst.ChainID] = nil
				}
			}

			chainIDs := make([]string, 0, len(chains))
			for chainID := range chains {
				chainIDs = append(chainIDs, chainID)
			}

			// get chain configurations
			chains, err := a.config.Chains.Gets(chainIDs...)
			if err != nil {
				return err
			}

			if err := ensureKeysExist(chains); err != nil {
				return err
			}

			maxMsgLength, err := cmd.Flags().GetUint64(flagMaxMsgLength)
			if err != nil {
				return err
			}

			if len(args) == 2 {
				// Only allow specific channel
				paths[0].Path.Filter = relayer.ChannelFilter{
					Rule:        processor.RuleAllowList,
					ChannelList: []string{args[1]},
				}
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), flushTimeout)
			defer cancel()

			rlyErrCh := relayer.StartRelayer(
				ctx,
				a.log,
				chains,
				paths,
				maxMsgLength,
				a.config.memo(cmd),
				0,
				0,
				&processor.FlushLifecycle{},
				relayer.ProcessorEvents,
				0,
				nil,
			)

			// Block until the error channel sends a message.
			// The context being canceled will cause the relayer to stop,
			// so we don't want to separately monitor the ctx.Done channel,
			// because we would risk returning before the relayer cleans up.
			if err := <-rlyErrCh; err != nil && !errors.Is(err, context.Canceled) {
				a.log.Warn(
					"Relayer start error",
					zap.Error(err),
				)
				return err
			}
			return nil
		},
	}

	cmd = strategyFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func relayMsgsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay-packets path_name src_channel_id",
		Aliases: []string{"relay-pkts"},
		Short:   "relay any remaining non-relayed packets on a given path, in both directions",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact relay-packets demo-path channel-0
$ %s tx relay-pkts demo-path channel-0`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.log.Warn("This command is deprecated. Please use 'tx flush' command instead")
			return flushCmd(a).RunE(cmd, args)
		},
	}

	cmd = strategyFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func relayAcksCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relay-acknowledgements path_name src_channel_id",
		Aliases: []string{"relay-acks"},
		Short:   "relay any remaining non-relayed acknowledgements on a given path, in both directions",
		Args:    withUsage(cobra.ExactArgs(2)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s transact relay-acknowledgements demo-path channel-0
$ %s tx relay-acks demo-path channel-0 -l 3 -s 6`,
			appName, appName,
		)),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.log.Warn("This command is deprecated. Please use 'tx flush' command instead")
			return flushCmd(a).RunE(cmd, args)
		},
	}

	cmd = strategyFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	return cmd
}

func xfersend(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer src_chain_name dst_chain_name amount dst_addr src_channel_id",
		Short: "initiate a transfer from one network to another",
		Long: `Initiate a token transfer via IBC between two networks. The created packet
must be relayed to the destination chain.`,
		Args: withUsage(cobra.ExactArgs(5)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s tx transfer ibc-0 ibc-1 100000stake cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk channel-0 --path demo-path
$ %s tx transfer ibc-0 ibc-1 100000stake cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk channel-0 --path demo -y 2 -c 10
$ %s tx transfer ibc-0 ibc-1 100000stake raw:non-bech32-address channel-0 --path demo
$ %s tx raw send ibc-0 ibc-1 100000stake cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk channel-0 --path demo -c 5
`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, ok := a.config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}
			dst, ok := a.config.Chains[args[1]]
			if !ok {
				return errChainNotFound(args[1])
			}

			pathString, err := cmd.Flags().GetString(flagPath)
			if err != nil {
				return err
			}

			var path *relayer.Path
			if path, err = setPathsFromArgs(a, src, dst, pathString); err != nil {
				return err
			}

			amount, err := sdk.ParseCoinNormalized(args[2])
			if err != nil {
				return err
			}

			srch, err := src.ChainProvider.QueryLatestHeight(cmd.Context())
			if err != nil {
				return err
			}

			// Query all channels for the configured connection on the src chain
			srcChannelID := args[4]

			var pathConnectionID string
			if src.ChainID() == path.Src.ChainID {
				pathConnectionID = path.Src.ConnectionID
			} else if src.ChainID() == path.Dst.ChainID {
				pathConnectionID = path.Dst.ConnectionID
			} else {
				return fmt.Errorf("no path configured using chain-id: %s", src.ChainID())
			}

			channels, err := src.ChainProvider.QueryConnectionChannels(cmd.Context(), srch, pathConnectionID)
			if err != nil {
				return err
			}

			// Ensure the specified channel exists for the given path
			var srcChannel *chantypes.IdentifiedChannel
			for _, channel := range channels {
				if channel.ChannelId == srcChannelID {
					srcChannel = channel
					break
				}
			}

			if srcChannel == nil {
				return fmt.Errorf("could not find channel{%s} for chain{%s}@connection{%s}",
					srcChannelID, src, pathConnectionID)
			}

			dts, err := src.ChainProvider.QueryDenomTraces(cmd.Context(), 0, 100, srch)
			if err != nil {
				return err
			}

			for _, d := range dts {
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
			var dstAddr string
			dstAddr = args[3]
			if rawDstAddr != args[3] {
				// Don't parse the rest of the dstAddr... it's raw.
				dstAddr = rawDstAddr
			}

			return src.SendTransferMsg(cmd.Context(), a.log, dst, amount, dstAddr, toHeightOffset, toTimeOffset, srcChannel)
		},
	}

	return timeoutFlags(a.viper, pathFlag(a.viper, cmd))
}

func setPathsFromArgs(a *appState, src, dst *relayer.Chain, name string) (*relayer.Path, error) {
	// find any configured paths between the chains
	paths, err := a.config.Paths.PathsFromChains(src.ChainID(), dst.ChainID())
	if err != nil {
		return nil, err
	}

	// Given the number of args and the number of paths, work on the appropriate
	// path.
	var path *relayer.Path
	switch {
	case name != "" && len(paths) > 1:
		if path, err = paths.Get(name); err != nil {
			return nil, err
		}

	case name != "" && len(paths) == 1:
		if path, err = paths.Get(name); err != nil {
			return nil, err
		}

	case name == "" && len(paths) > 1:
		return nil, fmt.Errorf("more than one path between %s and %s exists, pass in path name", src.ChainID(), dst.ChainID())

	case name == "" && len(paths) == 1:
		for _, v := range paths {
			path = v
		}
	}

	if err := src.SetPath(path.End(src.ChainID())); err != nil {
		return nil, err
	}

	if err := dst.SetPath(path.End(dst.ChainID())); err != nil {
		return nil, err
	}

	return path, nil
}

// ensureKeysExist returns an error if a configured key for a given chain does not exist.
func ensureKeysExist(chains map[string]*relayer.Chain) error {
	for _, v := range chains {
		if exists := v.ChainProvider.KeyExists(v.ChainProvider.Key()); !exists {
			return fmt.Errorf("key %s not found on chain %s", v.ChainProvider.Key(), v.ChainID())
		}
	}

	return nil
}

// MsgRegisterCounterpartyPayee registers the counterparty_payee
func registerCounterpartyCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "register-counterparty chain_name channel_id port_id relay_addr counterparty_payee",
		Aliases: []string{"reg-cpt"},
		Short:   "register the counterparty relayer address for ics-29 fee middleware",
		Args:    withUsage(cobra.MatchAll(cobra.ExactArgs(5))),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s register-counterparty channel-1 transfer cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk juno1g0ny488ws4064mjjxk4keenwfjrthn503ngjxd
$ %s reg-cpt channel-1 cosmos1skjwj5whet0lpe65qaq4rpq03hjxlwd9nf39lk juno1g0ny488ws4064mjjxk4keenwfjrthn503ngjxd`,
			appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {

			chain, ok := a.config.Chains[args[0]]
			if !ok {
				return errChainNotFound(args[0])
			}

			channelID := args[1]
			portID := args[2]

			relayerAddr := args[3]
			counterpartyPayee := args[4]

			msg, err := chain.ChainProvider.MsgRegisterCounterpartyPayee(portID, channelID, relayerAddr, counterpartyPayee)
			if err != nil {
				return err
			}
			res, success, err := chain.ChainProvider.SendMessage(cmd.Context(), msg, "")
			fmt.Println(res, success, err)
			return nil
		},
	}
	return cmd
}
