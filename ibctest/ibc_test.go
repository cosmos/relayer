package ibctest_test

import (
	"bufio"
	"context"
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

// TestRelayer runs the ibctest conformance tests against
// the current state of this relayer implementation.
//
// This is meant to be a relatively quick sanity check,
// so it uses only one pair of chains.
//
// The canonical set of test chains are defined in the ibctest repository.
func TestRelayer(t *testing.T) {
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.1", ChainConfig: ibc.ChainConfig{ChainID: "cosmoshub-1004"}},
		{Name: "osmosis", Version: "v7.2.0", ChainConfig: ibc.ChainConfig{ChainID: "osmosis-1001"}},
	})

	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Join(filepath.Dir(b), "..")

	tar, err := archive.TarWithOptions(basepath, &archive.TarOptions{})
	require.NoError(t, err, "error archiving relayer for docker image build")

	const dockerImageName = "ibctestrelayer"

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "error building docker client")

	res, err := cli.ImageBuild(context.Background(), tar, dockertypes.ImageBuildOptions{
		Dockerfile: "local.Dockerfile",
		Tags:       []string{dockerImageName},
	})
	if err != nil {
		if res.Body == nil {
			t.Fatal("error building docker image, body is nil: ", err)
		}
		scanner := bufio.NewScanner(res.Body)
		if scanner == nil {
			t.Fatal("error building docker image, scanner is nil: ", err)
		}
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		t.Fatal("error building docker image", err)
	}

	relayerFactory := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t), ibctestrelayer.CustomDockerImage(dockerImageName, "latest"), ibctestrelayer.ImagePull(false))

	conformance.Test(
		t,
		[]ibctest.ChainFactory{cf},
		[]ibctest.RelayerFactory{relayerFactory},
		// The nop write closer means no test report will be generated,
		// which is fine for these tests for now.
		testreporter.NewReporter(newNopWriteCloser()),
	)
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
