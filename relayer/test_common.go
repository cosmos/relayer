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

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/tendermint/go-amino"
)

// testChannelPair tests that the only channel on src and dst is between the two chains
func testChannelPair(src, dst *Chain) (err error) {
	if err = testChannel(src, dst); err != nil {
		return err
	}
	if err = testChannel(dst, src); err != nil {
		return err
	}
	return nil
}

// testChannel tests that the only channel on src is a counterparty of dst
func testChannel(src, dst *Chain) error {
	chans, err := src.QueryChannels(1, 1000)
	switch {
	case err != nil:
		return fmt.Errorf("[%s] failed to query channels on chain: %w", src.ChainID, err)
	case len(chans) != 1:
		return fmt.Errorf("[%s] too many (act(%d) != exp(1)) channels returned", src.ChainID, len(chans))
	// TODO: Change this testcase to pull expected order from PathEnd ()
	case chans[0].GetOrdering().String() != "ORDERED":
		return fmt.Errorf("[%s] channel order (act(%s) != exp(%s)) wrong for channel", src.ChainID, chans[0].GetOrdering().String(), "ORDERED")
	case chans[0].GetState().String() != "OPEN":
		return fmt.Errorf("[%s] channel state (act(%s) != exp(%s)) wrong for channel", src.ChainID, chans[0].GetState().String(), "OPEN")
	case chans[0].GetCounterparty().GetChannelID() != dst.PathEnd.ChannelID:
		return fmt.Errorf("[%s] cp channel id (act(%s) != exp(%s)) wrong on channel", src.ChainID, chans[0].GetCounterparty().GetChannelID(), dst.PathEnd.ChannelID)
	case chans[0].GetCounterparty().GetPortID() != dst.PathEnd.PortID:
		return fmt.Errorf("[%s] cp port id (act(%s) != exp(%s)) wrong on channel", src.ChainID, chans[0].GetCounterparty().GetPortID(), dst.PathEnd.PortID)
	default:
	}

	h, err := src.Client.Status()
	if err != nil {
		return fmt.Errorf("[%s] failed to query height: %w", src.ChainID, err)
	}

	ch, err := src.QueryChannel(h.SyncInfo.LatestBlockHeight)
	switch {
	case err != nil:
		return fmt.Errorf("[%s] failed to query channels on chain: %w", src.ChainID, err)
	case ch.Channel.GetOrdering().String() != "ORDERED":
		return fmt.Errorf("[%s] channel order (act(%s) != exp(%s)) wrong for channel", src.ChainID, ch.Channel.GetOrdering().String(), "ORDERED")
	case ch.Channel.GetState().String() != "OPEN":
		return fmt.Errorf("[%s] channel state (act(%s) != exp(%s)) wrong for channel", src.ChainID, ch.Channel.GetState().String(), "OPEN")
	case ch.Channel.GetCounterparty().GetChannelID() != dst.PathEnd.ChannelID:
		return fmt.Errorf("[%s] cp channel id (act(%s) != exp(%s)) wrong on channel", src.ChainID, ch.Channel.GetCounterparty().GetChannelID(), dst.PathEnd.ChannelID)
	case ch.Channel.GetCounterparty().GetPortID() != dst.PathEnd.PortID:
		return fmt.Errorf("[%s] cp port id (act(%s) != exp(%s)) wrong on channel", src.ChainID, ch.Channel.GetCounterparty().GetPortID(), dst.PathEnd.PortID)
	default:
		return nil
	}
}

// testConnectionPair tests that the only connection on src and dst is between the two chains
func testConnectionPair(src, dst *Chain) (err error) {
	if err = testConnection(src, dst); err != nil {
		return err
	}
	if err = testConnection(dst, src); err != nil {
		return err
	}
	return nil
}

// testConnection tests that the only connection on src has a counterparty that is the connection on dst
func testConnection(src, dst *Chain) error {
	conns, err := src.QueryConnections(1, 1000)
	switch {
	case err != nil:
		return fmt.Errorf("[%s] failed to query connections on chain: %w", src.ChainID, err)
	case len(conns) != 1:
		return fmt.Errorf("[%s] too many (act(%d) != exp(1)) connections returned", src.ChainID, len(conns))
	case conns[0].GetClientID() != src.PathEnd.ClientID:
		return fmt.Errorf("[%s] client id (act(%s) != exp(%s)) wrong on connection", src.ChainID, conns[0].GetClientID(), src.PathEnd.ClientID)
	case conns[0].GetCounterparty().GetClientID() != dst.PathEnd.ClientID:
		return fmt.Errorf("[%s] cp client id (act(%s) != exp(%s)) wrong on connection", src.ChainID, conns[0].GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	case conns[0].GetCounterparty().GetConnectionID() != dst.PathEnd.ConnectionID:
		return fmt.Errorf("[%s] cp connection id (act(%s) != exp(%s)) wrong on connection", src.ChainID, conns[0].GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	case conns[0].GetState().String() != "OPEN":
		return fmt.Errorf("[%s] connection state (act(%s) != exp(%s)) wrong for connection", src.ChainID, conns[0].GetState().String(), "OPEN")
	default:
	}

	h, err := src.Client.Status()
	if err != nil {
		return fmt.Errorf("[%s] failed to query height: %w", src.ChainID, err)
	}

	conn, err := src.QueryConnection(h.SyncInfo.LatestBlockHeight)
	switch {
	case err != nil:
		return fmt.Errorf("[%s] failed to query connection on chain: %w", src.ChainID, err)
	case conn.Connection.GetClientID() != src.PathEnd.ClientID:
		return fmt.Errorf("[%s] client id (act(%s) != exp(%s)) wrong on connection", src.ChainID, conn.Connection.GetClientID(), src.PathEnd.ClientID)
	case conn.Connection.GetCounterparty().GetClientID() != dst.PathEnd.ClientID:
		return fmt.Errorf("[%s] cp client id (act(%s) != exp(%s)) wrong on connection", src.ChainID, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	case conn.Connection.GetCounterparty().GetConnectionID() != dst.PathEnd.ConnectionID:
		return fmt.Errorf("[%s] cp connection id (act(%s) != exp(%s)) wrong on connection", src.ChainID, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	case conn.Connection.GetState().String() != "OPEN":
		return fmt.Errorf("[%s] connection state (act(%s) != exp(%s)) wrong for connection", src.ChainID, conn.Connection.GetState().String(), "OPEN")
	default:
		return nil
	}
}

// testClientPair tests that the client for src on dst and dst on src are the only clients on those chains
func testClientPair(src, dst *Chain) (err error) {
	if err = testClient(src, dst); err != nil {
		return err
	}
	if err = testClient(dst, src); err != nil {
		return err
	}
	return nil
}

// testClient queries clients and client for dst on src and returns a variety of errors
// testClient expects just one client on src, that for dst
// TODO: we should be able to find the chain id of dst on src, add a case for this in each switch
func testClient(src, dst *Chain) error {
	clients, err := src.QueryClients(1, 1000)
	switch {
	case err != nil:
		return fmt.Errorf("[%s] failed to query clients on chain: %w", src.ChainID, err)
	case len(clients) != 1:
		return fmt.Errorf("[%s] too many (act(%d) != exp(1)) clients returned", src.ChainID, len(clients))
	case clients[0].GetID() != src.PathEnd.ClientID:
		return fmt.Errorf("[%s] client id (act(%s) != exp(%s)) wrong on client", src.ChainID, clients[0].GetID(), src.PathEnd.ClientID)
	default:
	}

	client, err := src.QueryClientState()
	switch {
	case err != nil:
		return fmt.Errorf("[%s] failed to query clients on chain: %w", src.ChainID, err)
	case client.ClientState.GetID() != src.PathEnd.ClientID:
		return fmt.Errorf("[%s] client id (act(%s) != exp(%s)) wrong on client", src.ChainID, client.ClientState.GetID(), src.PathEnd.ClientID)
	case client.ClientState.ClientType().String() != "tendermint":
		return fmt.Errorf("[%s] client type (act(%s) != exp(tendermint)", src.ChainID, client.ClientState.ClientType().String())
	default:
		return nil
	}
}

// testChain represents the different configuration options for spinning up a test
// cosmos-sdk based blockchain
type testChain struct {
	chainID     string
	dockerImage string
	dockerTag   string
	cdc         *codecstd.Codec
	amino       *amino.Codec
	rpcPort     string
	timeout     time.Duration
}

// spinUpTestChains is to be passed any number of test chains with given configuration options
// to be created as individual docker containers at the beginning of a test. It is safe to run
// in parallel tests as all created resources are independent of eachother
func spinUpTestChains(t *testing.T, testChains ...testChain) (Chains, func()) {
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
	if err != nil {
		t.Error(err)
	}

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Errorf("Could not connect to docker: %w", err)
	}

	// make each container and initalize the chains
	for _, tc := range testChains {
		c := newTestChain(tc.chainID)
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

	// return the chains and the doneFunc
	return chains, func() {
		testsDone <- struct{}{}
		<-contDone
	}
}

// cleanUpTest is called as a goroutine to wait until the tests have completed and cleans up the docker containers and relayer config
func cleanUpTest(t *testing.T, testsDone <-chan struct{}, contDone chan<- struct{}, resources []*dockertest.Resource, pool *dockertest.Pool, dir string, chains []*Chain) {
	// block here until tests are complete
	<-testsDone

	// clean up the tmp dir
	if err := os.RemoveAll(dir); err != nil {
		t.Errorf("{cleanUpTest} failed to rm dir(%s), %s ", dir, err)
	}

	// remove all the docker containers
	for i, r := range resources {
		if err := pool.Purge(r); err != nil {
			t.Errorf("Could not purge container %s: %w", r.Container.Name, err)
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

// spinUpTestContainer spins up a test container with the given configuration
func spinUpTestContainer(t *testing.T, rchan chan<- *dockertest.Resource, pool *dockertest.Pool, c *Chain, dir string, wg *sync.WaitGroup, tc testChain) {
	defer wg.Done()
	var err error
	// initialize the chain

	// add extra logging if TEST_DEBUG=true
	var debug bool
	if val, ok := os.LookupEnv("TEST_DEBUG"); ok {
		debug, err = strconv.ParseBool(val)
		if err != nil {
			debug = false
		}
	}

	if err = c.Init(dir, tc.cdc, tc.amino, tc.timeout, debug); err != nil {
		t.Error(err)
	}

	// create the test key
	if err = c.createTestKey(); err != nil {
		t.Error(err)
	}

	dockerOpts := &dockertest.RunOptions{
		Name:         fmt.Sprintf("%s-%s", c.ChainID, t.Name()),
		Repository:   tc.dockerImage,
		Tag:          tc.dockerTag,
		Cmd:          []string{c.ChainID, c.MustGetAddress().String()},
		ExposedPorts: []string{tc.rpcPort},
		PortBindings: map[dc.Port][]dc.PortBinding{
			dc.Port(tc.rpcPort): []dc.PortBinding{{HostPort: c.getRPCPort()}},
		},
	}

	// create the proper docker image with port forwarding setup
	var resource *dockertest.Resource
	if resource, err = pool.RunWithOptions(dockerOpts); err != nil {
		t.Error(err)
	}

	// retry polling the container until status doesn't error
	if err = pool.Retry(c.statusErr); err != nil {
		t.Errorf("Could not connect to docker: %s", err)
	}

	c.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", c.ChainID, resource.Container.Name, resource.Container.Config.Image))

	// initalize the lite client
	if err = c.forceInitLite(); err != nil {
		t.Error(err)
	}

	rchan <- resource
}

// newTestChain generates a new instance of *Chain with a free TCP port configured as the RPC port
func newTestChain(chainID string) *Chain {
	_, port, err := server.FreeTCPAddr()
	if err != nil {
		return nil
	}
	return &Chain{
		Key:            "testkey",
		ChainID:        chainID,
		RPCAddr:        fmt.Sprintf("http://localhost:%s", port),
		AccountPrefix:  "cosmos",
		Gas:            200000,
		GasPrices:      "0.025stake",
		DefaultDenom:   "stake",
		TrustingPeriod: "330h",
	}
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
