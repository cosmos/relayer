package ibctest_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/moby/moby/client"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/conformance"
	"github.com/strangelove-ventures/ibctest/ibc"
	ibctestrelayer "github.com/strangelove-ventures/ibctest/relayer"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const relayerImageName = "ibctestrelayer"

// TestRelayer runs the ibctest conformance tests against
// the current state of this relayer implementation.
//
// This is meant to be a relatively quick sanity check,
// so it uses only one pair of chains.
//
// The canonical set of test chains are defined in the ibctest repository.
func TestRelayer(t *testing.T) {
	buildRelayerImage(t)

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.1", ChainConfig: ibc.ChainConfig{ChainID: "cosmoshub-1004"}},
		{Name: "osmosis", Version: "v7.2.0", ChainConfig: ibc.ChainConfig{ChainID: "osmosis-1001"}},
	})

	relayerFactory := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		ibctestrelayer.CustomDockerImage(relayerImageName, "latest"),
		ibctestrelayer.ImagePull(false),
		ibctestrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
	)

	conformance.Test(
		t,
		[]ibctest.ChainFactory{cf},
		[]ibctest.RelayerFactory{relayerFactory},
		// The nop write closer means no test report will be generated,
		// which is fine for these tests for now.
		testreporter.NewReporter(newNopWriteCloser()),
	)
}

type dockerLogLine struct {
	Stream      string            `json:"stream"`
	Aux         any               `json:"aux"`
	Error       string            `json:"error"`
	ErrorDetail dockerErrorDetail `json:"errorDetail"`
}

type dockerErrorDetail struct {
	Message string `json:"message"`
}

func buildRelayerImage(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Join(filepath.Dir(b), "..")

	tar, err := archive.TarWithOptions(basepath, &archive.TarOptions{})
	require.NoError(t, err, "error archiving relayer for docker image build")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "error building docker client")

	res, err := cli.ImageBuild(context.Background(), tar, dockertypes.ImageBuildOptions{
		Dockerfile: "local.Dockerfile",
		Tags:       []string{relayerImageName},
	})
	require.NoError(t, err, "error building docker image")

	defer res.Body.Close()
	handleDockerBuildOutput(t, res.Body)
}

func handleDockerBuildOutput(t *testing.T, body io.Reader) {
	var logLine dockerLogLine

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		logLine.Stream = ""
		logLine.Aux = nil
		logLine.Error = ""
		logLine.ErrorDetail = dockerErrorDetail{}

		line := scanner.Text()

		_ = json.Unmarshal([]byte(line), &logLine)
		if logLine.Stream != "" {
			fmt.Print(logLine.Stream)
		}
		if logLine.Aux != nil {
			fmt.Println(logLine.Aux)
		}
	}

	require.Equalf(t, "", logLine.Error, "docker image build error: %s", logLine.ErrorDetail.Message)
}

// nopWriteCloser is a no-op io.WriteCloser used to satisfy the ibctest TestReporter type.
// Because the relayer is used in-process, all logs are simply streamed to the test log.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

func newNopWriteCloser() io.WriteCloser {
	return nopWriteCloser{Writer: io.Discard}
}
