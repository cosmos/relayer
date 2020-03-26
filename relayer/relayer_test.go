package relayer

import (
	"testing"
	"time"

	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/simapp"
)

var (
	xferPort = "transfer"

	// gaia codec for the chainz
	gaiaCdc      = codecstd.MakeCodec(simapp.ModuleBasics)
	aminoGaiaCdc = codecstd.NewAppCodec(gaiaCdc)
)

func TestBasicTransfer(t *testing.T) {
	tcs := []testChain{
		// GAIA BLOCK TIMEOUTS on jackzampolin/gaiatest:jack_relayer-testing
		// timeout_commit = "500ms"
		// timeout_propose = "500ms"
		// relayer 1 second timeout
		{"ibc0", "jackzampolin/gaiatest", "jack_relayer-testing", aminoGaiaCdc, gaiaCdc, "26657", 1 * time.Second},
		{"ibc1", "jackzampolin/gaiatest", "jack_relayer-testing", aminoGaiaCdc, gaiaCdc, "26657", 1 * time.Second},
	}

	chains, doneFunc := spinUpTestChains(t, tcs...)
	t.Cleanup(doneFunc)

	src, dst := chains.MustGet(tcs[0].chainID), chains.MustGet(tcs[1].chainID)

	if _, err := genTestPathAndSet(src, dst, xferPort, xferPort); err != nil {
		t.Error(err)
	}

	// Check if clients have been created, if not create them
	if err := src.CreateClients(dst); err != nil {
		t.Error(err)
	}

	// Test querying clients from src and dst sides
	if err := testClientPair(src, dst); err != nil {
		t.Error(err)
	}

	// Check if connection has been created, if not create it
	if err := src.CreateConnection(dst, src.timeout); err != nil {
		t.Error(err)
	}

	// Test querying connections from src and dst sides
	if err := testConnectionPair(src, dst); err != nil {
		t.Error(err)
	}

	// Check if channel has been created, if not create it
	if err := src.CreateChannel(dst, true, src.timeout); err != nil {
		t.Error(err)
	}

	// Test querying channels from src and dst sides
	if err := testChannelPair(src, dst); err != nil {
		t.Error(err)
	}
}
