package cmd_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/cosmos/relayer/cmd"
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
func (s *System) Run(args ...string) RunResult {
	return s.RunWithInput(bytes.NewReader(nil), args...)
}

// RunWithInput executes the root command with the given args,
// providing in as the command's standard input,
// and returns a RunResult that has its Stdout and Stderr populated.
func (s *System) RunWithInput(in io.Reader, args ...string) RunResult {
	rootCmd := cmd.NewRootCmd()
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

	res := s.RunWithInput(in, args...)
	if res.Err != nil {
		t.Logf("Error executing %v: %v", args, res.Err)
		t.Logf("Stdout: %q", res.Stdout.String())
		t.Logf("Stderr: %q", res.Stderr.String())
		t.FailNow()
	}

	return res
}
