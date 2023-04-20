package archway

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

type ArchwayChainProcessor struct {
	log *zap.Logger

	chainProvider *ArchwayProvider

	pathProcessors processor.PathProcessors

	// indicates whether queries are in sync with latest height of the chain
	inSync bool

	// highest block
	latestBlock provider.LatestBlock

	// holds highest consensus height and header for all clients
	latestClientState

	// holds open state for known connections
	connectionStateCache processor.ConnectionStateCache

	// holds open state for known channels
	channelStateCache processor.ChannelStateCache

	// map of connection ID to client ID
	connectionClients map[string]string

	// map of channel ID to connection ID
	channelConnections map[string]string

	// metrics to monitor lifetime of processor
	metrics *processor.PrometheusMetrics
}

func NewArchwayChainProcessor(log *zap.Logger, provider *ArchwayProvider, metrics *processor.PrometheusMetrics) *ArchwayChainProcessor {
	return &ArchwayChainProcessor{
		log:                  log.With(zap.String("chain_name", provider.ChainName()), zap.String("chain_id", provider.ChainId())),
		chainProvider:        provider,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
		connectionClients:    make(map[string]string),
		channelConnections:   make(map[string]string),
		metrics:              metrics,
	}
}

type latestClientState map[string]provider.ClientState

func (l latestClientState) update(ctx context.Context, clientInfo clientInfo, ccp *ArchwayChainProcessor) {
	// TODO:
}

// Provider returns the ChainProvider, which provides the methods for querying, assembling IBC messages, and sending transactions.
func (ccp *ArchwayChainProcessor) Provider() provider.ChainProvider {
	return ccp.chainProvider
}

// Set the PathProcessors that this ChainProcessor should publish relevant IBC events to.
// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
func (ccp *ArchwayChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	ccp.pathProcessors = pathProcessors
}

func (ccp *ArchwayChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	return nil
}
