// Package relayertest enables testing the relayer command-line interface
// from within Go unit tests.
package relayertest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cosmos/relayer/v2/cmd"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"
)

// System is a system under test.
type System struct {
	// Temporary directory to be injected as --home argument.
	HomeDir string
}

// NewSystem creates a new system with a home dir associated with a temp dir belonging to t.
//
// The returned System does not store a reference to t;
// some of its methods expect a *testing.T as an argument.
// This allows creating one instance of System to be shared with subtests.
func NewSystem(t *testing.T) *System {
	t.Helper()

	homeDir := t.TempDir()

	return &System{
		HomeDir: homeDir,
	}
}

// RunResult is the stdout and stderr resulting from a call to (*System).Run,
// and any error that was returned.
type RunResult struct {
	Stdout, Stderr bytes.Buffer

	Err error
}

// Run calls s.RunC with context.Background().
func (s *System) Run(log *zap.Logger, args ...string) RunResult {
	return s.RunC(context.Background(), log, args...)
}

// RunC calls s.RunWithInputC with an empty stdin.
func (s *System) RunC(ctx context.Context, log *zap.Logger, args ...string) RunResult {
	return s.RunWithInputC(ctx, log, bytes.NewReader(nil), args...)
}

// RunWithInput is shorthand for RunWithInputC(context.Background(), ...).
func (s *System) RunWithInput(log *zap.Logger, in io.Reader, args ...string) RunResult {
	return s.RunWithInputC(context.Background(), log, in, args...)
}

// RunWithInputC executes the root command with the given context and args,
// providing in as the command's standard input,
// and returns a RunResult that has its Stdout and Stderr populated.
func (s *System) RunWithInputC(ctx context.Context, log *zap.Logger, in io.Reader, args ...string) RunResult {
	rootCmd := cmd.NewRootCmd(log)
	rootCmd.SetIn(in)
	// cmd.Execute also sets SilenceUsage,
	// so match that here for more correct assertions.
	rootCmd.SilenceUsage = true

	var res RunResult
	rootCmd.SetOutput(&res.Stdout)
	rootCmd.SetErr(&res.Stderr)

	// Prepend the system's home directory to any provided args.
	args = append([]string{"--home", s.HomeDir}, args...)
	rootCmd.SetArgs(args)

	res.Err = rootCmd.ExecuteContext(ctx)
	return res
}

// MustRun calls Run, but also calls t.Fatal if RunResult.Err is not nil.
func (s *System) MustRun(t *testing.T, args ...string) RunResult {
	t.Helper()

	return s.MustRunWithInput(t, bytes.NewReader(nil), args...)
}

// MustRunWithInput calls RunWithInput, but also calls t.Fatal if RunResult.Err is not nil.
func (s *System) MustRunWithInput(t *testing.T, in io.Reader, args ...string) RunResult {
	t.Helper()

	res := s.RunWithInput(zaptest.NewLogger(t), in, args...)
	if res.Err != nil {
		t.Logf("Error executing %v: %v", args, res.Err)
		t.Logf("Stdout: %q", res.Stdout.String())
		t.Logf("Stderr: %q", res.Stderr.String())
		t.FailNow()
	}

	return res
}

// MustAddChain serializes pcw to disk and calls "chains add --file".
func (s *System) MustAddChain(t *testing.T, chainName string, pcw cmd.ProviderConfigWrapper) {
	t.Helper()

	// Temporary file for the new chain.
	tmpdir := t.TempDir()
	f, err := os.CreateTemp(tmpdir, "chain*.json")
	require.NoError(t, err)
	defer f.Close() // Defer close in case the test fails before we explicitly close.

	enc := json.NewEncoder(f)
	require.NoError(t, enc.Encode(pcw))
	f.Close() // Now that the content has been written, close the file so it is readable by the 'chains add' call.

	// Add the chain. Output is expected to be silent.
	res := s.MustRun(t, "chains", "add", "--file", f.Name(), chainName)
	require.Empty(t, res.Stdout.String())
	require.Empty(t, res.Stderr.String())
}

// MustAddChain serializes pcw to disk and calls "chains add --file".
func (s *System) MustGetConfig(t *testing.T) (config cmd.ConfigInputWrapper) {
	t.Helper()

	configBz, err := os.ReadFile(filepath.Join(s.HomeDir, "config", "config.yaml"))
	require.NoError(t, err, "failed to read config file")

	err = yaml.Unmarshal(configBz, &config)
	require.NoError(t, err, "failed to unmarshal config file")

	return config
}

// A fixed mnemonic and its resulting cosmos address, helpful for tests that need a mnemonic.
const (
	ZeroMnemonic   = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
	ZeroCosmosAddr = "cosmos1r5v5srda7xfth3hn2s26txvrcrntldjumt8mhl"
)
