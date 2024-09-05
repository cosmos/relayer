package cclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosmos/relayer/v2/cclient"
)

func TestGordian(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gc := cclient.NewGordianConsensus("http://127.0.0.1:26657")
	bt, err := gc.GetBlockTime(ctx, 2)
	if err != nil {
		t.Error(err)
	}
	t.Log(bt)
}
