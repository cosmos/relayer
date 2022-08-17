package substrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

func (l latestClientState) update(clientInfo clientInfo) {
	existingClientInfo, ok := l[clientInfo.ClientID]
	if ok && clientInfo.ConsensusHeight.LT(existingClientInfo.ConsensusHeight) {
		// height is less than latest, so no-op
		return
	}

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.ClientID] = clientInfo.ClientState()
}

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
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, queryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = scp.chainProvider.QueryLatestHeight(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		scp.log.Info(
			"Failed to query latest height",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (scp *SubstrateChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	if state, ok := scp.latestClientState[clientID]; ok {
		return state, nil
	}
	cs, err := scp.chainProvider.QueryClientState(ctx, int64(scp.latestBlock.Height), clientID)
	if err != nil {
		return provider.ClientState{}, err
	}
	clientState := provider.ClientState{
		ClientID:        clientID,
		ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
	}
	scp.latestClientState[clientID] = clientState
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
func (scp *SubstrateChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	// this will be used for persistence across query cycle loop executions
	persistence := queryCyclePersistence{
		minQueryLoopDuration: defaultMinQueryLoopDuration,
	}

	// Infinite retry to get initial latest height
	for {
		latestHeight, err := scp.latestHeightWithRetry(ctx)
		if err != nil {
			scp.log.Error(
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
		return scp.initializeConnectionState(ctx)
	})
	eg.Go(func() error {
		return scp.initializeChannelState(ctx)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	scp.log.Debug("Entering main query loop")

	ticker := time.NewTicker(persistence.minQueryLoopDuration)

	for {
		if err := scp.queryCycle(ctx, &persistence); err != nil {
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
func (scp *SubstrateChainProcessor) initializeConnectionState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	connections, err := scp.chainProvider.QueryConnections(ctx)
	if err != nil {
		return fmt.Errorf("error querying connections: %w", err)
	}
	for _, c := range connections {
		scp.connectionClients[c.Id] = c.ClientId
		scp.connectionStateCache[processor.ConnectionKey{
			ConnectionID:         c.Id,
			ClientID:             c.ClientId,
			CounterpartyConnID:   c.Counterparty.ConnectionId,
			CounterpartyClientID: c.Counterparty.ClientId,
		}] = c.State == conntypes.OPEN
	}
	return nil
}

// initializeChannelState will bootstrap the channelStateCache with the open channel state.
func (scp *SubstrateChainProcessor) initializeChannelState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	channels, err := scp.chainProvider.QueryChannels(ctx)
	if err != nil {
		return fmt.Errorf("error querying channels: %w", err)
	}
	for _, ch := range channels {
		if len(ch.ConnectionHops) != 1 {
			scp.log.Error("Found channel using multiple connection hops. Not currently supported, ignoring.",
				zap.String("channel_id", ch.ChannelId),
				zap.String("port_id", ch.PortId),
				zap.Strings("connection_hops", ch.ConnectionHops),
			)
			continue
		}
		scp.channelConnections[ch.ChannelId] = ch.ConnectionHops[0]
		scp.channelStateCache[processor.ChannelKey{
			ChannelID:             ch.ChannelId,
			PortID:                ch.PortId,
			CounterpartyChannelID: ch.Counterparty.ChannelId,
			CounterpartyPortID:    ch.Counterparty.PortId,
		}] = ch.State == chantypes.OPEN
	}
	return nil
}

func (scp *SubstrateChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) error {
	var err error
	persistence.latestHeight, err = scp.latestHeightWithRetry(ctx)

	// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
	if err != nil {
		scp.log.Error(
			"Failed to query latest height after max attempts",
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	scp.log.Debug("Queried latest height",
		zap.Int64("latest_height", persistence.latestHeight),
	)

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !scp.inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncNumBlocksThreshold {
			scp.inSync = true
			firstTimeInSync = true
			scp.log.Info("Chain is in sync")
		} else {
			scp.log.Info("Chain is not yet in sync",
				zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
				zap.Int64("latest_height", persistence.latestHeight),
			)
		}
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()

	ibcHeaderCache := make(processor.IBCHeaderCache)

	ppChanged := false

	var latestHeader SubstrateIBCHeader

	newLatestQueriedBlock := persistence.latestQueriedBlock

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var ibcEvents rpcclienttypes.IBCEventsQueryResult
		var ibcHeader provider.IBCHeader
		i := i
		heightUint64 := uint64(i)

		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()
			ibcEvents, err = scp.chainProvider.RPCClient.RPC.IBC.QueryIbcEvents(queryCtx, []rpcclienttypes.BlockNumberOrHash{{Number: uint32(heightUint64)}})
			return err
		})
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()
			ibcHeader, err = scp.chainProvider.QueryIBCHeader(queryCtx, i)
			return err
		})

		if err := eg.Wait(); err != nil {
			scp.log.Warn("Error querying block data", zap.Error(err))
			break
		}

		latestHeader = ibcHeader.(SubstrateIBCHeader)

		// TODO: update latest bolock
		// scp.latestBlock = provider.LatestBlock{
		// 	Height: heightUint64,
		// 	Time:   latestHeader.SignedHeader.Time,
		// }

		ibcHeaderCache[heightUint64] = latestHeader
		ppChanged = true

		scp.handleIBCMessagesFromEvents(ibcEvents, heightUint64, ibcMessagesCache)

		newLatestQueriedBlock = i
	}

	if newLatestQueriedBlock == persistence.latestQueriedBlock {
		return nil
	}

	if !ppChanged {
		if firstTimeInSync {
			for _, pp := range scp.pathProcessors {
				pp.ProcessBacklogIfReady()
			}
		}

		return nil
	}

	chainID := scp.chainProvider.ChainId()

	for _, pp := range scp.pathProcessors {
		clientID := pp.RelevantClientID(chainID)
		clientState, err := scp.clientState(ctx, clientID)
		if err != nil {
			scp.log.Error("Error fetching client state",
				zap.String("client_id", clientID),
				zap.Error(err),
			)
			continue
		}

		pp.HandleNewData(chainID, processor.ChainProcessorCacheData{
			LatestBlock:          scp.latestBlock,
			LatestHeader:         latestHeader,
			IBCMessagesCache:     ibcMessagesCache,
			InSync:               scp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: scp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    scp.channelStateCache.FilterForClient(clientID, scp.channelConnections, scp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache,
		})
	}

	persistence.latestQueriedBlock = newLatestQueriedBlock

	return nil
}
