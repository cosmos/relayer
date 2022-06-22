package cosmos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/avast/retry-go/v4"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	provtypes "github.com/tendermint/tendermint/light/provider"
	prov "github.com/tendermint/tendermint/light/provider/http"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type CosmosChainProcessor struct {
	log *zap.Logger

	chainProvider *cosmos.CosmosProvider

	// sdk context
	cc *cosmosClient.Context

	// for fetching light blocks from tendermint
	lightProvider provtypes.Provider

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

func NewCosmosChainProcessor(log *zap.Logger, provider *cosmos.CosmosProvider, rpcAddress string, input io.Reader, output io.Writer, pathProcessors processor.PathProcessors) (*CosmosChainProcessor, error) {
	chainID := provider.ChainId()
	cc, err := getCosmosClient(rpcAddress, chainID, input, output)
	if err != nil {
		return nil, fmt.Errorf("error getting cosmos client: %w", err)
	}
	lightProvider, err := prov.New(chainID, rpcAddress)
	if err != nil {
		return nil, fmt.Errorf("error getting light provider: %w", err)
	}
	return &CosmosChainProcessor{
		log:                  log.With(zap.String("chain_name", provider.ChainName()), zap.String("chain_id", chainID)),
		chainProvider:        provider,
		cc:                   cc,
		lightProvider:        lightProvider,
		pathProcessors:       pathProcessors,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
	}, nil
}

const (
	latestHeightQueryTimeout    = 5 * time.Second
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

var messageHandlers = map[string]func(*CosmosChainProcessor, msgHandlerParams) bool{
	processor.MsgTransfer:        (*CosmosChainProcessor).handleMsgTransfer,
	processor.MsgRecvPacket:      (*CosmosChainProcessor).handleMsgRecvPacket,
	processor.MsgAcknowledgement: (*CosmosChainProcessor).handleMsgAcknowledgement,
	processor.MsgTimeout:         (*CosmosChainProcessor).handleMsgTimeout,
	processor.MsgTimeoutOnClose:  (*CosmosChainProcessor).handleMsgTimeoutOnClose,

	processor.MsgCreateClient:       (*CosmosChainProcessor).handleMsgCreateClient,
	processor.MsgUpdateClient:       (*CosmosChainProcessor).handleMsgUpdateClient,
	processor.MsgUpgradeClient:      (*CosmosChainProcessor).handleMsgUpgradeClient,
	processor.MsgSubmitMisbehaviour: (*CosmosChainProcessor).handleMsgSubmitMisbehaviour,

	processor.MsgConnectionOpenInit:    (*CosmosChainProcessor).handleMsgConnectionOpenInit,
	processor.MsgConnectionOpenTry:     (*CosmosChainProcessor).handleMsgConnectionOpenTry,
	processor.MsgConnectionOpenAck:     (*CosmosChainProcessor).handleMsgConnectionOpenAck,
	processor.MsgConnectionOpenConfirm: (*CosmosChainProcessor).handleMsgConnectionOpenConfirm,

	processor.MsgChannelCloseConfirm: (*CosmosChainProcessor).handleMsgChannelCloseConfirm,
	processor.MsgChannelCloseInit:    (*CosmosChainProcessor).handleMsgChannelCloseInit,
	processor.MsgChannelOpenAck:      (*CosmosChainProcessor).handleMsgChannelOpenAck,
	processor.MsgChannelOpenConfirm:  (*CosmosChainProcessor).handleMsgChannelOpenConfirm,
	processor.MsgChannelOpenInit:     (*CosmosChainProcessor).handleMsgChannelOpenInit,
	processor.MsgChannelOpenTry:      (*CosmosChainProcessor).handleMsgChannelOpenTry,
}

func (ccp *CosmosChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.String("message", m)).Debug("Observed IBC message", fields...)
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
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, latestHeightQueryTimeout)
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

	ppChanged := false

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var blockRes *ctypes.ResultBlockResults
		var lightBlock *tmtypes.LightBlock
		eg.Go(func() (err error) {
			blockRes, err = ccp.cc.Client.BlockResults(ctx, &i)
			return err
		})
		eg.Go(func() (err error) {
			lightBlock, err = ccp.lightProvider.LightBlock(ctx, i)
			return err
		})

		if err := eg.Wait(); err != nil {
			ccp.log.Error("Error querying block data", zap.Error(err))
			return nil
		}

		ccp.latestBlock = provider.LatestBlock{
			Height: uint64(lightBlock.Height),
			Time:   lightBlock.Header.Time,
		}

		for _, tx := range blockRes.TxsResults {
			if tx.Code != 0 {
				// tx was not successful
				continue
			}
			messages := ccp.ibcMessagesFromTransaction(tx)

			ccp.log.Debug("Parsed IBC messages", zap.Any("messages", messages))

			for _, m := range messages {
				handler, ok := messageHandlers[m.messageType]
				if !ok {
					continue
				}
				// call message handler for this ibc message type. can do things like cache things on the chain processor or retain ibc messages that should be sent to the PathProcessors.
				changed := handler(ccp, msgHandlerParams{
					messageInfo:      m.messageInfo,
					ibcMessagesCache: ibcMessagesCache,
				})
				ppChanged = ppChanged || changed
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
		pp.HandleNewData(chainID, processor.ChainProcessorCacheData{
			LatestBlock:          ccp.latestBlock,
			IBCMessagesCache:     ibcMessagesCache,
			InSync:               ccp.inSync,
			ConnectionStateCache: ccp.connectionStateCache,
			ChannelStateCache:    ccp.channelStateCache,
		})
	}

	return nil
}
