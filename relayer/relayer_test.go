package relayer

import (
	"testing"
	"time"

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/simapp"
)

var (
	// docker configuration
	dockerImage = "jackzampolin/gaiatest"
	dockerTag   = "jack_relayer-testing"
	defaultPort = "26657"
	xferPort    = "transfer"

	// GAIA BLOCK TIMEOUTS
	// timeout_commit = "500ms"
	// timeout_propose = "500ms"
	defaultTo = 1 * time.Second

	// gaia codec for the chainz
	gaiaCdc = codecstd.MakeCodec(simapp.ModuleBasics)
)

func TestBasicTransfer(t *testing.T) {
	srcCID, dstCID := "ibc0", "ibc1"
	chains, doneFunc := spinUpTestChains(t, srcCID, dstCID)
	t.Cleanup(doneFunc)
	src, dst := chains.MustGet(srcCID), chains.MustGet(dstCID)

	if _, err := genTestPathAndSet(src, dst, xferPort, xferPort); err != nil {
		t.Error(err)
	}

	// Check if clients have been created, if not create them
	if err := src.CreateClients(dst); err != nil {
		t.Error(err)
	}

	if err := testClientPair(src, dst); err != nil {
		t.Error(err)
	}

	// Check if connection has been created, if not create it
	if err := src.CreateConnection(dst, defaultTo); err != nil {
		t.Error(err)
	}

	// Test querying connections from src and dst sides
	if err := testConnectionPair(src, dst); err != nil {
		t.Error(err)
	}

	// Check if channel has been created, if not create it
	if err := src.CreateChannel(dst, true, defaultTo); err != nil {
		t.Error(err)
	}
}
