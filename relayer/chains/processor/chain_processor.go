package processor

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// ChainProcessor interface is reponsible for polling blocks and emitting ibc message events to the PathProcessors
// it is also responsible for tracking open channels and not sending messages to the PathProcessors for closed channels
type ChainProcessor interface {
	// starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors
	Run(ctx context.Context, initialBlockHistory uint64) error

	// are queries in sync with latest height of the chain?
	// path processors use this as a signal for determining if packets should be relayed or not
	InSync() bool
}

// blocking call that launches all chain processors in parallel (main process)
func Run(ctx context.Context, initialBlockHistory uint64, cp ...ChainProcessor) error {
	var eg errgroup.Group
	for _, chainProcessor := range cp {
		chainProcessor := chainProcessor
		eg.Go(func() error { return chainProcessor.Run(ctx, initialBlockHistory) })
	}
	return eg.Wait()
}
