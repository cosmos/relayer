package cosmos

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type CosmosChainProcessor struct {
	log *zap.Logger

	ChainProvider *cosmos.CosmosProvider

	// sdk context
	cc *cosmosClient.Context

	pathProcessors processor.PathProcessors

	inSync     bool
	inSyncLock sync.RWMutex
}

func NewCosmosChainProcessor(log *zap.Logger, provider *cosmos.CosmosProvider, rpcAddress string, pathProcessors processor.PathProcessors) (*CosmosChainProcessor, error) {
	cc, err := getCosmosClient(rpcAddress, provider.ChainId())
	if err != nil {
		return nil, fmt.Errorf("error getting cosmos client: %w", err)
	}
	return &CosmosChainProcessor{
		log:            log,
		ChainProvider:  provider,
		cc:             cc,
		pathProcessors: pathProcessors,
	}, nil
}

const (
	latestHeightQueryTimeout    = 5 * time.Second
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration = 1 * time.Second
	inSyncThreshold             = 2 //blocks
)

// InSync indicates whether queries are in sync with latest height of the chain.
// The PathProcessors use this as a signal for determining if the backlog of messaged is ready to be processed and relayed.
func (ccp *CosmosChainProcessor) InSync() bool {
	ccp.inSyncLock.RLock()
	defer ccp.inSyncLock.RUnlock()
	return ccp.inSync
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

// latestHeightWithRetry will query for the latest height, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (ccp *CosmosChainProcessor) latestHeightWithRetry(ctx context.Context) (latestHeight int64, err error) {
	return latestHeight, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, latestHeightQueryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = ccp.ChainProvider.QueryLatestHeight(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		ccp.log.Info(
			"Failed to query latest height",
			zap.String("chain_id", ccp.ChainProvider.ChainId()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

type QueryCyclePersistence struct {
	latestHeight         int64
	latestQueriedBlock   int64
	minQueryLoopDuration time.Duration
}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (ccp *CosmosChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	// this will be used for persistence across query cycle loop executions
	queryCyclePersistence := QueryCyclePersistence{
		minQueryLoopDuration: defaultMinQueryLoopDuration,
	}

	// Infinite retry to get initial latest height
	for {
		latestHeight, err := ccp.latestHeightWithRetry(ctx)
		if err == nil {
			queryCyclePersistence.latestHeight = latestHeight
			break
		}
	}

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlock := queryCyclePersistence.latestHeight - int64(initialBlockHistory)

	if latestQueriedBlock < 0 {
		latestQueriedBlock = 0
	}

	queryCyclePersistence.latestQueriedBlock = latestQueriedBlock

	ccp.log.Info("Entering main query loop", zap.String("chain_id", ccp.ChainProvider.ChainId()))

	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := ccp.queryCycle(ctx, &queryCyclePersistence); err != nil {
			return err
		}
	}
}

func (ccp *CosmosChainProcessor) queryCycle(ctx context.Context, persistence *QueryCyclePersistence) error {
	chainID := ccp.ChainProvider.ChainId()
	cycleTimeStart := time.Now()
	defer ccp.doneWithCycle(cycleTimeStart, persistence.minQueryLoopDuration)

	var err error
	persistence.latestHeight, err = ccp.latestHeightWithRetry(ctx)

	// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
	if err != nil {
		return nil
	}

	ccp.log.Debug("Queried latest height",
		zap.String("chain_id", chainID),
		zap.Int64("latest_height", persistence.latestHeight),
	)

	// Only need read lock here
	ccp.inSyncLock.RLock()
	inSync := ccp.inSync
	ccp.inSyncLock.RUnlock()

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncThreshold {
			// take write lock to update to true (only happens once)
			ccp.inSyncLock.Lock()
			ccp.inSync = true
			ccp.inSyncLock.Unlock()

			firstTimeInSync = true
			ccp.log.Info("chain is in sync", zap.String("chain_id", chainID))
		} else {
			ccp.log.Info("chain is not yet in sync",
				zap.String("chain_id", chainID),
				zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
				zap.Int64("latest_height", persistence.latestHeight),
			)
		}
	}

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		blockRes, err := ccp.cc.Client.BlockResults(ctx, &i)
		if err != nil {
			ccp.log.Error("error getting block results", zap.String("chainID", chainID), zap.Error(err))
			return nil
		}

		for _, tx := range blockRes.TxsResults {
			if tx.Code != 0 {
				// tx was not successful
				continue
			}
			messages := ccp.ibcMessagesFromTransaction(tx)

			ccp.log.Debug("Parsed IBC messages", zap.String("chainID", chainID), zap.Any("messages", messages))

			// TODO pass messages to handlers
		}
	}

	if firstTimeInSync {
		for _, pp := range ccp.pathProcessors {
			pp.ProcessBacklogIfReady()
		}
	}

	return nil
}

func (ccp *CosmosChainProcessor) doneWithCycle(cycleTimeStart time.Time, minQueryLoopDuration time.Duration) {
	queryDuration := time.Since(cycleTimeStart)
	if queryDuration < minQueryLoopDuration {
		time.Sleep(minQueryLoopDuration - queryDuration)
	}
}
