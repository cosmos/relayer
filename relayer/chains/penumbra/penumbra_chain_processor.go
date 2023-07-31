package penumbra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const blockResultsQueryTimeout = 2 * time.Minute

type PenumbraChainProcessor struct {
	log *zap.Logger

	chainProvider *PenumbraProvider

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

func NewPenumbraChainProcessor(log *zap.Logger, provider *PenumbraProvider) *PenumbraChainProcessor {
	return &PenumbraChainProcessor{
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
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration = 1 * time.Second
	inSyncNumBlocksThreshold    = 2
)

type msgHandlerParams struct {
	// incoming IBC message
	messageInfo any

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
	// TODO: don't hardcode
	tp := time.Hour * 2

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientInfo.ClientState(tp)
}

// Provider returns the ChainProvider, which provides the methods for querying, assembling IBC messages, and sending transactions.
func (pcp *PenumbraChainProcessor) Provider() provider.ChainProvider {
	return pcp.chainProvider
}

// Set the PathProcessors that this ChainProcessor should publish relevant IBC events to.
// ChainProcessors need reference to their PathProcessors and vice-versa, handled by EventProcessorBuilder.Build().
func (pcp *PenumbraChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	pcp.pathProcessors = pathProcessors
}

// latestHeightWithRetry will query for the latest height, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (pcp *PenumbraChainProcessor) latestHeightWithRetry(ctx context.Context) (latestHeight int64, err error) {
	return latestHeight, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, queryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = pcp.chainProvider.QueryLatestHeight(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		pcp.log.Info(
			"Failed to query latest height",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (pcp *PenumbraChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	if state, ok := pcp.latestClientState[clientID]; ok {
		return state, nil
	}
	cs, err := pcp.chainProvider.QueryClientState(ctx, int64(pcp.latestBlock.Height), clientID)
	if err != nil {
		return provider.ClientState{}, err
	}
	clientState := provider.ClientState{
		ClientID:        clientID,
		ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
	}
	pcp.latestClientState[clientID] = clientState
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
func (pcp *PenumbraChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	minQueryLoopDuration := pcp.chainProvider.PCfg.MinLoopDuration
	if minQueryLoopDuration == 0 {
		minQueryLoopDuration = defaultMinQueryLoopDuration
	}

	// this will be used for persistence across query cycle loop executions
	persistence := queryCyclePersistence{
		minQueryLoopDuration: minQueryLoopDuration,
	}

	// Infinite retry to get initial latest height
	for {
		latestHeight, err := pcp.latestHeightWithRetry(ctx)
		if err != nil {
			pcp.log.Error(
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
		return pcp.initializeConnectionState(ctx)
	})
	eg.Go(func() error {
		return pcp.initializeChannelState(ctx)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	pcp.log.Debug("Entering main query loop")

	ticker := time.NewTicker(persistence.minQueryLoopDuration)

	for {
		if err := pcp.queryCycle(ctx, &persistence); err != nil {
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

// initializeConnectionState will bootstrap the connectionStateCache with the open connection state.
func (pcp *PenumbraChainProcessor) initializeConnectionState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	connections, err := pcp.chainProvider.QueryConnections(ctx)
	if err != nil {
		return fmt.Errorf("error querying connections: %w", err)
	}
	for _, c := range connections {
		pcp.connectionClients[c.Id] = c.ClientId
		pcp.connectionStateCache[processor.ConnectionKey{
			ConnectionID:         c.Id,
			ClientID:             c.ClientId,
			CounterpartyConnID:   c.Counterparty.ConnectionId,
			CounterpartyClientID: c.Counterparty.ClientId,
		}] = c.State == conntypes.OPEN
	}
	return nil
}

// initializeChannelState will bootstrap the channelStateCache with the open channel state.
func (pcp *PenumbraChainProcessor) initializeChannelState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	channels, err := pcp.chainProvider.QueryChannels(ctx)
	if err != nil {
		return fmt.Errorf("error querying channels: %w", err)
	}
	for _, ch := range channels {
		if len(ch.ConnectionHops) != 1 {
			pcp.log.Error("Found channel using multiple connection hops. Not currently supported, ignoring.",
				zap.String("channel_id", ch.ChannelId),
				zap.String("port_id", ch.PortId),
				zap.Any("connection_hops", ch.ConnectionHops),
			)
			continue
		}
		pcp.channelConnections[ch.ChannelId] = ch.ConnectionHops[0]
		k := processor.ChannelKey{
			ChannelID:             ch.ChannelId,
			PortID:                ch.PortId,
			CounterpartyChannelID: ch.Counterparty.ChannelId,
			CounterpartyPortID:    ch.Counterparty.PortId,
		}
		pcp.channelStateCache.SetOpen(k, ch.State == chantypes.OPEN, ch.Ordering)
	}
	return nil
}

// ABCI results from a block
type ResultBlockResults struct {
	Height                int64                    `json:"height,string"`
	TxsResults            []*ExecTxResult          `json:"txs_results"`
	TotalGasUsed          int64                    `json:"total_gas_used,string"`
	FinalizeBlockEvents   []Event                  `json:"finalize_block_events"`
	ValidatorUpdates      []ValidatorUpdate        `json:"validator_updates"`
	ConsensusParamUpdates *tmproto.ConsensusParams `json:"consensus_param_updates"`
}

func (pcp *PenumbraChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) error {
	var err error
	persistence.latestHeight, err = pcp.latestHeightWithRetry(ctx)

	// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
	if err != nil {
		pcp.log.Error(
			"Failed to query latest height after max attempts",
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	pcp.log.Debug("Queried latest height",
		zap.Int64("latest_height", persistence.latestHeight),
	)

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !pcp.inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncNumBlocksThreshold {
			pcp.inSync = true
			firstTimeInSync = true
			pcp.log.Info("Chain is in sync")
		} else {
			pcp.log.Info("Chain is not yet in sync",
				zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
				zap.Int64("latest_height", persistence.latestHeight),
			)
		}
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ibcHeaderCache := make(processor.IBCHeaderCache)

	ppChanged := false

	var latestHeader PenumbraIBCHeader

	newLatestQueriedBlock := persistence.latestQueriedBlock

	chainID := pcp.chainProvider.ChainId()

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var blockRes *ctypes.ResultBlockResults
		var ibcHeader provider.IBCHeader
		i := i
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, blockResultsQueryTimeout)
			defer cancelQueryCtx()
			blockRes, err = pcp.chainProvider.RPCClient.BlockResults(queryCtx, &i)
			return err
		})
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()
			ibcHeader, err = pcp.chainProvider.QueryIBCHeader(queryCtx, i)
			return err
		})

		if err := eg.Wait(); err != nil {
			pcp.log.Warn("Error querying block data", zap.Error(err))
			break
		}

		latestHeader = ibcHeader.(PenumbraIBCHeader)

		heightUint64 := uint64(i)

		pcp.latestBlock = provider.LatestBlock{
			Height: heightUint64,
			Time:   latestHeader.SignedHeader.Time,
		}

		ibcHeaderCache[heightUint64] = latestHeader
		ppChanged = true

		blockMsgs := pcp.ibcMessagesFromBlockEvents(blockRes.BeginBlockEvents, blockRes.EndBlockEvents, heightUint64, true)
		for _, m := range blockMsgs {
			pcp.handleMessage(m, ibcMessagesCache)
		}

		for _, tx := range blockRes.TxsResults {
			if tx.Code != 0 {
				// tx was not successful
				continue
			}
			messages := ibcMessagesFromEvents(pcp.log, tx.Events, chainID, heightUint64, true)

			for _, m := range messages {
				pcp.handleMessage(m, ibcMessagesCache)
			}
		}
		newLatestQueriedBlock = i
	}

	if newLatestQueriedBlock == persistence.latestQueriedBlock {
		return nil
	}

	if !ppChanged {
		if firstTimeInSync {
			for _, pp := range pcp.pathProcessors {
				pp.ProcessBacklogIfReady()
			}
		}

		return nil
	}

	for _, pp := range pcp.pathProcessors {
		clientID := pp.RelevantClientID(chainID)
		clientState, err := pcp.clientState(ctx, clientID)
		if err != nil {
			pcp.log.Error("Error fetching client state",
				zap.String("client_id", clientID),
				zap.Error(err),
			)
			continue
		}

		pp.HandleNewData(chainID, processor.ChainProcessorCacheData{
			LatestBlock:          pcp.latestBlock,
			LatestHeader:         latestHeader,
			IBCMessagesCache:     ibcMessagesCache.Clone(),
			InSync:               pcp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: pcp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    pcp.channelStateCache.FilterForClient(clientID, pcp.channelConnections, pcp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache.Clone(),
		})
	}

	persistence.latestQueriedBlock = newLatestQueriedBlock

	return nil
}
