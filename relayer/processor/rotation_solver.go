package processor

import (
	"bytes"
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

var errFalsePositive = fmt.Errorf("false positive (there is a bug): hub has latest valset")

// guaranteed to run on same thread as message proccessor
func (s *rotationSolver) solve(ctx c.Context) error {
	/*
		1. Get nextValidatorsHash, height of client state on hub
		2. Binary search rollapp to find change heights
		3. Send updates to hub
	*/
	h, preRotationValhash, err := s.hubClientValset(ctx)
	if err!=nil{
		return fmt.Errorf("hub client valset: %w", err)
	}
	// sanity check to make sure that the rollapp actually has a different val set (confirm there is a problem)
	if bytes.Equal(s.ra.latestHeader.NextValidatorsHash(), preRotationValhash){
		return errFalsePositive
	}
	// we know there's a problem, search to find appropriate update heights


}

func (s *rotationSolver) hubProvider() *cosmos.CosmosProvider {
	return s.hub.chainProvider.(*cosmos.CosmosProvider)
}

func (s *rotationSolver) raProvider() *cosmos.CosmosProvider {
	return s.ra.chainProvider.(*cosmos.CosmosProvider)
}

func (s *rotationSolver) hubClientValset(ctx c.Context) (uint64, []byte, error) {

	h := s.hub.clientState.ConsensusHeight.GetRevisionHeight()
	header, err := s.ra.chainProvider.QueryIBCHeader(ctx, int64(h))
	if err!=nil{
		return 0, nil, fmt.Errorf("query ibc header: %w", err)
	}
	if header.Height()!=h{
		return 0, nil, fmt.Errorf("header height mismatch: got %d, expected %d", header.Height(), h)
	}
	return header.Height(), header.NextValidatorsHash(), nil
}

func (s *rotationSolver) rollappValset(ctx context.Context, h uint64) error {
	s.ra.

}
