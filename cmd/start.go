/*
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
	"syscall"

	"github.com/iqlusioninc/relayer/relayer"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
// NOTE: This is basically psuedocode
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start [path-name]",
		Aliases: []string{"st"},
		Short:   "Start the listening relayer on a given path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, src, dst, err := config.ChainsFromPath(args[0])
			if err != nil {
				return err
			}

			path := config.Paths.MustGet(args[0])
			done, err := relayer.RunStrategy(c[src], c[dst], path.MustGetStrategy(), path.Ordered())
			if err != nil {
				return err
			}

			trapSignal(done)
			return nil
		},
	}
	return cmd
}

// trap signal waits for a SIGINT or SIGTERM and then sends down the done channel
func trapSignal(done func()) {
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// wait for a signal
	sig := <-sigCh
	fmt.Println("Signal Recieved:", sig.String())
	close(sigCh)

	// call the cleanup func
	done()
}
