package processor

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// EventProcessorBuilder is a configuration type with .With functions used for building an EventProcessor.
type EventProcessorBuilder struct {
	chainProcessors     ChainProcessors
	initialBlockHistory uint64
	pathProcessors      PathProcessors
}

// EventProcessor is a built instance that is ready to be executed with Run(ctx).
type EventProcessor struct {
	chainProcessors     ChainProcessors
	initialBlockHistory uint64
	pathProcessors      PathProcessors
}

// NewEventProcessor creates a builder than can be used to construct a multi-ChainProcessor, multi-PathProcessor topology for the relayer.
func NewEventProcessor() EventProcessorBuilder {
	return EventProcessorBuilder{}
}

// WithChainProcessors will set the ChainProcessors to be used.
func (ep EventProcessorBuilder) WithChainProcessors(chainProcessors ...ChainProcessor) EventProcessorBuilder {
	ep.chainProcessors = chainProcessors
	return ep
}

// WithInitialBlockHistory will set the initial block history to query for the ChainProcessors.
func (ep EventProcessorBuilder) WithInitialBlockHistory(initialBlockHistory uint64) EventProcessorBuilder {
	ep.initialBlockHistory = initialBlockHistory
	return ep
}

// WithPathProcessors will set the PathProcessors to be used.
func (ep EventProcessorBuilder) WithPathProcessors(pathProcessors ...*PathProcessor) EventProcessorBuilder {
	ep.pathProcessors = pathProcessors
	return ep
}

// Build will link the relevant ChainProcessors and PathProcessors, then return an EventProcessor that can be used to run the ChainProcessors and PathProcessors.
func (ep EventProcessorBuilder) Build() EventProcessor {
	for _, chainProcessor := range ep.chainProcessors {
		pathProcessorsForThisChain := PathProcessors{}
		for _, pathProcessor := range ep.pathProcessors {
			if pathProcessor.SetChainProviderIfApplicable(chainProcessor.Provider()) {
				pathProcessorsForThisChain = append(pathProcessorsForThisChain, pathProcessor)
			}
		}
		chainProcessor.SetPathProcessors(pathProcessorsForThisChain)
	}

	return EventProcessor(ep)
}

// Run is a blocking call that launches all provided PathProcessors and ChainProcessors in parallel.
// It will return once all PathProcessors and ChainProcessors have stopped running due to context cancellation,
// or if a critical error has occurred within one of the ChainProcessors.
func (ep EventProcessor) Run(ctx context.Context) error {
	var eg errgroup.Group
	runCtx, runCtxCancel := context.WithCancel(ctx)
	for _, pathProcessor := range ep.pathProcessors {
		pathProcessor := pathProcessor
		eg.Go(func() error {
			pathProcessor.Run(runCtx)
			return nil
		})
	}
	for _, chainProcessor := range ep.chainProcessors {
		chainProcessor := chainProcessor
		eg.Go(func() error {
			err := chainProcessor.Run(runCtx, ep.initialBlockHistory)
			// Signal the other chain processors to exit.
			runCtxCancel()
			return err
		})
	}
	err := eg.Wait()
	runCtxCancel()
	return err
}
