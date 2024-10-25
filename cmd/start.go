/*
Package cmd includes relayer commands
Copyright © 2020 Jack Zampolin <jack.zampolin@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/cosmos/relayer/v2/internal/relaydebug"
	"github.com/cosmos/relayer/v2/internal/relayermetrics"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// startCmd represents the start command
func startCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start path_name",
		Aliases: []string{"st"},
		Short:   "Start the listening relayer on a given path",
		Args:    withUsage(cobra.MinimumNArgs(0)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s start           # start all configured paths
$ %s start demo-path # start the 'demo-path' path
$ %s start demo-path --max-msgs 3
$ %s start demo-path2 --max-tx-size 10`, appName, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			chains := make(map[string]*relayer.Chain)
			paths := make([]relayer.NamedPath, len(args))

			if len(args) > 0 {
				for i, pathName := range args {
					path := a.config.Paths.MustGet(pathName)
					paths[i] = relayer.NamedPath{
						Name: pathName,
						Path: path,
					}

					// collect unique chain IDs
					chains[path.Src.ChainID] = nil
					chains[path.Dst.ChainID] = nil
				}
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

			err = setupDebugServer(cmd, a, err)
			if err != nil {
				return err
			}

			prometheusMetrics, err := setupMetricsServer(cmd, a, err, chains)
			if err != nil {
				return err
			}

			processorType, err := cmd.Flags().GetString(flagProcessor)
			if err != nil {
				return err
			}

			initialBlockHistory, err := cmd.Flags().GetUint64(flagInitialBlockHistory)
			if err != nil {
				return err
			}

			clientUpdateThresholdTime, err := cmd.Flags().GetDuration(flagThresholdTime)
			if err != nil {
				return err
			}

			flushInterval, err := cmd.Flags().GetDuration(flagFlushInterval)
			if err != nil {
				return err
			}

			stuckPacket, err := parseStuckPacketFromFlags(cmd)
			if err != nil {
				return err
			}

			rlyErrCh := relayer.StartRelayer(
				cmd.Context(),
				a.log,
				chains,
				paths,
				maxMsgLength,
				a.config.Global.MaxReceiverSize,
				a.config.Global.ICS20MemoLimit,
				a.config.memo(cmd),
				clientUpdateThresholdTime,
				flushInterval,
				nil,
				processorType,
				initialBlockHistory,
				prometheusMetrics,
				stuckPacket,
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
	cmd = updateTimeFlags(a.viper, cmd)
	cmd = strategyFlag(a.viper, cmd)
	cmd = debugServerFlags(a.viper, cmd)
	cmd = metricsServerFlags(a.viper, cmd)
	cmd = processorFlag(a.viper, cmd)
	cmd = initBlockFlag(a.viper, cmd)
	cmd = flushIntervalFlag(a.viper, cmd)
	cmd = memoFlag(a.viper, cmd)
	cmd = stuckPacketFlags(a.viper, cmd)
	return cmd
}

func setupMetricsServer(cmd *cobra.Command, a *appState, err error, chains map[string]*relayer.Chain) (*processor.PrometheusMetrics, error) {
	var prometheusMetrics *processor.PrometheusMetrics

	metricsListenAddr := a.config.Global.MetricsListenPort

	metricsListenAddrFlag, err := cmd.Flags().GetString(flagMetricsListenAddr)
	if err != nil {
		return nil, err
	}

	if metricsListenAddrFlag != "" {
		metricsListenAddr = metricsListenAddrFlag
	}

	flagEnableMetricsServer, err := cmd.Flags().GetBool(flagEnableMetricsServer)
	if err != nil {
		return nil, err
	}

	if flagEnableMetricsServer == false {
		a.log.Info("Metrics server is disabled you can enable it using --enable-metrics-server flag")
	} else if metricsListenAddr == "" {
		a.log.Warn("Disabled metrics server due to missing metrics-listen-addr setting in config file or --metrics-listen-addr flag")
	} else {
		a.log.Info("Metrics server is enabled")
		ln, err := net.Listen("tcp", metricsListenAddr)
		if err != nil {
			a.log.Error(fmt.Sprintf("Failed to start metrics server you can change the address and port using metrics-listen-addr config settingh or --metrics-listen-flag"))

			return nil, fmt.Errorf("failed to listen on metrics address %q: %w", metricsListenAddr, err)
		}
		log := a.log.With(zap.String("sys", "metricshttp"))
		log.Info("Metrics server listening", zap.String("addr", metricsListenAddr))
		prometheusMetrics = processor.NewPrometheusMetrics()
		relayermetrics.StartMetricsServer(cmd.Context(), log, ln, prometheusMetrics.Registry)
		for _, chain := range chains {
			if ccp, ok := chain.ChainProvider.(*cosmos.CosmosProvider); ok {
				ccp.SetMetrics(prometheusMetrics)
			}
		}
	}
	return prometheusMetrics, nil
}

func setupDebugServer(cmd *cobra.Command, a *appState, err error) error {
	debugListenAddr := a.config.Global.DebugListenPort

	if debugListenAddr == "" {
		debugListenAddr = a.config.Global.ApiListenPort
		if debugListenAddr != "" {
			a.log.Warn("DEPRECATED: api-listen-addr config setting is deprecated use debug-listen-addr instead")
		}
	}

	debugAddrFlag, err := cmd.Flags().GetString(flagDebugAddr)
	if err != nil {
		return err
	}

	debugListenAddrFlag, err := cmd.Flags().GetString(flagDebugListenAddr)
	if err != nil {
		return err
	}

	if debugAddrFlag != "" {
		debugListenAddr = debugAddrFlag
		a.log.Warn("DEPRECATED: --debug-addr flag is deprecated use --enable-debug-server and --debug-listen-addr instead")
	}

	if debugListenAddrFlag != "" {
		debugListenAddr = debugListenAddrFlag
	}

	flagEnableDebugServer, err := cmd.Flags().GetBool(flagEnableDebugServer)
	if err != nil {
		return err
	}

	enableDebugServer := flagEnableDebugServer == true || debugAddrFlag != ""

	if enableDebugServer == false {
		a.log.Info("Debug server is disabled you can enable it using --enable-debug-server flag")
	} else if debugListenAddr == "" {
		a.log.Warn("Disabled debug server due to missing debug-listen-addr setting in config file or --debug-listen-addr flag")
	} else {
		a.log.Info("Debug server is enabled")
		a.log.Warn("SECURITY WARNING! Debug server should only be run with caution and proper security in place")
		ln, err := net.Listen("tcp", debugListenAddr)
		if err != nil {
			a.log.Error(fmt.Sprintf("Failed to start debug server you can change the address and port using debug-listen-addr config settingh or --debug-listen-flag"))

			return fmt.Errorf("failed to listen on debug address %q: %w", debugListenAddr, err)
		}
		log := a.log.With(zap.String("sys", "debughttp"))
		log.Info("Debug server listening", zap.String("addr", debugListenAddr))
		relaydebug.StartDebugServer(cmd.Context(), log, ln)
	}
	return nil
}
