package processor

import (
	"bytes"
	c "context"
	"fmt"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

type rotationSolver struct {
	hub *pathEndRuntime
	ra  *pathEndRuntime
	log *zap.Logger
}

var (
	errFalsePositive  = fmt.Errorf("false positive (there is a bug): hub has latest valset")
	errTargetNotFound = fmt.Errorf("target not found")
)

// must be sure to run on same thread as message processor
func (s *rotationSolver) solve(ctx c.Context) error {
	/*
		1. Get nextValidatorsHash, height of client state on hub
		2. Binary search rollapp to find valset change heights
		3. Send updates to hub
	*/

	h, preRotationValhash, err := s.hubClientValset(ctx)
	if err != nil {
		return fmt.Errorf("get initial hub client valset: %w", err)
	}

	// sanity check to make sure that the rollapp actually has a different val set (confirm there is a problem)
	if bytes.Equal(s.ra.latestHeader.NextValidatorsHash(), preRotationValhash) {
		return errFalsePositive
	}

	rollappHeaders, err := s.rollappHeaders(ctx, h, preRotationValhash)
	if err != nil {
		return fmt.Errorf("search for two rollapp headers: %w", err)
	}

	err = s.sendUpdatesV2(ctx, rollappHeaders[0], rollappHeaders[1])
	if err != nil {
		return fmt.Errorf("send the two headers in updates: %w", err)
	}
	return nil
}

func (s *rotationSolver) hubClientValset(ctx c.Context) (uint64, []byte, error) {
	h := s.hub.clientState.LatestHeight.GetRevisionHeight()
	header, err := s.ra.chainProvider.QueryIBCHeader(ctx, int64(h))
	if err != nil {
		return 0, nil, fmt.Errorf("query ibc header: %w", err)
	}
	if header.Height() != h {
		return 0, nil, fmt.Errorf("header height mismatch: got %d, expected %d", header.Height(), h)
	}
	return header.Height(), header.NextValidatorsHash(), nil
}

// finds the two headers where it changes from hValhash to a different one
func (s *rotationSolver) rollappHeaders(ctx c.Context, hHub uint64, hubValHash []byte) ([]provider.IBCHeader, error) {
	// we know a height h that the hub has with an old valhash, need to find where valhash changes

	check := func(ansCandidate uint64) (int, error) {
		// Contract: return 0 if it's the FIRST header with a different nextValidatorsHash
		// In other words: the last header with the same (curr) val hash

		hQ := int64(ansCandidate) - 1
		headerSub1, err := s.ra.chainProvider.QueryIBCHeader(ctx, hQ)
		if err != nil {
			return 0, fmt.Errorf("query ibc header candidate h sub 1: %d: %w", hQ, err)
		}
		if !bytes.Equal(headerSub1.NextValidatorsHash(), hubValHash) {
			// too high
			return -1, nil
		}
		hQ = int64(ansCandidate)
		header, err := s.ra.chainProvider.QueryIBCHeader(ctx, hQ)
		if err != nil {
			return 0, fmt.Errorf("query ibc header candidate h: %d: %w", hQ, err)
		}
		if !bytes.Equal(header.NextValidatorsHash(), hubValHash) {
			// perfect: this is the FIRST header with a different nextValidatorsHash
			return 0, nil
		}
		// too low
		return 1, nil
	}

	// ans will be the first height on the hub where nextValidatorsHash changes
	ans, err := search(time.Millisecond*50, hHub, s.ra.latestHeader.Height(), check)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// last header produced by old sequencer
	a, err := s.ra.chainProvider.QueryIBCHeader(ctx, int64(ans))
	if err != nil {
		return nil, fmt.Errorf("query ibc header a : h: %d: %w", ans, err)
	}

	// first header produced by new sequencer
	b, err := s.ra.chainProvider.QueryIBCHeader(ctx, int64(ans+1))
	if err != nil {
		return nil, fmt.Errorf("query ibc header b : h: %d: %w", ans+1, err)
	}

	return []provider.IBCHeader{a, b}, nil
}

// search in [l, r]
// optional sleep: nasty hack to avoid spamming the rpc
func search(sleep time.Duration, l, r uint64, direction func(uint64) (int, error)) (uint64, error) {
	for l < r {
		m := (l + r) / 2
		d, err := direction(m)
		if err != nil {
			return 0, fmt.Errorf("direction: l: %d, m: %d, r: %d, : %w", l, m, r, err)
		}
		if d < 0 {
			r = m - 1 // TODO: check
		}
		if d == 0 {
			return m, nil
		}
		if 0 < d {
			l = m + 1 // TODO: check
		}
		time.Sleep(sleep)
	}
	return 0, fmt.Errorf("%w: l: %d, r: %d, ", errTargetNotFound, l, r)
}

// a = h, b = h+1 where valhash changes in between
func (s *rotationSolver) sendUpdatesV2(ctx c.Context, a, b provider.IBCHeader) error {
	trusted, err := s.ra.chainProvider.QueryIBCHeader(ctx, int64(s.hub.clientState.LatestHeight.GetRevisionHeight())+1)
	if err != nil {
		return fmt.Errorf("query ibc header: %w", err)
	}

	u1, err := s.ra.chainProvider.MsgUpdateClientHeader(
		a,
		s.hub.clientState.LatestHeight,
		trusted, // latest+1
	)
	if err != nil {
		return fmt.Errorf("create msg update client header: %w", err)
	}

	mu1, err := s.hub.chainProvider.MsgUpdateClient(s.hub.info.ClientID, u1)
	if err != nil {
		return fmt.Errorf("create msg update client: update one %w", err)
	}

	aHeight := clienttypes.Height{
		RevisionNumber: s.hub.clientState.LatestHeight.RevisionNumber,
		RevisionHeight: a.Height(),
	}

	u2, err := s.ra.chainProvider.MsgUpdateClientHeader(
		b,       // header to send in update
		aHeight, // trusted height is now the height of the first header
		b,       // use b trust validators aHeight
	)
	if err != nil {
		return fmt.Errorf("create msg update client header: %w", err)
	}

	mu2, err := s.hub.chainProvider.MsgUpdateClient(s.hub.info.ClientID, u2)
	if err != nil {
		return fmt.Errorf("create msg update client: update two : %w", err)
	}

	if err := s.broadcastUpdates(ctx, []provider.RelayerMessage{mu1, mu2}); err != nil {
		return fmt.Errorf("broadcast both updates: %w", err)
	}
	return nil
}

func (s *rotationSolver) broadcastUpdates(ctx c.Context, msgs []provider.RelayerMessage) error {
	broadcastCtx, cancel := c.WithTimeout(ctx, messageSendTimeout)
	defer cancel()
	cbs := make([]func(*provider.RelayerTxResponse, error), 0)
	cbs = append(cbs, func(r *provider.RelayerTxResponse, err error) {
		ok := true
		if err != nil {
			s.log.Error("Broadcast rotation solver update", zap.Error(err))
			ok = false
		}
		if r.Code != 0 {
			s.log.Error("Broadcast rotation solver update", zap.Any("code", r.Code))
			ok = false
		}
		if ok {
			s.log.Info("Rotation solver TX accepted.")
		}
	})
	return s.hub.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, " ", ctx, cbs)
}
