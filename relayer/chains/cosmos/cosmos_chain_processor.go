package cosmos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type CosmosChainProcessor struct {
	log *zap.Logger

	chainProvider *cosmos.CosmosProvider

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

func NewCosmosChainProcessor(log *zap.Logger, provider *cosmos.CosmosProvider) *CosmosChainProcessor {
	return &CosmosChainProcessor{
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
	// Timeout for initialization queries and light block query,
	queryTimeout = 10 * time.Second

	// Timeout for block results query. Longer timeout needed for large blocks.
	blockResultsQueryTimeout = 2 * time.Minute

	// Back off delay between latest height query retries
	latestHeightQueryRetryDelay = 1 * time.Second

	// Max retries for failed latest height queries
	latestHeightQueryRetries = 5

	// Initial query loop minimum duration.
	// Rolling average block time will be used after startup.
	defaultMinQueryLoopDuration = 1 * time.Second

	// How many blocks behind the latest to consider in sync with the chain.
	inSyncNumBlocksThreshold = 2

	// Fixed delay between block query retry attempts.
	blockQueryRetryDelay = 200 * time.Millisecond

	// With doubling the 10 second queryTimeout each time, this gives ~3 mins max timeout.
	blockQueryRetries = 4

	// Each new delta block time will affect the rolling average by 1/n of this.
	blockAverageSmoothing = 10

	// Delta block times will be excluded from rolling average if they exceed this.
	maxConsideredDeltaBlockTimeMs = 15000

	// Scenarios for how much clock drift should be added to
	// target ideal block query window.

	// Clock drift addition when a block query fails
	queryFailureClockDriftAdditionMs = 47

	// Clock drift addition when the block queries succeeds
	querySuccessClockDriftAdditionMs = -23

	// Clock drift addition when the latest block is the same as
	// the last successfully queried block
	sameBlockClockDriftAdditionMs = 71
)

// latestClientState is a map of clientID to the latest clientInfo for that client.
type latestClientState map[string]provider.ClientState

func (l latestClientState) update(clientInfo clientInfo) {
	existingClientInfo, ok := l[clientInfo.clientID]
	if ok && clientInfo.consensusHeight.LT(existingClientInfo.ConsensusHeight) {
		// height is less than latest, so no-op
		return
	}

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientInfo.ClientState()
}

// Provider returns the ChainProvider, which provides the methods for querying, assembling IBC messages, and sending transactions.
func (ccp *CosmosChainProcessor) Provider() provider.ChainProvider {
	return ccp.chainProvider
}

// Set the PathProcessors that this ChainProcessor should publish relevant IBC events to.
// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
func (ccp *CosmosChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	ccp.pathProcessors = pathProcessors
}

// latestHeightWithRetry will query for the latest height, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (ccp *CosmosChainProcessor) latestHeightWithRetry(ctx context.Context) (latestHeight int64, err error) {
	return latestHeight, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, queryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = ccp.chainProvider.QueryLatestHeight(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		ccp.log.Info(
			"Failed to query latest height",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (ccp *CosmosChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	if state, ok := ccp.latestClientState[clientID]; ok {
		return state, nil
	}
	cs, err := ccp.chainProvider.QueryClientState(ctx, int64(ccp.latestBlock.Height), clientID)
	if err != nil {
		return provider.ClientState{}, err
	}
	clientState := provider.ClientState{
		ClientID:        clientID,
		ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
	}
	ccp.latestClientState[clientID] = clientState
	return clientState, nil
}

// queryCyclePersistence hold the variables that should be retained across queryCycles.
type queryCyclePersistence struct {
	latestHeight           int64
	latestQueriedBlock     int64
	latestQueriedBlockTime time.Time
	averageBlockTimeMs     int64
	minQueryLoopDuration   time.Duration
	clockDriftMs           int64
}

// addClockDriftMs is used to modify the clock drift for targeting the
// next ideal window for block queries. For example if a block query fails
// because it is too early to be queried, clock drift should be added. If the query
// succeeds, clock drift should be removed, but not as much as is added for the error
// case. Clock drift should also be added when checking the latest height and it has
// not yet incremented.
func (p *queryCyclePersistence) addClockDriftMs(ms int64) {
	p.clockDriftMs += ms
	if p.clockDriftMs < 0 {
		p.clockDriftMs = 0
	} else if p.clockDriftMs > p.averageBlockTimeMs {
		// Should not add more than the average block time as a delay
		// when targeting the next available block query.
		p.clockDriftMs = p.averageBlockTimeMs
	}
}

// dynamicBlockTime targets the next ideal window for a block query to attempt
// to schedule block queries for the exact time when they are available.
func (p *queryCyclePersistence) dynamicBlockTime(
	log *zap.Logger,
	queryStart time.Time,
	latestBlockTime time.Time,
) {
	var deltaBlockTime int64

	if p.latestQueriedBlockTime.IsZero() {
		// latestQueriedBlockTime not yet initialized
		return
	}

	// deltaT between previous block time and latest block time.
	deltaBlockTime = latestBlockTime.Sub(p.latestQueriedBlockTime).Milliseconds()

	if deltaBlockTime > maxConsideredDeltaBlockTimeMs {
		// treat halts and upgrades as outliers
		return
	}
	if p.averageBlockTimeMs == 0 {
		// initialize average block time with first measurement of deltaT
		p.averageBlockTimeMs = deltaBlockTime
	} else {
		// compute rolling average of deltaT
		weightedComponent := p.averageBlockTimeMs * (blockAverageSmoothing - 1)
		p.averageBlockTimeMs = int64(float64(weightedComponent+deltaBlockTime) / blockAverageSmoothing)
	}

	// calculate deltaT between the block timestamp and when we initiated the query.
	timeQueriedAfterBlockTime := queryStart.Sub(latestBlockTime).Milliseconds()
	if timeQueriedAfterBlockTime <= 0 {
		log.Debug("Unexpected state, query start is before latest block time but query succeeded")
		return
	}

	queryDurationMs := time.Since(queryStart).Milliseconds()

	// also take into account older blocks, where timeQueriedAfterBlockTime > p.averageBlockTimeMs, by using remainder.
	// clock drift tolerant using clockDriftMs trim value
	targetedQueryTimeFromNow := p.averageBlockTimeMs - (timeQueriedAfterBlockTime % p.averageBlockTimeMs) -
		queryDurationMs + p.clockDriftMs

	p.minQueryLoopDuration = time.Millisecond * time.Duration(targetedQueryTimeFromNow)

	log.Debug("Dynamic query time",
		zap.Int64("avg_block_ms", p.averageBlockTimeMs),
		zap.Int64("targeted_query_ms", targetedQueryTimeFromNow),
		zap.Int64("clock_drift_ms", p.clockDriftMs),
	)
}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (ccp *CosmosChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	// this will be used for persistence across query cycle loop executions
	persistence := queryCyclePersistence{
		minQueryLoopDuration: defaultMinQueryLoopDuration,
	}

	// Infinite retry to get initial latest height
	for {
		latestHeight, err := ccp.latestHeightWithRetry(ctx)
		if err != nil {
			ccp.log.Error(
				"Failed to query latest height after max attempts",
				zap.Uint("attempts", latestHeightQueryRetries),
				zap.Error(err),
			)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			continue
		}
		persistence.latestHeight = latestHeight
		break
	}

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlock := persistence.latestHeight - int64(initialBlockHistory)

	if latestQueriedBlock < 0 {
		latestQueriedBlock = 0
	}

	persistence.latestQueriedBlock = latestQueriedBlock

	var eg errgroup.Group
	eg.Go(func() error {
		return ccp.initializeConnectionState(ctx)
	})
	eg.Go(func() error {
		return ccp.initializeChannelState(ctx)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	ccp.log.Debug("Entering main query loop")

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(persistence.minQueryLoopDuration):
			if err := ccp.queryCycle(ctx, &persistence); err != nil {
				return err
			}
		}
	}
}

// initializeConnectionState will bootstrap the connectionStateCache with the open connection state.
func (ccp *CosmosChainProcessor) initializeConnectionState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	connections, err := ccp.chainProvider.QueryConnections(ctx)
	if err != nil {
		return fmt.Errorf("error querying connections: %w", err)
	}
	for _, c := range connections {
		ccp.connectionClients[c.Id] = c.ClientId
		ccp.connectionStateCache[processor.ConnectionKey{
			ConnectionID:         c.Id,
			ClientID:             c.ClientId,
			CounterpartyConnID:   c.Counterparty.ConnectionId,
			CounterpartyClientID: c.Counterparty.ClientId,
		}] = c.State == conntypes.OPEN
	}
	return nil
}

// initializeChannelState will bootstrap the channelStateCache with the open channel state.
func (ccp *CosmosChainProcessor) initializeChannelState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	channels, err := ccp.chainProvider.QueryChannels(ctx)
	if err != nil {
		return fmt.Errorf("error querying channels: %w", err)
	}
	for _, ch := range channels {
		if len(ch.ConnectionHops) != 1 {
			ccp.log.Error("Found channel using multiple connection hops. Not currently supported, ignoring.",
				zap.String("channel_id", ch.ChannelId),
				zap.String("port_id", ch.PortId),
				zap.Strings("connection_hops", ch.ConnectionHops),
			)
			continue
		}
		ccp.channelConnections[ch.ChannelId] = ch.ConnectionHops[0]
		ccp.channelStateCache[processor.ChannelKey{
			ChannelID:             ch.ChannelId,
			PortID:                ch.PortId,
			CounterpartyChannelID: ch.Counterparty.ChannelId,
			CounterpartyPortID:    ch.Counterparty.PortId,
		}] = ch.State == chantypes.OPEN
	}
	return nil
}

func (ccp *CosmosChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) error {
	var err error
	persistence.latestHeight, err = ccp.latestHeightWithRetry(ctx)

	// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
	if err != nil {
		ccp.log.Error(
			"Failed to query latest height after max attempts",
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	ccp.log.Debug("Queried latest height",
		zap.Int64("latest_height", persistence.latestHeight),
	)

	if persistence.latestHeight == persistence.latestQueriedBlock {
		persistence.addClockDriftMs(sameBlockClockDriftAdditionMs)
		persistence.minQueryLoopDuration += 100 * time.Millisecond
		return nil
	}

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !ccp.inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncNumBlocksThreshold {
			ccp.inSync = true
			firstTimeInSync = true
			ccp.log.Info("Chain is in sync")
		} else {
			ccp.log.Info("Chain is not yet in sync",
				zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
				zap.Int64("latest_height", persistence.latestHeight),
			)
		}
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ibcHeaderCache := make(processor.IBCHeaderCache)

	var latestHeader cosmos.CosmosIBCHeader

	newLatestQueriedBlock := persistence.latestQueriedBlock

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var blockRes *ctypes.ResultBlockResults
		var ibcHeader provider.IBCHeader
		i := i
		queryStartTime := time.Now()
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, blockResultsQueryTimeout)
			defer cancelQueryCtx()
			blockRes, err = ccp.chainProvider.RPCClient.BlockResults(queryCtx, &i)
			return err
		})
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()
			ibcHeader, err = ccp.chainProvider.IBCHeaderAtHeight(queryCtx, i)
			return err
		})

		if err := eg.Wait(); err != nil {
			persistence.addClockDriftMs(queryFailureClockDriftAdditionMs)
			ccp.log.Warn("Error querying block data", zap.Error(err))
			break
		}
		persistence.addClockDriftMs(querySuccessClockDriftAdditionMs)

		latestHeader = ibcHeader.(cosmos.CosmosIBCHeader)

		heightUint64 := uint64(i)

		latestBlockTime := latestHeader.SignedHeader.Time

		ccp.latestBlock = provider.LatestBlock{
			Height: heightUint64,
			Time:   latestBlockTime,
		}

		ibcHeaderCache[heightUint64] = latestHeader

		for _, tx := range blockRes.TxsResults {
			if tx.Code != 0 {
				// tx was not successful
				continue
			}
			messages := ccp.ibcMessagesFromTransaction(tx, heightUint64)

			for _, m := range messages {
				ccp.handleMessage(m, ibcMessagesCache)
			}
		}
		newLatestQueriedBlock = i

		persistence.dynamicBlockTime(ccp.log, queryStartTime, latestBlockTime)
		persistence.latestQueriedBlockTime = latestBlockTime
	}

	if newLatestQueriedBlock == persistence.latestQueriedBlock {
		if firstTimeInSync {
			for _, pp := range ccp.pathProcessors {
				pp.ProcessBacklogIfReady()
			}
		}

		return nil
	}

	chainID := ccp.chainProvider.ChainId()

	for _, pp := range ccp.pathProcessors {
		clientID := pp.RelevantClientID(chainID)
		clientState, err := ccp.clientState(ctx, clientID)
		if err != nil {
			ccp.log.Error("Error fetching client state",
				zap.String("client_id", clientID),
				zap.Error(err),
			)
			continue
		}

		pp.HandleNewData(chainID, processor.ChainProcessorCacheData{
			LatestBlock:          ccp.latestBlock,
			LatestHeader:         latestHeader,
			IBCMessagesCache:     ibcMessagesCache,
			InSync:               ccp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: ccp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    ccp.channelStateCache.FilterForClient(clientID, ccp.channelConnections, ccp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache,
		})
	}

	persistence.latestQueriedBlock = newLatestQueriedBlock

	return nil
}
