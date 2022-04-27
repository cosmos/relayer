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
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/relayer/v2/internal/relaydebug"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// startCmd represents the start command
// NOTE: This is basically pseudocode
func startCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start path_name [path_name...]",
		Aliases: []string{"st"},
		Short:   "Start the listening relayer on a given path",
		Args:    withUsage(cobra.MinimumNArgs(1)),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s start demo-path --max-msgs 3
$ %s start demo-path2 --max-tx-size 10`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			maxTxSize, maxMsgLength, err := GetStartOptions(cmd)
			if err != nil {
				return err
			}

			debugAddr, err := cmd.Flags().GetString(flagDebugAddr)
			if err != nil {
				return err
			}
			if debugAddr == "" {
				a.Log.Info("Skipping debug server due to empty debug address flag")
			} else {
				ln, err := net.Listen("tcp", debugAddr)
				if err != nil {
					a.Log.Error("Failed to listen on debug address. If you have another relayer process open, use --" + flagDebugAddr + " to pick a different address.")
					return fmt.Errorf("failed to listen on debug address %q: %w", debugAddr, err)
				}
				log := a.Log.With(zap.String("sys", "debughttp"))
				log.Info("Debug server listening", zap.String("addr", debugAddr))
				relaydebug.StartDebugServer(cmd.Context(), log, ln)
			}

			eg, egCtx := errgroup.WithContext(cmd.Context())
			for _, arg := range args {
				c, src, dst, err := a.Config.ChainsFromPath(arg)
				if err != nil {
					return fmt.Errorf("failed to get chain from path %q: %w", arg, err)
				}

				if err := ensureKeysExist(c); err != nil {
					return fmt.Errorf("failed to get keys for path %q: %w", arg, err)
				}

				path := a.Config.Paths.MustGet(arg)

				eg.Go(func() error {
					rlyErrCh := relayer.StartRelayer(egCtx, a.Log, c[src], c[dst], path.Filter, maxTxSize, maxMsgLength)
					return <-rlyErrCh
				})
			}

			// Block until the error channel sends a message.
			// The context being canceled will cause the relayer to stop,
			// so we don't want to separately monitor the ctx.Done channel,
			// because we would risk returning before the relayer cleans up.
			if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
				a.Log.Warn(
					"Relayer start error",
					zap.Error(err),
				)
				return err
			}
			return nil
		},
	}
	return debugServerFlags(a.Viper, strategyFlag(a.Viper, updateTimeFlags(a.Viper, cmd)))
}

// UpdateClientsFromChains takes src, dst chains, threshold time and update clients based on expiry time
func UpdateClientsFromChains(ctx context.Context, src, dst *relayer.Chain, thresholdTime time.Duration) (time.Duration, error) {
	var (
		srcTimeExpiry, dstTimeExpiry time.Duration
		err                          error
	)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcTimeExpiry, err = src.ChainProvider.AutoUpdateClient(egCtx, dst.ChainProvider, thresholdTime, src.ClientID(), dst.ClientID())
		return err
	})
	eg.Go(func() error {
		var err error
		dstTimeExpiry, err = dst.ChainProvider.AutoUpdateClient(egCtx, src.ChainProvider, thresholdTime, dst.ClientID(), src.ClientID())
		return err
	})
	if err = eg.Wait(); err != nil {
		return 0, err
	}

	if srcTimeExpiry <= 0 {
		return 0, fmt.Errorf("client (%s) of chain: %s is expired",
			src.PathEnd.ClientID, src.ChainID())
	}

	if dstTimeExpiry <= 0 {
		return 0, fmt.Errorf("client (%s) of chain: %s is expired",
			dst.PathEnd.ClientID, dst.ChainID())
	}

	minTimeExpiry := math.Min(float64(srcTimeExpiry), float64(dstTimeExpiry))

	return time.Duration(int64(minTimeExpiry)), nil
}

// GetStartOptions sets strategy specific fields.
func GetStartOptions(cmd *cobra.Command) (uint64, uint64, error) {
	maxTxSize, err := cmd.Flags().GetString(flagMaxTxSize)
	if err != nil {
		return 0, 0, err
	}

	txSize, err := strconv.ParseUint(maxTxSize, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	maxMsgLength, err := cmd.Flags().GetString(flagMaxMsgLength)
	if err != nil {
		return txSize * MB, 0, err
	}

	msgLen, err := strconv.ParseUint(maxMsgLength, 10, 64)
	if err != nil {
		return txSize * MB, 0, err
	}

	return txSize * MB, msgLen, nil
}
