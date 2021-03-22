package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"

	ry "github.com/cosmos/relayer/relayer"
)

// spinUpTestChains is to be passed any number of test chains with given configuration options
// to be created as individual docker containers at the beginning of a test. It is safe to run
// in parallel tests as all created resources are independent of eachother
func spinUpTestChains(t *testing.T, testChains ...testChain) ry.Chains {
	var (
		resources []*dockertest.Resource
		chains    = make([]*ry.Chain, len(testChains))

		wg    sync.WaitGroup
		rchan = make(chan *dockertest.Resource, len(testChains))

		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
	)

	// Create temporary relayer test directory
	dir, err := ioutil.TempDir("", "relayer-test")
	require.NoError(t, err)

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		require.NoError(t, fmt.Errorf("could not connect to docker at %s: %w", pool.Client.Endpoint(), err))
	}

	// make each container and initialize the chains
	for i, tc := range testChains {
		c := newTestChain(t, tc)
		chains[i] = c
		wg.Add(1)
		go spinUpTestContainer(t, rchan, pool, c, dir, &wg, tc)
	}

	// wait for all containers to be created
	wg.Wait()

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
		return fmt.Errorf("error while listing containers with name %s %w", containerName, err)
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
		return fmt.Errorf("error while removing container with name %s %w", containerName, err)
	}

	return nil
}

// spinUpTestContainer spins up a test container with the given configuration
// A docker image is built for each chain using its provided configuration.
// This image is then ran using the options set below.
func spinUpTestContainer(t *testing.T, rchan chan<- *dockertest.Resource,
	pool *dockertest.Pool, c *ry.Chain, dir string, wg *sync.WaitGroup, tc testChain) {
	defer wg.Done()
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
	require.NoError(t, c.Init(dir, tc.t.timeout, nil, debug))

	// create the test key
	require.NoError(t, c.CreateTestKey())

	containerName := c.ChainID

	// setup docker options
	dockerOpts := &dockertest.RunOptions{
		Name:         containerName,
		Repository:   containerName, // Name must match Repository
		Tag:          "latest",      // Must match docker default build tag
		ExposedPorts: []string{tc.t.rpcPort, c.GetRPCPort()},
		Cmd:          []string{c.ChainID, c.MustGetAddress().String()},
		PortBindings: map[dc.Port][]dc.PortBinding{
			dc.Port(tc.t.rpcPort): {{HostPort: c.GetRPCPort()}},
		},
	}

	// err = removeTestContainer(pool, containerName)
	require.NoError(t, removeTestContainer(pool, containerName))

	// create the proper docker image with port forwarding setup
	resource, err = pool.BuildAndRunWithOptions(tc.t.dockerfile, dockerOpts)
	require.NoError(t, err)

	c.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", c.ChainID,
		resource.Container.Name, resource.Container.Config.Image))

	// retry polling the container until status doesn't error
	if err = pool.Retry(c.StatusErr); err != nil {
		require.NoError(t, fmt.Errorf("could not connect to container at %s: %s", c.RPCAddr, err))
	}

	c.Log(fmt.Sprintf("- [%s] CONTAINER AVAILABLE AT PORT %s", c.ChainID, c.RPCAddr))

	// initialize the light client
	require.NoError(t, c.ForceInitLight())

	rchan <- resource
}

// cleanUpTest is called as a goroutine to wait until the tests have completed and
// cleans up the docker containers and relayer config
func cleanUpTest(t *testing.T, testsDone <-chan struct{}, contDone chan<- struct{},
	resources []*dockertest.Resource, pool *dockertest.Pool, dir string, chains []*ry.Chain) {
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
		chains[i].Log(fmt.Sprintf("- [%s] SPUN DOWN CONTAINER %s from %s", c.ChainID, r.Container.Name,
			r.Container.Config.Image))
	}

	// Notify the other side that we have deleted the docker containers
	contDone <- struct{}{}
}

// for the love of logs https://www.youtube.com/watch?v=DtsKcHmceqY
func getLoggingChain(chns []*ry.Chain, rsr *dockertest.Resource) *ry.Chain {
	for _, c := range chns {
		if strings.Contains(rsr.Container.Name, c.ChainID) {
			return c
		}
	}
	return nil
}

func genTestPathAndSet(src, dst *ry.Chain, srcPort, dstPort string) (*ry.Path, error) {
	path := ry.GenPath(src.ChainID, dst.ChainID, srcPort, dstPort, "UNORDERED", "ics20-1")

	src.PathEnd = path.Src
	dst.PathEnd = path.Dst
	return path, nil
}
