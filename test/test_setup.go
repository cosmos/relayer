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

	. "github.com/iqlusioninc/relayer/relayer"
)

// spinUpTestChains is to be passed any number of test chains with given configuration options
// to be created as individual docker containers at the beginning of a test. It is safe to run
// in parallel tests as all created resources are independent of eachother
func spinUpTestChains(t *testing.T, testChains ...testChain) Chains {
	var (
		resources []*dockertest.Resource
		chains    []*Chain

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
		require.NoError(t, fmt.Errorf("Could not connect to docker at %s: %w", pool.Client.Endpoint(), err))
	}

	// make each container and initalize the chains
	for _, tc := range testChains {
		c := newTestChain(t, tc)
		chains = append(chains, c)
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
			"name":  {containerName},
			"label": {"io.iqlusion.relayer.test=true"},
		},
	})
	if err != nil {
		return fmt.Errorf("Error while listing containers with name %s %w", containerName, err)
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
		return fmt.Errorf("Error while removing container with name %s %w", containerName, err)
	}

	return nil
}

// spinUpTestContainer spins up a test container with the given configuration
func spinUpTestContainer(t *testing.T, rchan chan<- *dockertest.Resource, pool *dockertest.Pool, c *Chain, dir string, wg *sync.WaitGroup, tc testChain) {
	defer wg.Done()
	var err error

	// add extra logging if TEST_DEBUG=true
	var debug bool
	if val, ok := os.LookupEnv("TEST_DEBUG"); ok {
		debug, err = strconv.ParseBool(val)
		if err != nil {
			debug = false
		}
	}

	// initialize the chain
	require.NoError(t, c.Init(dir, tc.t.cdc, tc.t.amino, tc.t.timeout, debug))

	// create the test key
	require.NoError(t, c.CreateTestKey())

	containerName := fmt.Sprintf("%s-%s", c.ChainID, t.Name())
	// setup docker options
	dockerOpts := &dockertest.RunOptions{
		Name:         containerName,
		Repository:   tc.t.dockerImage,
		Tag:          tc.t.dockerTag,
		ExposedPorts: []string{tc.t.rpcPort},
		PortBindings: map[dc.Port][]dc.PortBinding{
			dc.Port(tc.t.rpcPort): {{HostPort: c.GetRPCPort()}},
		},
	}

	func() {
		// Ensure our address is encoded properly.
		defer c.UseSDKContext()()
		dockerOpts.Cmd = []string{c.ChainID, c.MustGetAddress().String()}
		dockerOpts.Labels = make(map[string]string)
		dockerOpts.Labels["io.iqlusion.relayer.test"] = "true"
	}()

	err = removeTestContainer(pool, containerName)
	require.NoError(t, err)
	// create the proper docker image with port forwarding setup
	var resource *dockertest.Resource
	resource, err = pool.RunWithOptions(dockerOpts)
	require.NoError(t, err)

	c.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", c.ChainID, resource.Container.Name, resource.Container.Config.Image))

	// retry polling the container until status doesn't error
	if err = pool.Retry(c.StatusErr); err != nil {
		require.NoError(t, fmt.Errorf("Could not connect to container at %s: %s", c.RPCAddr, err))
	}

	c.Log(fmt.Sprintf("- [%s] CONTAINER AVAILABLE AT PORT %s", c.ChainID, c.RPCAddr))

	// initalize the lite client
	require.NoError(t, c.ForceInitLite())

	rchan <- resource
}

// cleanUpTest is called as a goroutine to wait until the tests have completed and cleans up the docker containers and relayer config
func cleanUpTest(t *testing.T, testsDone <-chan struct{}, contDone chan<- struct{}, resources []*dockertest.Resource, pool *dockertest.Pool, dir string, chains []*Chain) {
	// block here until tests are complete
	<-testsDone

	// clean up the tmp dir
	if err := os.RemoveAll(dir); err != nil {
		require.NoError(t, fmt.Errorf("{cleanUpTest} failed to rm dir(%w), %s ", err, dir))
	}

	// remove all the docker containers
	for i, r := range resources {
		if err := pool.Purge(r); err != nil {
			require.NoError(t, fmt.Errorf("Could not purge container %s: %w", r.Container.Name, err))
		}
		c := getLoggingChain(chains, r)
		chains[i].Log(fmt.Sprintf("- [%s] SPUN DOWN CONTAINER %s from %s", c.ChainID, r.Container.Name, r.Container.Config.Image))
	}

	// Notify the other side that we have deleted the docker containers
	contDone <- struct{}{}
}

// for the love of logs https://www.youtube.com/watch?v=DtsKcHmceqY
func getLoggingChain(chns []*Chain, rsr *dockertest.Resource) *Chain {
	for _, c := range chns {
		if strings.Contains(rsr.Container.Name, c.ChainID) {
			return c
		}
	}
	return nil
}

func genTestPathAndSet(src, dst *Chain, srcPort, dstPort string) (*Path, error) {
	path := GenPath(src.ChainID, dst.ChainID, srcPort, dstPort, "ORDERED")
	if err := src.SetPath(path.Src); err != nil {
		return nil, err
	}
	if err := dst.SetPath(path.Dst); err != nil {
		return nil, err
	}
	return path, nil
}
