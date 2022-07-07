package cosmos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
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
}

func NewCosmosChainProcessor(log *zap.Logger, provider *cosmos.CosmosProvider) *CosmosChainProcessor {
	return &CosmosChainProcessor{
		log:                  log.With(zap.String("chain_name", provider.ChainName()), zap.String("chain_id", provider.ChainId())),
		chainProvider:        provider,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
	}
}

const (
	queryTimeout                = 5 * time.Second
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration = 1 * time.Second
	inSyncNumBlocksThreshold    = 2
)

type msgHandlerParams struct {
	// incoming IBC message
	messageInfo interface{}

	// reference to the caches that will be assembled by the handlers in this file
	ibcMessagesCache processor.IBCMessagesCache
}

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

	ccp.initializeChannelState(ctx)

	ccp.log.Debug("Entering main query loop")

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

// initializeChannelState will bootstrap the channelStateCache with the open channel state.
func (ccp *CosmosChainProcessor) initializeChannelState(ctx context.Context) error {
	queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
	defer cancelQueryCtx()
	channels, err := ccp.chainProvider.QueryChannels(queryCtx)
	if err != nil {
		return fmt.Errorf("error querying channels: %w", err)
	}
	for _, ch := range channels {
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

	ppChanged := false

	var latestHeader cosmos.CosmosIBCHeader

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var blockRes *ctypes.ResultBlockResults
		var ibcHeader provider.IBCHeader
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
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
			ccp.log.Error("Error querying block data", zap.Error(err))
			return nil
		}

		latestHeader = ibcHeader.(cosmos.CosmosIBCHeader)

		heightUint64 := uint64(i)

		ccp.latestBlock = provider.LatestBlock{
			Height: heightUint64,
			Time:   latestHeader.SignedHeader.Time,
		}

		ibcHeaderCache[heightUint64] = latestHeader
		ppChanged = true

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
	}

	if !ppChanged {
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
			ConnectionStateCache: ccp.connectionStateCache.Clone(),
			ChannelStateCache:    ccp.channelStateCache.Clone(),
			IBCHeaderCache:       ibcHeaderCache,
		})
	}

	persistence.latestQueriedBlock = persistence.latestHeight

	return nil
}
