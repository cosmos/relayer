package cosmos

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type CosmosChainProcessor struct {
	log *zap.Logger

	chainProvider *cosmos.CosmosProvider

	// sdk context
	cc *cosmosClient.Context

	pathProcessors processor.PathProcessors

	// indicates whether queries are in sync with latest height of the chain
	inSync bool
}

func NewCosmosChainProcessor(log *zap.Logger, provider *cosmos.CosmosProvider, rpcAddress string, pathProcessors processor.PathProcessors) (*CosmosChainProcessor, error) {
	cc, err := getCosmosClient(rpcAddress, provider.ChainId())
	if err != nil {
		return nil, fmt.Errorf("error getting cosmos client: %w", err)
	}
	return &CosmosChainProcessor{
		log:            log,
		chainProvider:  provider,
		cc:             cc,
		pathProcessors: pathProcessors,
	}, nil
}

const (
	latestHeightQueryTimeout    = 5 * time.Second
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration = 1 * time.Second
	inSyncNumBlocksThreshold    = 2
)

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
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, latestHeightQueryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = ccp.chainProvider.QueryLatestHeight(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		ccp.log.Info(
			"Failed to query latest height",
			zap.String("chain_id", ccp.chainProvider.ChainId()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

type queryCyclePersistence struct {
	latestHeight         int64
	latestQueriedBlock   int64
	minQueryLoopDuration time.Duration
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
				zap.String("chain_id", ccp.chainProvider.ChainId()),
				zap.Uint("attempts", latestHeightQueryRetries),
				zap.Error(err),
			)
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

	ccp.log.Info("Entering main query loop", zap.String("chain_id", ccp.chainProvider.ChainId()))

	ticker := time.NewTicker(persistence.minQueryLoopDuration)

	for {
		if err := ccp.queryCycle(ctx, &persistence); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			ticker.Reset(persistence.minQueryLoopDuration)
		}
	}
}

func (ccp *CosmosChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) error {
	chainID := ccp.chainProvider.ChainId()

	var err error
	persistence.latestHeight, err = ccp.latestHeightWithRetry(ctx)

	// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
	if err != nil {
		ccp.log.Error(
			"Failed to query latest height after max attempts",
			zap.String("chain_id", ccp.chainProvider.ChainId()),
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	ccp.log.Debug("Queried latest height",
		zap.String("chain_id", chainID),
		zap.Int64("latest_height", persistence.latestHeight),
	)

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !ccp.inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncNumBlocksThreshold {
			ccp.inSync = true
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
