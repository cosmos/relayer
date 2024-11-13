package processor

import (
	"bytes"
	c "context"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type rotationSolver struct {
	hub *pathEndRuntime
	ra  *pathEndRuntime
}

func (s *rotationSolver) hubProvider() *cosmos.CosmosProvider {
	return s.hub.chainProvider.(*cosmos.CosmosProvider)
}

func (s *rotationSolver) raProvider() *cosmos.CosmosProvider {
	return s.ra.chainProvider.(*cosmos.CosmosProvider)
}

var errFalsePositive = fmt.Errorf("false positive (there is a bug): hub has latest valset")

// guaranteed to run on same thread as message processor
func (s *rotationSolver) solve(ctx c.Context) error {
	/*
		1. Get nextValidatorsHash, height of client state on hub
		2. Binary search rollapp to find change heights
		3. Send updates to hub
	*/
	h, preRotationValhash, err := s.hubClientValset(ctx)
	if err != nil {
		return fmt.Errorf("hub client valset: %w", err)
	}
	// sanity check to make sure that the rollapp actually has a different val set (confirm there is a problem)
	if bytes.Equal(s.ra.latestHeader.NextValidatorsHash(), preRotationValhash) {
		return errFalsePositive
	}
	// we know there's a problem, search to find appropriate update heights

	// NOTE: VERY IMPORTANT - TRUSTED HEIGHT IS PRIOR TO TRUSTED HEADER
	// TODO:

	// now send the updates

	rollappHeaders, err := s.rollappHeaders(ctx, h)
	if err != nil {
		return fmt.Errorf("rollapp headers: %w", err)
	}

	err = s.sendUpdates(ctx, rollappHeaders[0], rollappHeaders[1])
	if err != nil {
		return fmt.Errorf("send updates: %w", err)
	}

}

// a = h, b = h+1
func (s *rotationSolver) sendUpdates(ctx c.Context, a, b provider.IBCHeader) error {
	// here we assume by this code we can reconstruct the trust
	// https://github.com/dymensionxyz/go-relayer/blob/838f324793473de99cbf285f66537580c4158f39/relayer/processor/message_processor.go#L309-L316

	u1, err := s.ra.chainProvider.MsgUpdateClientHeader(
		a,
		s.hub.clientState.ConsensusHeight,
		s.hub.clientTrustedState.IBCHeader,
	)
	if err != nil {
		return fmt.Errorf("msg update client header: %w", err)
	}

	mu1, err := s.hub.chainProvider.MsgUpdateClient(s.hub.info.ClientID, u1)
	if err != nil {
		return fmt.Errorf("msg update client: %w", err)
	}

	u2, err := s.ra.chainProvider.MsgUpdateClientHeader(
		a,
		s.hub.clientState.ConsensusHeight,
		s.hub.clientTrustedState.IBCHeader,
	)
	if err != nil {
		return fmt.Errorf("msg update client header: %w", err)
	}
}

func (s *rotationSolver) rollappHeaders(ctx c.Context, h uint64) ([]provider.IBCHeader, error) {

}

func (s *rotationSolver) hubClientValset(ctx c.Context) (uint64, []byte, error) {

	h := s.hub.clientState.ConsensusHeight.GetRevisionHeight()
	header, err := s.ra.chainProvider.QueryIBCHeader(ctx, int64(h))
	if err != nil {
		return 0, nil, fmt.Errorf("query ibc header: %w", err)
	}
	if header.Height() != h {
		return 0, nil, fmt.Errorf("header height mismatch: got %d, expected %d", header.Height(), h)
	}
	return header.Height(), header.NextValidatorsHash(), nil
}
