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
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// startCmd represents the start command
// NOTE: This is basically pseudocode
func startCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start [path-name]",
		Aliases: []string{"st"},
		Short:   "Start the listening relayer on a given path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s start demo-path --max-msgs 3
$ %s start demo-path2 --max-tx-size 10`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := a.Config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = ensureKeysExist(c); err != nil {
				return err
			}

			path := a.Config.Paths.MustGet(args[0])

			maxTxSize, maxMsgLength, err := GetStartOptions(cmd)
			if err != nil {
				return err
			}

			filter := path.Filter

			if relayer.SendToController != nil {
				action := relayer.PathAction{
					Path: path,
					Type: "RELAYER_PATH_START",
				}
				cont, err := relayer.ControllerUpcall(&action)
				if !cont {
					return err
				}
			}

			errorChan := relayer.StartRelayer(cmd.Context(), c[src], c[dst], filter, maxTxSize, maxMsgLength)

			// NOTE: This block of code is useful for ensuring that the clients tracking each chain do not expire
			// when there are no packets flowing across the channels. It is currently a source of errors that have been
			// hard to rectify, so we are just avoiding this code path for now
			if false {
				thresholdTime := a.Viper.GetDuration(flagThresholdTime)
				eg, egCtx := errgroup.WithContext(cmd.Context())

				eg.Go(func() error {
					for {
						var timeToExpiry time.Duration
						if err = retry.Do(func() error {
							timeToExpiry, err = UpdateClientsFromChains(egCtx, c[src], c[dst], thresholdTime)
							return err
						}, retry.Context(egCtx), retry.Attempts(5), retry.Delay(time.Millisecond*500), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
							if a.Debug {
								c[src].Log(fmt.Sprintf("- [%s]<->[%s] - try(%d/%d) updating clients from chains: %s",
									c[src].ChainID(), c[dst].ChainID(), n+1, relayer.RtyAttNum, err))
							}
						})); err != nil {
							return err
						}
						time.Sleep(timeToExpiry - thresholdTime)
					}
				})
				if err = eg.Wait(); err != nil {
					c[src].Log(fmt.Sprintf("update clients error. Err: %v", err))
				}
			}

			// Block until the error channel sends a message.
			// The context being canceled will cause the relayer to stop,
			// so we don't want to separately monitor the ctx.Done channel,
			// because we would risk returning before the relayer cleans up.
			if err := <-errorChan; err != nil && !errors.Is(err, context.Canceled) {
				c[src].Log(fmt.Sprintf("relayer start error. Err: %v", err))
				return err
			}
			return nil
		},
	}
	return strategyFlag(a.Viper, updateTimeFlags(a.Viper, cmd))
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
