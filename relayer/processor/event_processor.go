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
	messageLifecycle    MessageLifecycle
}

// EventProcessor is a built instance that is ready to be executed with Run(ctx).
type EventProcessor struct {
	chainProcessors     ChainProcessors
	initialBlockHistory uint64
	pathProcessors      PathProcessors
	messageLifecycle    MessageLifecycle
}

// NewEventProcessor creates a builder than can be used to construct a multi-ChainProcessor, multi-PathProcessor topology for the relayer.
func NewEventProcessor() EventProcessorBuilder {
	return EventProcessorBuilder{}
}

// WithChainProcessors adds to the list of ChainProcessors to be used
// if the chain name has not already been added.
func (ep EventProcessorBuilder) WithChainProcessors(chainProcessors ...ChainProcessor) EventProcessorBuilder {
ChainProcessorLoop:
	for _, cp := range chainProcessors {
		for _, existingCp := range ep.chainProcessors {
			if existingCp.Provider().ChainName() == cp.Provider().ChainName() {
				continue ChainProcessorLoop
			}
		}
		ep.chainProcessors = append(ep.chainProcessors, cp)
	}
	return ep
}

// WithInitialBlockHistory sets the initial block history to query for the ChainProcessors.
func (ep EventProcessorBuilder) WithInitialBlockHistory(initialBlockHistory uint64) EventProcessorBuilder {
	ep.initialBlockHistory = initialBlockHistory
	return ep
}

// WithPathProcessors adds to the list of PathProcessors to be used.
func (ep EventProcessorBuilder) WithPathProcessors(pathProcessors ...*PathProcessor) EventProcessorBuilder {
	ep.pathProcessors = append(ep.pathProcessors, pathProcessors...)
	return ep
}

// WithMessageLifecycle sets the message that should be sent to a chain once in sync,
// and also the message that should stop the EventProcessor once observed.
func (ep EventProcessorBuilder) WithMessageLifecycle(messageLifecycle MessageLifecycle) EventProcessorBuilder {
	ep.messageLifecycle = messageLifecycle
	return ep
}

// Build links the relevant ChainProcessors and PathProcessors, then returns an EventProcessor that can be used to run the ChainProcessors and PathProcessors.
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
	for _, pathProcessor := range ep.pathProcessors {
		pathProcessor.SetMessageLifecycle(ep.messageLifecycle)
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
			pathProcessor.Run(runCtx, runCtxCancel)
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
