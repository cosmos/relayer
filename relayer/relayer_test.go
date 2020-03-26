package relayer

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

var (
	// docker configuration
	dockerImage = "jackzampolin/gaiatest"
	dockerTag   = "jack_relayer-testing"
	defaultPort = "26657"
	xferPort    = "transfer"
	defaultTo   = 3 * time.Second

	// gaia codec for the chainz
	gaiaCdc = codecstd.MakeCodec(simapp.ModuleBasics)
)

func TestBasicTransfer(t *testing.T) {
	chains, testsDone, containersDone, err := spinUpTestChains("ibc0", "ibc1")
	if err != nil {
		t.Error(err)
	}

	src, dst := chains[0], chains[1]
	if _, err := genTestPathAndSet(src, dst, xferPort, xferPort); err != nil {
		t.Error(err)
	}

	// Check if clients have been created, if not create them
	if err := src.CreateClients(dst); err != nil {
		t.Error(err)
	}

	// Check if connection has been created, if not create it
	if err := src.CreateConnection(dst, defaultTo); err != nil {
		t.Error(err)
	}

	// Check if channel has been created, if not create it
	if err := src.CreateChannel(dst, true, defaultTo); err != nil {
		t.Error(err)
	}

	testsDone <- struct{}{}
	<-containersDone
}

func spinUpTestChains(chainIDs ...string) (Chains, chan<- struct{}, <-chan struct{}, error) {
	// Initialize variables
	var (
		resources []*dockertest.Resource
		chains    []*Chain
		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
		// wg        sync.WaitGroup
	)

	for _, id := range chainIDs {
		chains = append(chains, testChain(id))
	}

	// Create temporary relayer test directory
	dir, err := ioutil.TempDir("", "relayer-test")
	if err != nil {
		return nil, nil, nil, err
	}

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Could not connect to docker: %w", err)
	}

	for _, c := range chains {
		// initialize the chain
		if err = c.Init(dir, codecstd.NewAppCodec(gaiaCdc), gaiaCdc, defaultTo, true); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to initialize chain: %w", err)
		}

		// create the test key
		if err = c.createTestKey(); err != nil {
			return nil, nil, nil, err
		}

		dockerOpts := &dockertest.RunOptions{
			Name:         fmt.Sprintf("%s-%s", c.ChainID, randString(8)),
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
			return nil, nil, nil, err
		}

		// retry polling the container until status doesn't error
		if err = pool.Retry(c.statusErr); err != nil {
			return nil, nil, nil, fmt.Errorf("Could not connect to docker: %s", err)
		}

		// initalize the lite client
		if err = c.forceInitLite(); err != nil {
			log.Fatal(err)
		}

		resources = append(resources, resource)
	}

	// spin off the cleanup goroutine
	go func(testsDone <-chan struct{}, contDone chan<- struct{}, resources []*dockertest.Resource, pool *dockertest.Pool, dir string) {
		// BLOCK HERE TILL CHANNEL SEND
		<-testsDone

		// clean up the tmp dir
		os.RemoveAll(dir)

		// remove all the docker containers
		for _, r := range resources {
			if err := pool.Purge(r); err != nil {
				log.Fatalf("Could not purge resource: %s", err)
			}
		}

		// Notify the other side that we have deleted the docker containers
		contDone <- struct{}{}
	}(testsDone, contDone, resources, pool, dir)

	return chains, testsDone, contDone, nil
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
