package relayer

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

var (
	// docker configuration
	dockerImage = "jackzampolin/gaiatest"
	dockerTag   = "jack_relayer-testing"
	defaultPort = "26657"
	defaultTo   = 3 * time.Second

	// codec for the chainz
	cdc      = codecstd.MakeCodec(simapp.ModuleBasics)
	appCodec = codecstd.NewAppCodec(cdc)

	// path to relay across
	testPath = &Path{
		Src: &PathEnd{
			ChainID:      "ibc0",
			ClientID:     "ibconeclient",
			ConnectionID: "ibconeconnection",
			ChannelID:    "ibconechan",
			PortID:       "transfer",
		},
		Dst: &PathEnd{
			ChainID:      "ibc1",
			ClientID:     "ibczeroclient",
			ConnectionID: "ibczeroconnection",
			ChannelID:    "ibczerochan",
			PortID:       "transfer",
		},
	}

	// test chains
	testChains = Chains{
		&Chain{
			Key:            "testkey",
			ChainID:        "ibc0",
			RPCAddr:        "http://localhost:26657",
			AccountPrefix:  "cosmos",
			Gas:            200000,
			GasPrices:      "0.025stake",
			DefaultDenom:   "stake",
			TrustingPeriod: "330h",
		},
		&Chain{
			Key:            "testkey",
			ChainID:        "ibc1",
			RPCAddr:        "http://localhost:26557",
			AccountPrefix:  "cosmos",
			Gas:            200000,
			GasPrices:      "0.025stake",
			DefaultDenom:   "stake",
			TrustingPeriod: "330h",
		},
	}
)

func TestMain(m *testing.M) {
	// Create temporary relayer test directory
	dir, err := ioutil.TempDir("", "relayer-test")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Created tmp dir", dir)

	// Clean up tests after run
	defer os.RemoveAll(dir)

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// Initialize the chains
	var resources []*dockertest.Resource

	for i, c := range testChains {
		// initialize the chain
		ch, err := c.Init(dir, appCodec, cdc, 10*time.Second, true)
		if err != nil {
			log.Fatalf("failed to initialize chain: %s", err)
		}

		log.Println("chain initialized", ch.ChainID)

		// reset the chain in the array
		testChains[i] = ch

		// create the test key
		if err = ch.createTestKey(); err != nil {
			log.Fatal(err)
		}

		log.Println("key created", ch.MustGetAddress().String())

		// create the proper docker image with port forwarding setup
		resource, err := pool.RunWithOptions(&dockertest.RunOptions{
			Name:         fmt.Sprintf("%s", ch.ChainID),
			Repository:   dockerImage,
			Tag:          dockerTag,
			Cmd:          []string{c.ChainID, ch.MustGetAddress().String()},
			ExposedPorts: []string{defaultPort},
			PortBindings: map[dc.Port][]dc.PortBinding{
				dc.Port(defaultPort): []dc.PortBinding{{HostPort: ch.getRPCPort()}},
			},
		})

		if err != nil {
			log.Fatal(err)
		}

		resources = append(resources, resource)

		// retry polling the container until status doesn't error
		if err := pool.Retry(func() error {
			stat, err := ch.Client.Status()
			switch {
			case err != nil:
				return err
			case stat.SyncInfo.LatestBlockHeight < 3:
				return fmt.Errorf("haven't produced any blocks yet")
			default:
				log.Println(ch.RPCAddr)
				return nil
			}
		}); err != nil {
			log.Fatalf("Could not connect to docker: %s", err)
		}

		// initalize the lite client
		db, df, err := ch.NewLiteDB()
		if err != nil {
			log.Fatal(err)
		}
		_, err = ch.TrustNodeInitClient(db)
		if err != nil {
			log.Fatal(err)
		}
		df()

	}

	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	for _, r := range resources {
		if err := pool.Purge(r); err != nil {
			log.Fatalf("Could not purge resource: %s", err)
		}
	}

	os.Exit(code)
}

func TestSomething(t *testing.T) {
	src, dst := testChains.MustGet(testPath.Src.ChainID), testChains.MustGet(testPath.Dst.ChainID)

	// Check for any (unexpected) path errors
	if err := src.SetPath(testPath.Src); err != nil {
		t.Error(err)
		t.Fail()
	}
	if err := dst.SetPath(testPath.Dst); err != nil {
		t.Error(err)
		t.Fail()
	}

	// Check if clients have been created, if not create them
	if err := src.CreateClients(dst); err != nil {
		t.Error(err)
		t.Fail()
	}

	// Check if connection has been created, if not create it
	if err := src.CreateConnection(dst, defaultTo); err != nil {
		t.Error(err)
		t.Fail()
	}

	// Check if channel has been created, if not create it
	if err := src.CreateChannel(dst, true, defaultTo); err != nil {
		t.Error(err)
		t.Fail()
	}
}
