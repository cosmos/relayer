package substrate_test

import (
	"testing"

	"github.com/cosmos/relayer/v2/relayer/provider/substrate"
)

func initProvider() *substrate.SubstrateProvider {
	provider, err := substrate.NewSubstrateProvider(&substrate.SubstrateProviderConfig{RPCAddr: "127.0.0.1:9944"}, "")
	if err != nil {
		panic(err)
	}
	return provider
}

func TestQueryLatestHeight(t *testing.T) {
	p := initProvider()
	height, err := p.QueryLatestHeight()
	if err != nil {
		panic(err)
	}

	if height <= 0 {
		t.Errorf("latest height should be greater than genesis height")
	}
}
