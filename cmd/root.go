/*
Package cmd includes relayer commands
Copyright © 2020 Jack Zampolin jack.zampolin@gmail.com

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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
)

const (
	MB      = 1024 * 1024 // in bytes
	appName = "rly"
)

var defaultHome = filepath.Join(os.Getenv("HOME"), ".relayer")

const (
	// Default identifiers for dummy usage
	dcli = "defaultclientid"
	dcon = "defaultconnectionid"
)

// NewRootCmd returns the root command for relayer.
// If log is nil, a new zap.Logger is set on the app state
// based on the command line flags regarding logging.
func NewRootCmd(log *zap.Logger) *cobra.Command {
	// Use a local app state instance scoped to the new root command,
	// so that tests don't concurrently access the state.
	a := &appState{
		Viper: viper.New(),

		Log: log,
	}

	// RootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   appName,
		Short: "This application makes data relay between IBC enabled chains easy!",
		Long: strings.TrimSpace(`rly has:
   1. Configuration management for Chains and Paths
   2. Key management for managing multiple keys for multiple chains
   3. Query and transaction functionality for IBC

   NOTE: Most of the commands have aliases that make typing them much quicker 
         (i.e. 'rly tx', 'rly q', etc...)`),
	}

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Inside persistent pre-run because this takes effect after flags are parsed.
		if log == nil {
			log, err := newRootLogger(a.Viper.GetString("log-format"), a.Viper.GetBool("debug"))
			if err != nil {
				return err
			}

			a.Log = log
		}

		// reads `homeDir/config/config.yaml` into `a.Config`
		return initConfig(rootCmd, a)
	}

	rootCmd.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
		// Force syncing the logs before exit, if anything is buffered.
		a.Log.Sync()
	}

	// Register --home flag
	rootCmd.PersistentFlags().StringVar(&a.HomePath, flagHome, defaultHome, "set home directory")
	if err := a.Viper.BindPFlag(flagHome, rootCmd.PersistentFlags().Lookup(flagHome)); err != nil {
		panic(err)
	}

	// Register --debug flag
	rootCmd.PersistentFlags().BoolVarP(&a.Debug, "debug", "d", false, "debug output")
	if err := a.Viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		panic(err)
	}

	rootCmd.PersistentFlags().String("log-format", "auto", "log output format (auto, logfmt, json, or console)")
	if err := a.Viper.BindPFlag("log-format", rootCmd.PersistentFlags().Lookup("log-format")); err != nil {
		panic(err)
	}

	// Register subcommands
	rootCmd.AddCommand(
		configCmd(a),
		chainsCmd(a),
		pathsCmd(a),
		keysCmd(a),
		lineBreakCommand(),
		transactionCmd(a),
		queryCmd(a),
		startCmd(a),
		lineBreakCommand(),
		getVersionCmd(a),
	)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false

	rootCmd := NewRootCmd(nil)
	rootCmd.SilenceUsage = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt) // Using signal.Notify, instead of signal.NotifyContext, in order to see details of signal.
	go func() {
		// Wait for interrupt signal.
		sig := <-sigCh

		// Cancel context on root command.
		// If the invoked command respects this quickly, the main goroutine will quit right away.
		cancel()

		// Short delay before printing the received signal message.
		// This should result in cleaner output from non-interactive commands that stop quickly.
		time.Sleep(250 * time.Millisecond)
		fmt.Fprintf(os.Stderr, "Received signal %v. Attempting clean shutdown. Send interrupt again to force hard shutdown.\n", sig)

		// Dump all goroutines on panic, not just the current one.
		debug.SetTraceback("all")

		// Block waiting for a second interrupt or a timeout.
		// The main goroutine ought to finish before either case is reached.
		// But if a case is reached, panic so that we get a non-zero exit and a dump of remaining goroutines.
		select {
		case <-time.After(time.Minute):
			panic(errors.New("rly did not shut down within one minute of interrupt"))
		case sig := <-sigCh:
			panic(fmt.Errorf("received signal %v; forcing quit", sig))
		}
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func newRootLogger(format string, debug bool) (*zap.Logger, error) {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	config.LevelKey = "lvl"

	var enc zapcore.Encoder
	switch format {
	case "json":
		enc = zapcore.NewJSONEncoder(config)
	case "console":
		enc = zapcore.NewConsoleEncoder(config)
	case "logfmt":
		enc = zaplogfmt.NewEncoder(config)
	case "auto":
		if term.IsTerminal(int(os.Stderr.Fd())) {
			// When a user runs relayer in the foreground, use easier to read output.
			enc = zapcore.NewConsoleEncoder(config)
		} else {
			// Otherwise, use consistent logfmt format for simplistic machine processing.
			enc = zaplogfmt.NewEncoder(config)
		}
	default:
		return nil, fmt.Errorf("unrecognized log format %q", format)
	}

	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}
	return zap.New(zapcore.NewCore(
		enc,
		os.Stderr,
		level,
	)), nil
}

// readLine reads one line from the given reader.
func readLine(in io.Reader) (string, error) {
	str, err := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(str), err
}

// lineBreakCommand returns a new instance of the lineBreakCommand every time to avoid
// data races in concurrent tests exercising commands.
func lineBreakCommand() *cobra.Command {
	return &cobra.Command{Run: func(*cobra.Command, []string) {}}
}

// withUsage wraps a PositionalArgs to display usage only when the PositionalArgs
// variant is violated.
func withUsage(inner cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := inner(cmd, args); err != nil {
			cmd.Root().SilenceUsage = false
			cmd.SilenceUsage = false
			return err
		}

		return nil
	}
}
