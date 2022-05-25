package processor

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// The ChainProcessor interface is reponsible for polling blocks and emitting IBC message events to the PathProcessors.
// It is also responsible for tracking open channels and not sending messages to the PathProcessors for closed channels.
type ChainProcessor interface {
	// Run is the function that starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
	// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
	// ChainProcessors should obey the context and return upon context cancellation.
	Run(ctx context.Context, initialBlockHistory uint64) error

	// InSync is the function that indicates whether queries are in sync with latest height of the chain.
	// The PathProcessors use this as a signal for determining if the backlog of messaged is ready to be processed and relayed.
	InSync() bool
}

// Run is a blocking call that launches all provided chain processors in parallel.
// It will return once all ChainProcessors have stopped running due to context cancellation,
// or if a critical error has occurred within one of the chain processors.
func Run(ctx context.Context, initialBlockHistory uint64, cp ...ChainProcessor) error {
	var eg errgroup.Group
	runCtx, runCtxCancel := context.WithCancel(ctx)
	for _, chainProcessor := range cp {
		chainProcessor := chainProcessor
		eg.Go(func() error {
			err := chainProcessor.Run(runCtx, initialBlockHistory)
			// Signal the other chain processors to exit.
			runCtxCancel()
			return err
		})
	}
	runCtxCancel()
	return eg.Wait()
}
