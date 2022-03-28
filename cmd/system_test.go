package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/cosmos/relayer/cmd"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
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

// Run calls s.RunWithInput with an empty stdin.
func (s *System) Run(log *zap.Logger, args ...string) RunResult {
	return s.RunWithInput(log, bytes.NewReader(nil), args...)
}

// RunWithInput executes the root command with the given args,
// providing in as the command's standard input,
// and returns a RunResult that has its Stdout and Stderr populated.
func (s *System) RunWithInput(log *zap.Logger, in io.Reader, args ...string) RunResult {
	rootCmd := cmd.NewRootCmd(log, zap.NewAtomicLevel())
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

	res.Err = rootCmd.Execute()
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
func (s *System) MustAddChain(t *testing.T, pcw cmd.ProviderConfigWrapper) {
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
	res := s.MustRun(t, "chains", "add", "--file", f.Name())
	require.Empty(t, res.Stdout.String())
	require.Empty(t, res.Stderr.String())
}

// A fixed mnemonic and its resulting cosmos address, helpful for tests that need a mnemonic.
const (
	ZeroMnemonic   = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
	ZeroCosmosAddr = "cosmos1r5v5srda7xfth3hn2s26txvrcrntldjumt8mhl"
)
