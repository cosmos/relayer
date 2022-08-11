package substrate

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

type SubstrateChainProcessor struct {
	log *zap.Logger

	chainProvider *SubstrateProvider

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
}

func NewSubstrateChainProcessor(log *zap.Logger, provider *SubstrateProvider) *SubstrateChainProcessor {
	return &SubstrateChainProcessor{
		log:                  log.With(zap.String("chain_name", provider.ChainName()), zap.String("chain_id", provider.ChainId())),
		chainProvider:        provider,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
		connectionClients:    make(map[string]string),
		channelConnections:   make(map[string]string),
	}
}

const (
	queryTimeout                = 5 * time.Second
	blockResultsQueryTimeout    = 2 * time.Minute
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration = 1 * time.Second
	inSyncNumBlocksThreshold    = 2
)

// latestClientState is a map of clientID to the latest clientInfo for that client.
type latestClientState map[string]provider.ClientState

func (l latestClientState) update(clientInfo clientInfo) {}

// Provider returns the ChainProvider, which provides the methods for querying, assembling IBC messages, and sending transactions.
func (scp *SubstrateChainProcessor) Provider() provider.ChainProvider {
	return scp.chainProvider
}

// Set the PathProcessors that this ChainProcessor should publish relevant IBC events to.
// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
func (scp *SubstrateChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	scp.pathProcessors = pathProcessors
}

// latestHeightWithRetry will query for the latest height, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (scp *SubstrateChainProcessor) latestHeightWithRetry(ctx context.Context) (latestHeight int64, err error) {
	return latestHeight, retry.Do(func() error {
		return nil
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
	}))
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (scp *SubstrateChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	return provider.ClientState{}, nil
}

// queryCyclePersistence hold the variables that should be retained across queryCycles.
type queryCyclePersistence struct{}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (scp *SubstrateChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	return nil
}

// initializeConnectionState will bootstrap the connectionStateCache with the open connection state.
func (scp *SubstrateChainProcessor) initializeConnectionState(ctx context.Context) error {
	return nil
}

// initializeChannelState will bootstrap the channelStateCache with the open channel state.
func (scp *SubstrateChainProcessor) initializeChannelState(ctx context.Context) error {
	return nil
}

func (scp *SubstrateChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) error {
	return nil
}
