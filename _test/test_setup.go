package test

import (
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/relayer/relayer"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"

	sdked25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdkcryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	tmed25519 "github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
)

// spinUpTestChains is to be passed any number of test chains with given configuration options
// to be created as individual docker containers at the beginning of a test. It is safe to run
// in parallel tests as all created resources are independent of eachother
func spinUpTestChains(t *testing.T, testChains ...testChain) relayer.Chains {
	var (
		resources []*dockertest.Resource
		chains    = make([]*relayer.Chain, len(testChains))

		wg    sync.WaitGroup
		rchan = make(chan *dockertest.Resource, len(testChains))

		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
	)

	// Create temporary relayer test directory
	dir := t.TempDir()

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		require.NoError(t, fmt.Errorf("could not connect to docker at %s: %w", pool.Client.Endpoint(), err))
	}

	var eg errgroup.Group
	// make each container and initialize the chains
	for i, tc := range testChains {
		tc := tc
		c := newTestChain(t, tc)
		chains[i] = c
		wg.Add(1)
		genPrivValKeyJSON(tc.seed)
		eg.Go(func() error {
			return spinUpTestContainer(rchan, pool, c, tc)
		})
	}

	// wait for all containers to be created
	require.NoError(t, eg.Wait())

	// read all the containers out of the channel
	for i := 0; i < len(chains); i++ {
		r := <-rchan
		resources = append(resources, r)
	}

	// close the channel
	close(rchan)

	// start the wait for cleanup function
	go cleanUpTest(t, testsDone, contDone, resources, pool, dir, chains)

	// set the test cleanup function
	t.Cleanup(func() {
		testsDone <- struct{}{}
		<-contDone
	})

	// return the chains and the doneFunc
	return chains
}

func removeTestContainer(pool *dockertest.Pool, containerName string) error {
	containers, err := pool.Client.ListContainers(dc.ListContainersOptions{
		All: true,
		Filters: map[string][]string{
			"name": {containerName},
		},
	})
	if err != nil {
		return fmt.Errorf("error while listing containers with name %s: %w", containerName, err)
	}

	if len(containers) == 0 {
		return nil
	}

	err = pool.Client.RemoveContainer(dc.RemoveContainerOptions{
		ID:            containers[0].ID,
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil {
		return fmt.Errorf("error while removing container with name %s: %w", containerName, err)
	}

	return nil
}

// spinUpTestContainer spins up a test container with the given configuration
// A docker image is built for each chain using its provided configuration.
// This image is then ran using the options set below.
func spinUpTestContainer(rchan chan<- *dockertest.Resource, pool *dockertest.Pool, c *relayer.Chain, tc testChain) error {
	var (
		err      error
		debug    bool
		resource *dockertest.Resource
	)

	// add extra logging if TEST_DEBUG=true
	if val, ok := os.LookupEnv("TEST_DEBUG"); ok {
		debug, err = strconv.ParseBool(val)
		if err != nil {
			debug = false
		}
	}

	// initialize the chain
	// TODO: is there a better logger we can use for tests?
	c.Init(log.NewTMLogger(log.NewSyncWriter(os.Stderr)), debug)

	// create the test key
	if err := c.CreateTestKey(); err != nil {
		return err
	}

	containerName := c.ChainID()

	// setup docker options
	addr, err := c.ChainProvider.Address()
	if err != nil {
		return err
	}

	dockerOpts := &dockertest.RunOptions{
		Name:         containerName,
		Repository:   containerName, // Name must match Repository
		Tag:          "latest",      // Must match docker default build tag
		ExposedPorts: []string{tc.t.rpcPort, c.GetRPCPort()},
		Cmd: []string{
			c.ChainID(),
			addr,
			getPrivValFileName(tc.seed),
		},
		PortBindings: map[dc.Port][]dc.PortBinding{
			dc.Port(tc.t.rpcPort): {{HostPort: c.GetRPCPort()}},
		},
	}

	if err := removeTestContainer(pool, containerName); err != nil {
		return err
	}

	// create the proper docker image with port forwarding setup
	d, err := os.Getwd()
	if err != nil {
		return err
	}

	buildOpts := &BuildOptions{
		Dockerfile: tc.t.dockerfile,
		ContextDir: path.Dir(d),
		BuildArgs:  tc.t.buildArgs,
	}
	hcOpt := func(hc *dc.HostConfig) {
		hc.LogConfig.Type = "json-file"
	}

	resource, err = BuildAndRunWithBuildOptions(pool, buildOpts, dockerOpts, hcOpt)
	if err != nil {
		return err
	}

	c.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", c.ChainID(),
		resource.Container.Name, resource.Container.Config.Image))

	// retry polling the container until status doesn't error
	//if err = pool.Retry(c.StatusErr); err != nil {
	//	return fmt.Errorf("could not connect to container at %s: %s", c.RPCAddr, err)
	//}

	// TODO maybe this works?
	time.Sleep(time.Second * 5)

	c.Log(fmt.Sprintf("- [%s] CONTAINER AVAILABLE AT PORT %s", c.ChainID(), c.RPCAddr))

	rchan <- resource
	return nil
}

// cleanUpTest is called as a goroutine to wait until the tests have completed and
// cleans up the docker containers and relayer config
func cleanUpTest(t *testing.T, testsDone <-chan struct{}, contDone chan<- struct{},
	resources []*dockertest.Resource, pool *dockertest.Pool, dir string, chains []*relayer.Chain) {
	// block here until tests are complete
	<-testsDone

	// clean up the tmp dir
	if err := os.RemoveAll(dir); err != nil {
		require.NoError(t, fmt.Errorf("{cleanUpTest} failed to rm dir(%w), %s ", err, dir))
	}

	// remove all the docker containers
	for i, r := range resources {
		if err := pool.Purge(r); err != nil {
			require.NoError(t, fmt.Errorf("could not purge container %s: %w", r.Container.Name, err))
		}
		c := getLoggingChain(chains, r)
		chains[i].Log(fmt.Sprintf("- [%s] SPUN DOWN CONTAINER %s from %s", c.ChainID(), r.Container.Name,
			r.Container.Config.Image))
	}

	// Notify the other side that we have deleted the docker containers
	contDone <- struct{}{}
}

// for the love of logs https://www.youtube.com/watch?v=DtsKcHmceqY
func getLoggingChain(chns []*relayer.Chain, rsr *dockertest.Resource) *relayer.Chain {
	for _, c := range chns {
		if strings.Contains(rsr.Container.Name, c.ChainID()) {
			return c
		}
	}
	return nil
}

func genTestPathAndSet(src, dst *relayer.Chain) (*relayer.Path, error) {
	p := relayer.GenPath(src.ChainID(), dst.ChainID())

	src.PathEnd = p.Src
	dst.PathEnd = p.Dst
	return p, nil
}

func genPrivValKeyJSON(seedNumber int) {
	privKey := getPrivKey(seedNumber)
	filePV := getFilePV(privKey, seedNumber)
	filePV.Key.Save()
}

func getPrivKey(seedNumber int) tmed25519.PrivKey {
	return tmed25519.GenPrivKeyFromSecret([]byte(seeds[seedNumber]))
}

func getSDKPrivKey(seedNumber int) sdkcryptotypes.PrivKey {
	return sdked25519.GenPrivKeyFromSecret([]byte(seeds[seedNumber]))
}

func getFilePV(privKey tmed25519.PrivKey, seedNumber int) *privval.FilePV {
	return privval.NewFilePV(privKey, getPrivValFileName(seedNumber), "/")
}

func getPrivValFileName(seedNumber int) string {
	return fmt.Sprintf("./setup/valkeys/priv_val%d.json", seedNumber)
}

type BuildOptions struct {
	Dockerfile string
	ContextDir string
	BuildArgs  []dc.BuildArg
}

var muDockerBuild sync.Mutex

// BuildAndRunWithBuildOptions builds and starts a docker container.
// Optional modifier functions can be passed in order to change the hostconfig values not covered in RunOptions
func BuildAndRunWithBuildOptions(pool *dockertest.Pool, buildOpts *BuildOptions, runOpts *dockertest.RunOptions, hcOpts ...func(*dc.HostConfig)) (*dockertest.Resource, error) {

	muDockerBuild.Lock()
	defer muDockerBuild.Unlock()
	err := pool.Client.BuildImage(dc.BuildImageOptions{
		Name:         runOpts.Name,
		Dockerfile:   buildOpts.Dockerfile,
		OutputStream: io.Discard,
		ContextDir:   buildOpts.ContextDir,
		BuildArgs:    buildOpts.BuildArgs,
	})

	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	runOpts.Repository = runOpts.Name

	return pool.RunWithOptions(runOpts, hcOpts...)
}
