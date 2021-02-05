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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientutils "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/client/utils"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	ibctmtypes "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// startCmd represents the start command
// NOTE: This is basically psuedocode
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
			strategy, err := GetStrategyWithOptions(cmd, path.MustGetStrategy())
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

			done, err := relayer.RunStrategy(c[src], c[dst], strategy)
			if err != nil {
				return err
			}

			timeInterval := viper.GetDuration(flagTimeInterval)

			go func() {
				for {
					// run for every 60 seconds
					if err := UpdateClientsFromChains(c[src], c[dst]); err != nil {
						fmt.Printf("Error in updating clients....%v", err)
					}
					time.Sleep(timeInterval)
				}
			}()

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

// UpdateClientsFromChains takes src, dst chains and update clients based on expiry time
func UpdateClientsFromChains(src, dst *relayer.Chain) (err error) {
	if src.PathEnd.ClientID == "" || dst.PathEnd.ClientID == "" {
		return nil
	}
	eg := new(errgroup.Group)
	eg.Go(func() error {
		err = GetClientAndUpdate(src, dst)
		return err
	})
	eg.Go(func() error {
		err = GetClientAndUpdate(dst, src)
		return err
	})
	err = eg.Wait()
	return err
}

// GetClientAndUpdate update clients to prevent expiry
func GetClientAndUpdate(src, dst *relayer.Chain) error {
	height, err := src.QueryLatestHeight()
	if err != nil {
		return err
	}

	clientStateRes, err := src.QueryClientState(height)
	if err != nil {
		return err
	}

	// unpack any into ibc tendermint client state
	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return err
	}

	// cast from interface to concrete type
	clientState, ok := clientStateExported.(*ibctmtypes.ClientState)
	if !ok {
		return fmt.Errorf("error when casting exported clientstate on chain: %s", src.PathEnd.ClientID)
	}

	// query the latest consensus state of the potential matching client
	consensusStateResp, err := clientutils.QueryConsensusStateABCI(src.CLIContext(0),
		src.PathEnd.ClientID, clientState.GetLatestHeight())
	if err != nil {
		return err
	}

	exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
	if err != nil {
		return err
	}

	consensusState, ok := exportedConsState.(*ibctmtypes.ConsensusState)
	if !ok {
		return fmt.Errorf("error: consensus state is not tendermint type on chain %s", src.PathEnd.ChainID)
	}

	expirationTime := consensusState.Timestamp.Add(clientState.TrustingPeriod)

	checkExpiryTime := viper.GetFloat64(flagThresholdTime)

	// Checking expiration time is below 10minutes to current time
	if expirationTime.After(time.Now()) && time.Until(expirationTime).Minutes() <= checkExpiryTime {
		sh, err := relayer.NewSyncHeaders(src, dst)
		if err != nil {
			return err
		}

		dstUH, err := sh.GetTrustedHeader(dst, src)
		if err != nil {
			return err
		}

		msgs := []sdk.Msg{
			src.UpdateClient(dstUH),
		}

		_, _, err = src.SendMsgs(msgs)
		if err != nil {
			return err
		}
		src.Log(fmt.Sprintf("★ Client updated: [%s]client(%s) {%d}->{%d}",
			src.ChainID,
			src.PathEnd.ClientID,
			relayer.MustGetHeight(dstUH.TrustedHeight),
			dstUH.Header.Height,
		))
	} else if !expirationTime.After(time.Now()) {
		fmt.Printf("Client is already expired on chain: %s", src.ChainID)
	}
	return nil
}
