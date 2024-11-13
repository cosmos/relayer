package processor

import (
	c "context"
	"fmt"

	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	tmclient "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type rotationSolver struct {
	hub *pathEndRuntime
	ra  *pathEndRuntime
}

func (s *rotationSolver) solve(ctx c.Context) error {
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

func (s *rotationSolver) hubClientValset(ctx c.Context) error {

	s.hub.latestHeader
	clientStateRes, err := s.hubProvider().QueryClientStateResponse(ctx, srch, srcClientId)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	clientState, ok := clientStateExported.(*tmclient.ClientState)
	if !ok {
		return &tmclient.ClientState{},
			fmt.Errorf("when casting exported clientstate to tendermint type, got(%T)", clientStateExported)
	}

}

func (s *rotationSolver) rollappValset(ctx context.Context, h uint64) error {
	s.ra.

}
