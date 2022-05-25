package processor

import (
	"context"
)

// ChainProcessor interface is reponsible for polling blocks and emitting ibc message events to the PathProcessors
// it is also responsible for tracking open channels and not sending messages to the PathProcessors for closed channels
type ChainProcessor interface {
	// starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors
	Start(ctx context.Context, initialBlockHistory uint64, errCh chan<- error)

	// are queries in sync with latest height of the chain?
	// path processors use this as a signal for determining if packets should be relayed or not
	InSync() bool
}

// blocking call that launches all chain processors in parallel (main process)
func Start(ctx context.Context, initialBlockHistory uint64, errCh chan<- error, cp ...ChainProcessor) {
	for _, chainProcessor := range cp {
		go chainProcessor.Start(ctx, initialBlockHistory, errCh)
	}
	<-ctx.Done()
}
