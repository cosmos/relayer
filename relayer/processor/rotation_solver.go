package processor

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
)

type rotationSolver struct {
	hub *pathEndRuntime
	ra  *pathEndRuntime
}

func (s *rotationSolver) solve() error {
	/*
		1. Get nextValidatorsHash, height of client state on hub
		2. Binary search rollapp to find change heights
		3. Send updates to hub
	*/

}

func (s *rotationSolver) hubProvider() *cosmos.CosmosProvider {
	return s.hub.chainProvider.(*cosmos.CosmosProvider)
}

func (s *rotationSolver) raProvider() *cosmos.CosmosProvider {
	return s.ra.chainProvider.(*cosmos.CosmosProvider)
}

func (s *rotationSolver) hubClientValset() error {

}

func (s *rotationSolver) rollappValset(ctx context.Context, h uint64) error {
	s.ra.

}
