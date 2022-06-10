package processor

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

// The ChainProcessor interface is reponsible for polling blocks and emitting IBC message events to the PathProcessors.
// It is also responsible for tracking open channels and not sending messages to the PathProcessors for closed channels.
type ChainProcessor interface {
	// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
	// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
	// ChainProcessors should obey the context and return upon context cancellation.
	Run(ctx context.Context, initialBlockHistory uint64) error

	// Provider returns the ChainProvider, which provides the methods for querying, assembling IBC messages, and sending transactions.
	Provider() provider.ChainProvider

	// Set the PathProcessors that this ChainProcessor should publish relevant IBC events to.
	// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
	SetPathProcessors(pathProcessors PathProcessors)
}

// ChainProcessors is a slice of ChainProcessor instances.
type ChainProcessors []ChainProcessor
