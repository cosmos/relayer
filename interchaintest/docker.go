package interchaintest

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
	"github.com/google/uuid"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

const RelayerImagePrefix = "interchaintestrelayer"

type dockerLogLine struct {
	Stream      string            `json:"stream"`
	Aux         any               `json:"aux"`
	Error       string            `json:"error"`
	ErrorDetail dockerErrorDetail `json:"errorDetail"`
}

type dockerErrorDetail struct {
	Message string `json:"message"`
}

func uniqueRelayerImageName() (string, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid %v", err)
	}
	return RelayerImagePrefix + uuid.String()[:6], nil
}
func BuildRelayerImage(t *testing.T) string {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Join(filepath.Dir(b), "..")

	tar, err := archive.TarWithOptions(basepath, &archive.TarOptions{})
	require.NoError(t, err, "error archiving relayer for docker image build")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "error building docker client")

	image, err := uniqueRelayerImageName()
	require.NoError(t, err, "error generating unique tag for docker image")

	res, err := cli.ImageBuild(context.Background(), tar, dockertypes.ImageBuildOptions{
		Dockerfile: "local.Dockerfile",
		Tags:       []string{image},
	})
	require.NoError(t, err, "error building docker image")

	defer res.Body.Close()
	t.Cleanup(func() {
		destroyRelayerImage(t, image)
	})
	handleDockerBuildOutput(t, res.Body)
	return image
}

func destroyRelayerImage(t *testing.T, image string) {
	// Create a Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "error building docker client")

	// Remove the Docker image using the provided tag (uniquestr)
	_, err = cli.ImageRemove(context.Background(), image, dockertypes.ImageRemoveOptions{
		Force:         true, // Force remove the image
		PruneChildren: true, // Remove all child images
	})
	require.NoError(t, err, "error removing docker image")
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
			fmt.Print(logLine.Aux)
		}
	}

	require.Equalf(t, "", logLine.Error, "docker image build error: %s", logLine.ErrorDetail.Message)
}
