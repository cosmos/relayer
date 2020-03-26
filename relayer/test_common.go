package relayer

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

func testConnectionPair(src, dst *Chain) (err error) {
	if err = testConnection(src, dst); err != nil {
		return err
	}
	if err = testConnection(dst, src); err != nil {
		return err
	}
	return nil
}

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

func testClientPair(src, dst *Chain) (err error) {
	if err = testClient(src, dst); err != nil {
		return err
	}
	if err = testClient(dst, src); err != nil {
		return err
	}
	return nil
}

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

func spinUpTestChains(t *testing.T, chainIDs ...string) (Chains, func()) {
	// Initialize variables
	var (
		resources []*dockertest.Resource
		chains    []*Chain
		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
		wg        sync.WaitGroup
	)

	for _, id := range chainIDs {
		chains = append(chains, testChain(id))
	}

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

	// make chan to catch all the container resources
	rchan := make(chan *dockertest.Resource, len(chains))

	// make each container
	for _, c := range chains {
		wg.Add(1)
		go spinUpContainer(t, rchan, pool, c, dir, &wg)
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
	go cleanUpTest(t, testsDone, contDone, resources, pool, dir)
	doneFunc := func() {
		testsDone <- struct{}{}
		<-contDone
	}

	return chains, doneFunc
}

func cleanUpTest(t *testing.T, testsDone <-chan struct{}, contDone chan<- struct{}, resources []*dockertest.Resource, pool *dockertest.Pool, dir string) {
	// BLOCK HERE TILL CHANNEL SEND
	<-testsDone

	// clean up the tmp dir
	if err := os.RemoveAll(dir); err != nil {
		t.Errorf("{cleanUpTest} failed to rm dir(%s), %s ", dir, err)
	}

	// remove all the docker containers
	for _, r := range resources {
		if err := pool.Purge(r); err != nil {
			t.Errorf("Could not purge container %s: %w", r.Container.Name, err)
		}
	}

	// Notify the other side that we have deleted the docker containers
	contDone <- struct{}{}
}

func spinUpContainer(t *testing.T, rchan chan<- *dockertest.Resource, pool *dockertest.Pool, c *Chain, dir string, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	// initialize the chain
	if err = c.Init(dir, codecstd.NewAppCodec(gaiaCdc), gaiaCdc, defaultTo, true); err != nil {
		t.Error(err)
	}

	// create the test key
	if err = c.createTestKey(); err != nil {
		t.Error(err)
	}

	dockerOpts := &dockertest.RunOptions{
		Name:         fmt.Sprintf("%s-%s", c.ChainID, t.Name()),
		Repository:   dockerImage,
		Tag:          dockerTag,
		Cmd:          []string{c.ChainID, c.MustGetAddress().String()},
		ExposedPorts: []string{defaultPort},
		PortBindings: map[dc.Port][]dc.PortBinding{
			dc.Port(defaultPort): []dc.PortBinding{{HostPort: c.getRPCPort()}},
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

	// initalize the lite client
	if err = c.forceInitLite(); err != nil {
		t.Error(err)
	}

	rchan <- resource
}

func testChain(chainID string) *Chain {
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
	path := genTestPath(src, dst, srcPort, dstPort)
	if err := src.SetPath(path.Src); err != nil {
		return nil, err
	}
	if err := dst.SetPath(path.Dst); err != nil {
		return nil, err
	}
	return path, nil
}

func genTestPath(src, dst *Chain, srcPort, dstPort string) *Path {
	return &Path{
		Src: &PathEnd{
			ChainID:      src.ChainID,
			ClientID:     randString(10),
			ConnectionID: randString(10),
			ChannelID:    randString(10),
			PortID:       srcPort,
		},
		Dst: &PathEnd{
			ChainID:      dst.ChainID,
			ClientID:     randString(10),
			ConnectionID: randString(10),
			ChannelID:    randString(10),
			PortID:       dstPort,
		},
	}
}

func randString(length int) string {
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	var b strings.Builder
	for i := 0; i < length; i++ {
		i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b.WriteRune(chars[i.Int64()])
	}
	return b.String()
}
