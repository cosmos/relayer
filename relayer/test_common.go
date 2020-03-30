package relayer

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	// TODO: replace this codec with the gaia codec

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/go-amino"
)

var (
	// GAIA BLOCK TIMEOUTS on jackzampolin/gaiatest:jack_relayer-testing
	// timeout_commit = "500ms"
	// timeout_propose = "500ms"
	// 1 second relayer timeout works well with these block times
	gaiaTestConfig = testChainConfig{
		cdc:            codecstd.NewAppCodec(codecstd.MakeCodec(simapp.ModuleBasics)),
		amino:          codecstd.MakeCodec(simapp.ModuleBasics),
		dockerImage:    "jackzampolin/gaiatest",
		dockerTag:      "jack_relayer-testing",
		timeout:        3 * time.Second,
		rpcPort:        "26657",
		accountPrefix:  "cosmos",
		gas:            200000,
		gasPrices:      "0.025stake",
		defaultDenom:   "stake",
		trustingPeriod: "330h",
	}
)

type (
	// testChain represents the different configuration options for spinning up a test
	// cosmos-sdk based blockchain
	testChain struct {
		chainID string
		t       testChainConfig
	}

	// testChainConfig represents the chain specific docker and codec configurations
	// required.
	testChainConfig struct {
		dockerImage    string
		dockerTag      string
		cdc            *codecstd.Codec
		amino          *amino.Codec
		rpcPort        string
		timeout        time.Duration
		accountPrefix  string
		gas            uint64
		gasPrices      string
		defaultDenom   string
		trustingPeriod string
	}
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
	require.NoError(t, c.createTestKey())

	// setup docker options
	dockerOpts := &dockertest.RunOptions{
		Name:         fmt.Sprintf("%s-%s", c.ChainID, t.Name()),
		Repository:   tc.t.dockerImage,
		Tag:          tc.t.dockerTag,
		Cmd:          []string{c.ChainID, c.MustGetAddress().String()},
		ExposedPorts: []string{tc.t.rpcPort},
		PortBindings: map[dc.Port][]dc.PortBinding{
			dc.Port(tc.t.rpcPort): []dc.PortBinding{{HostPort: c.getRPCPort()}},
		},
	}

	// create the proper docker image with port forwarding setup
	var resource *dockertest.Resource
	resource, err = pool.RunWithOptions(dockerOpts)
	require.NoError(t, err)

	c.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", c.ChainID, resource.Container.Name, resource.Container.Config.Image))

	// retry polling the container until status doesn't error
	if err = pool.Retry(c.statusErr); err != nil {
		require.NoError(t, fmt.Errorf("Could not connect to container at %s: %s", c.RPCAddr, err))
	}

	c.Log(fmt.Sprintf("- [%s] CONTAINER AVAILABLE AT PORT %s", c.ChainID, c.RPCAddr))

	// initalize the lite client
	require.NoError(t, c.forceInitLite())

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

// newTestChain generates a new instance of *Chain with a free TCP port configured as the RPC port
func newTestChain(t *testing.T, tc testChain) *Chain {
	_, port, err := server.FreeTCPAddr()
	require.NoError(t, err)
	return &Chain{
		Key:            "testkey",
		ChainID:        tc.chainID,
		RPCAddr:        fmt.Sprintf("http://localhost:%s", port),
		AccountPrefix:  tc.t.accountPrefix,
		Gas:            tc.t.gas,
		GasPrices:      tc.t.gasPrices,
		DefaultDenom:   tc.t.defaultDenom,
		TrustingPeriod: tc.t.trustingPeriod,
	}
}

// testClientPair tests that the client for src on dst and dst on src are the only clients on those chains
func testClientPair(t *testing.T, src, dst *Chain) {
	testClient(t, src, dst)
	testClient(t, dst, src)
}

// testClient queries clients and client for dst on src and returns a variety of errors
// testClient expects just one client on src, that for dst
// TODO: we should be able to find the chain id of dst on src, add a case for this in each switch
func testClient(t *testing.T, src, dst *Chain) {
	clients, err := src.QueryClients(1, 1000)
	require.NoError(t, err)
	require.Equal(t, len(clients), 1)
	require.Equal(t, clients[0].GetID(), src.PathEnd.ClientID)

	client, err := src.QueryClientState()
	require.NoError(t, err)
	require.Equal(t, client.ClientState.GetID(), src.PathEnd.ClientID)
	require.Equal(t, client.ClientState.ClientType().String(), "tendermint")
}

// testConnectionPair tests that the only connection on src and dst is between the two chains
func testConnectionPair(t *testing.T, src, dst *Chain) {
	testConnection(t, src, dst)
	testConnection(t, dst, src)
}

// testConnection tests that the only connection on src has a counterparty that is the connection on dst
func testConnection(t *testing.T, src, dst *Chain) {
	conns, err := src.QueryConnections(1, 1000)
	require.NoError(t, err)
	require.Equal(t, len(conns), 1)
	require.Equal(t, conns[0].GetClientID(), src.PathEnd.ClientID)
	require.Equal(t, conns[0].GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conns[0].GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conns[0].GetState().String(), "OPEN")

	h, err := src.Client.Status()
	require.NoError(t, err)

	conn, err := src.QueryConnection(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, conn.Connection.GetClientID(), src.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conn.Connection.GetState().String(), "OPEN")
}

// testChannelPair tests that the only channel on src and dst is between the two chains
func testChannelPair(t *testing.T, src, dst *Chain) {
	testChannel(t, src, dst)
	testChannel(t, dst, src)
}

// testChannel tests that the only channel on src is a counterparty of dst
func testChannel(t *testing.T, src, dst *Chain) {
	chans, err := src.QueryChannels(1, 1000)
	require.NoError(t, err)
	require.Equal(t, 1, len(chans))
	require.Equal(t, chans[0].GetOrdering().String(), "ORDERED")
	require.Equal(t, chans[0].GetState().String(), "OPEN")
	require.Equal(t, chans[0].GetCounterparty().GetChannelID(), dst.PathEnd.ChannelID)
	require.Equal(t, chans[0].GetCounterparty().GetPortID(), dst.PathEnd.PortID)

	h, err := src.Client.Status()
	require.NoError(t, err)

	ch, err := src.QueryChannel(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, ch.Channel.GetOrdering().String(), "ORDERED")
	require.Equal(t, ch.Channel.GetState().String(), "OPEN")
	require.Equal(t, ch.Channel.GetCounterparty().GetChannelID(), dst.PathEnd.ChannelID)
	require.Equal(t, ch.Channel.GetCounterparty().GetPortID(), dst.PathEnd.PortID)
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
	path := GenPath(src.ChainID, dst.ChainID, srcPort, dstPort)
	if err := src.SetPath(path.Src); err != nil {
		return nil, err
	}
	if err := dst.SetPath(path.Dst); err != nil {
		return nil, err
	}
	return path, nil
}
