package processor

import (
	"bytes"
	c "context"
	"fmt"

	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
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

	rollappHeaders, err := s.rollappHeaders(ctx, h, preRotationValhash)
	if err != nil {
		return fmt.Errorf("rollapp headers: %w", err)
	}

	err = s.sendUpdates(ctx, rollappHeaders[0], rollappHeaders[1])
	if err != nil {
		return fmt.Errorf("send updates: %w", err)
	}
	return nil
}

// a = h, b = h+1
func (s *rotationSolver) sendUpdates(ctx c.Context, a, b provider.IBCHeader) error {
	// here we assume by this code we can reconstruct the trust
	// https://github.com/dymensionxyz/go-relayer/blob/838f324793473de99cbf285f66537580c4158f39/relayer/processor/message_processor.go#L309-L316

	// TODO: this is very sus
	// uses the validator set of the
	u1, err := s.ra.chainProvider.MsgUpdateClientHeader(
		a,
		s.hub.clientState.ConsensusHeight,  // trust height
		s.hub.clientTrustedState.IBCHeader, // trust header. Should be trust height + 1 in theory
	)
	if err != nil {
		return fmt.Errorf("msg update client header: %w", err)
	}

	mu1, err := s.hub.chainProvider.MsgUpdateClient(s.hub.info.ClientID, u1)
	if err != nil {
		return fmt.Errorf("msg update client: %w", err)
	}

	aHeight := clienttypes.Height{
		RevisionNumber: s.hub.clientState.ConsensusHeight.RevisionNumber,
		RevisionHeight: a.Height(),
	}
	s.hub.clientState.ConsensusHeight.RevisionHeight = 4

	u2, err := s.ra.chainProvider.MsgUpdateClientHeader(
		b,
		aHeight,
		b, // use b as a trust basis for itself, pretty sure this should work
	)
	if err != nil {
		return fmt.Errorf("msg update client header: %w", err)
	}

	mu2, err := s.hub.chainProvider.MsgUpdateClient(s.hub.info.ClientID, u2)
	if err != nil {
		return fmt.Errorf("msg update client: %w", err)
	}

	if err := s.broadcastUpdates(ctx, []provider.RelayerMessage{mu1, mu2}); err != nil {
		return fmt.Errorf("broadcast updates: %w", err)
	}
	return nil
}

func (s *rotationSolver) broadcastUpdates(ctx c.Context, msgs []provider.RelayerMessage) error {
	broadcastCtx, cancel := c.WithTimeout(ctx, messageSendTimeout)
	defer cancel()
	return s.hub.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, " ", ctx, nil)
}

func (s *rotationSolver) rollappHeaders(ctx c.Context, h uint64, hValhash []byte) ([]provider.IBCHeader, error) {
	// we know a height h that the hub has with an old valhash, need to find where valhash changes

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
