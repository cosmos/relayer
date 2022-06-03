package cosmos

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type CosmosChainProcessor struct {
	log *zap.Logger

	pathProcessors processor.PathProcessors
	ChainProvider  *cosmos.CosmosProvider
}

func NewCosmosChainProcessor(log *zap.Logger, provider *cosmos.CosmosProvider, pathProcessors processor.PathProcessors) *CosmosChainProcessor {
	return &CosmosChainProcessor{
		log:            log,
		ChainProvider:  provider,
		pathProcessors: pathProcessors,
	}
}

// InSync indicates whether queries are in sync with latest height of the chain.
// The PathProcessors use this as a signal for determining if the backlog of messaged is ready to be processed and relayed.
func (ccp *CosmosChainProcessor) InSync() bool {
	return false
}

// ChainID returns the identifier of the chain
func (ccp *CosmosChainProcessor) ChainID() string {
	return ccp.ChainProvider.ChainId()
}

// Set the PathProcessors that this ChainProcessor should publish relevant IBC events to.
// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
func (ccp *CosmosChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	ccp.pathProcessors = pathProcessors
}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (ccp *CosmosChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	panic("not yet implemented")
}
