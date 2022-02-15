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
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/spf13/viper"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// startCmd represents the start command
// NOTE: This is basically pseudocode
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start [path-name]",
		Aliases: []string{"st"},
		Short:   "Start the listening relayer on a given path",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s start demo-path --max-msgs 3
$ %s start demo-path2 --max-tx-size 10`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			if err = ensureKeysExist(c); err != nil {
				return err
			}

			path := config.Paths.MustGet(args[0])
			maxTxSize, maxMsgLength, err := GetStartOptions(cmd)
			if err != nil {
				return err
			}

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

			done, err := relayer.StartRelayer(c[src], c[dst], maxTxSize, maxMsgLength)
			if err != nil {
				c[src].Log(fmt.Sprintf("relayer start error. Err: %v", err))
			}

			thresholdTime := viper.GetDuration(flagThresholdTime)

			eg := new(errgroup.Group)
			eg.Go(func() error {
				for {
					var timeToExpiry time.Duration
					if err := retry.Do(func() error {
						timeToExpiry, err = UpdateClientsFromChains(c[src], c[dst], thresholdTime)
						if err != nil {
							return err
						}
						return nil
					}, retry.Attempts(5), retry.Delay(time.Millisecond*500), retry.LastErrorOnly(true)); err != nil {
						return err
					}
					time.Sleep(timeToExpiry - thresholdTime)
				}
			})
			if err = eg.Wait(); err != nil {
				c[src].Log(fmt.Sprintf("update clients error. Err: %v", err))
			}

			trapSignal(done)
			return nil
		},
	}
	return strategyFlag(updateTimeFlags(cmd))
}

// trap signal waits for a SIGINT or SIGTERM and then sends down the done channel
func trapSignal(done func()) {
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// wait for a signal
	sig := <-sigCh
	fmt.Println("Signal Received", sig.String())
	close(sigCh)

	// call the cleanup func
	done()
}

// UpdateClientsFromChains takes src, dst chains, threshold time and update clients based on expiry time
func UpdateClientsFromChains(src, dst *relayer.Chain, thresholdTime time.Duration) (time.Duration, error) {
	var (
		srcTimeExpiry, dstTimeExpiry time.Duration
		err                          error
	)

	eg := new(errgroup.Group)
	eg.Go(func() error {
		srcTimeExpiry, err = src.ChainProvider.AutoUpdateClient(dst.ChainProvider, thresholdTime, src.ClientID(), dst.ClientID())
		return err
	})
	eg.Go(func() error {
		dstTimeExpiry, err = dst.ChainProvider.AutoUpdateClient(src.ChainProvider, thresholdTime, dst.ClientID(), src.ClientID())
		return err
	})
	if err := eg.Wait(); err != nil {
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
