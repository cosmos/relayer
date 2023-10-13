package cosmos

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/relayer/v2/relayer/chains"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type CosmosChainProcessor struct {
	log *zap.Logger

	chainProvider *CosmosProvider

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

	// parsed gas prices accepted by the chain (only used for metrics)
	parsedGasPrices *sdk.DecCoins
}

func NewCosmosChainProcessor(
	log *zap.Logger,
	provider *CosmosProvider,
	metrics *processor.PrometheusMetrics,
) *CosmosChainProcessor {
	return &CosmosChainProcessor{
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

const (
	queryTimeout                = 5 * time.Second
	queryStateTimeout           = 60 * time.Second
	blockResultsQueryTimeout    = 2 * time.Minute
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration      = 1 * time.Second
	defaultBalanceUpdateWaitDuration = 60 * time.Second
	inSyncNumBlocksThreshold         = 2
	blockMaxRetries                  = 5
)

// latestClientState is a map of clientID to the latest clientInfo for that client.
type latestClientState map[string]provider.ClientState

func (l latestClientState) update(ctx context.Context, clientInfo chains.ClientInfo, ccp *CosmosChainProcessor) {
	existingClientInfo, ok := l[clientInfo.ClientID]
	var trustingPeriod time.Duration
	if ok {
		if clientInfo.ConsensusHeight.LT(existingClientInfo.ConsensusHeight) {
			// height is less than latest, so no-op
			return
		}
		trustingPeriod = existingClientInfo.TrustingPeriod
	}
	if trustingPeriod == 0 {
		cs, err := ccp.chainProvider.queryTMClientState(ctx, 0, clientInfo.ClientID)
		if err != nil {
			ccp.log.Error(
				"Failed to query client state to get trusting period",
				zap.String("client_id", clientInfo.ClientID),
				zap.Error(err),
			)
			return
		}
		trustingPeriod = cs.TrustingPeriod
	}
	clientState := clientInfo.ClientState(trustingPeriod)

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.ClientID] = clientState
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
		ccp.log.Error(
			"Failed to query latest height",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// nodeStatusWithRetry will query for the latest node status, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (ccp *CosmosChainProcessor) nodeStatusWithRetry(ctx context.Context) (status *ctypes.ResultStatus, err error) {
	return status, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, queryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		status, err = ccp.chainProvider.QueryStatus(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		ccp.log.Error(
			"Failed to query node status",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (ccp *CosmosChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	if state, ok := ccp.latestClientState[clientID]; ok && state.TrustingPeriod > 0 {
		return state, nil
	}

	var clientState provider.ClientState
	if clientID == ibcexported.LocalhostClientID {
		cs, err := ccp.chainProvider.queryLocalhostClientState(ctx, int64(ccp.latestBlock.Height))
		if err != nil {
			return provider.ClientState{}, err
		}
		clientState = provider.ClientState{
			ClientID:        clientID,
			ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
		}
	} else {
		cs, err := ccp.chainProvider.queryTMClientState(ctx, int64(ccp.latestBlock.Height), clientID)
		if err != nil {
			return provider.ClientState{}, err
		}
		clientState = provider.ClientState{
			ClientID:        clientID,
			ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
			TrustingPeriod:  cs.TrustingPeriod,
		}
	}

	ccp.latestClientState[clientID] = clientState
	return clientState, nil
}

// queryCyclePersistence hold the variables that should be retained across queryCycles.
type queryCyclePersistence struct {
	latestHeight                int64
	latestQueriedBlock          int64
	retriesAtLatestQueriedBlock int
	minQueryLoopDuration        time.Duration
	lastBalanceUpdate           time.Time
	balanceUpdateWaitDuration   time.Duration
}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (ccp *CosmosChainProcessor) Run(ctx context.Context, initialBlockHistory uint64, stuckPacket *processor.StuckPacket) error {
	minQueryLoopDuration := ccp.chainProvider.PCfg.MinLoopDuration
	if minQueryLoopDuration == 0 {
		minQueryLoopDuration = defaultMinQueryLoopDuration
	}

	// this will be used for persistence across query cycle loop executions
	persistence := queryCyclePersistence{
		minQueryLoopDuration:      minQueryLoopDuration,
		lastBalanceUpdate:         time.Unix(0, 0),
		balanceUpdateWaitDuration: defaultBalanceUpdateWaitDuration,
	}

	// Infinite retry to get initial latest height
	for {
		status, err := ccp.nodeStatusWithRetry(ctx)
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
		persistence.latestHeight = status.SyncInfo.LatestBlockHeight
		ccp.chainProvider.setCometVersion(ccp.log, status.NodeInfo.Version)
		break
	}

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlock := persistence.latestHeight - int64(initialBlockHistory)

	if latestQueriedBlock < 0 {
		latestQueriedBlock = 0
	}

	if stuckPacket != nil && ccp.chainProvider.ChainId() == stuckPacket.ChainID {
		latestQueriedBlock = int64(stuckPacket.StartHeight)
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

	ticker := time.NewTicker(persistence.minQueryLoopDuration)
	defer ticker.Stop()

	for {
		if err := ccp.queryCycle(ctx, &persistence, stuckPacket); err != nil {
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
func (ccp *CosmosChainProcessor) initializeConnectionState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryStateTimeout)
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
	ctx, cancel := context.WithTimeout(ctx, queryStateTimeout)
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
		k := processor.ChannelKey{
			ChannelID:             ch.ChannelId,
			PortID:                ch.PortId,
			CounterpartyChannelID: ch.Counterparty.ChannelId,
			CounterpartyPortID:    ch.Counterparty.PortId,
		}
		ccp.channelStateCache.SetOpen(k, ch.State == chantypes.OPEN, ch.Ordering)
	}
	return nil
}

func (ccp *CosmosChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence, stuckPacket *processor.StuckPacket) error {
	status, err := ccp.nodeStatusWithRetry(ctx)
	if err != nil {
		// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
		ccp.log.Error(
			"Failed to query node status after max attempts",
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	persistence.latestHeight = status.SyncInfo.LatestBlockHeight
	ccp.chainProvider.setCometVersion(ccp.log, status.NodeInfo.Version)

	// This debug log is very noisy, but is helpful when debugging new chains.
	// ccp.log.Debug("Queried latest height",
	// 	zap.Int64("latest_height", persistence.latestHeight),
	// )

	if ccp.metrics != nil {
		ccp.CollectMetrics(ctx, persistence)
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

	ppChanged := false

	var latestHeader provider.TendermintIBCHeader

	newLatestQueriedBlock := persistence.latestQueriedBlock

	chainID := ccp.chainProvider.ChainId()

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		var eg errgroup.Group
		var blockRes *ctypes.ResultBlockResults
		var ibcHeader provider.IBCHeader
		sI := i
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, blockResultsQueryTimeout)
			defer cancelQueryCtx()
			blockRes, err = ccp.chainProvider.RPCClient.BlockResults(queryCtx, &sI)
			if err != nil && ccp.metrics != nil {
				ccp.metrics.IncBlockQueryFailure(chainID, "RPC Client")
			}
			return err
		})
		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()
			ibcHeader, err = ccp.chainProvider.QueryIBCHeader(queryCtx, sI)
			if err != nil && ccp.metrics != nil {
				ccp.metrics.IncBlockQueryFailure(chainID, "IBC Header")
			}
			return err
		})

		if err := eg.Wait(); err != nil {
			ccp.log.Debug(
				"Error querying block data",
				zap.Int64("height", i),
				zap.Error(err),
			)

			persistence.retriesAtLatestQueriedBlock++
			if persistence.retriesAtLatestQueriedBlock >= blockMaxRetries {
				ccp.log.Warn("Reached max retries querying for block, skipping", zap.Int64("height", i))
				// skip this block. now depends on flush to pickup anything missed in the block.
				persistence.latestQueriedBlock = i
				persistence.retriesAtLatestQueriedBlock = 0
				continue
			}
			break
		}

		ccp.log.Debug(
			"Queried block",
			zap.Int64("height", i),
			zap.Int64("latest", persistence.latestHeight),
			zap.Int64("delta", persistence.latestHeight-i),
		)

		persistence.retriesAtLatestQueriedBlock = 0

		latestHeader = ibcHeader.(provider.TendermintIBCHeader)

		heightUint64 := uint64(i)

		ccp.latestBlock = provider.LatestBlock{
			Height: heightUint64,
			Time:   latestHeader.SignedHeader.Time,
		}

		ibcHeaderCache[heightUint64] = latestHeader
		ppChanged = true

		base64Encoded := ccp.chainProvider.cometLegacyEncoding

		blockMsgs := ccp.ibcMessagesFromBlockEvents(
			blockRes.BeginBlockEvents,
			blockRes.EndBlockEvents,
			heightUint64,
			base64Encoded,
		)
		for _, m := range blockMsgs {
			ccp.handleMessage(ctx, m, ibcMessagesCache)
		}

		for _, tx := range blockRes.TxsResults {
			if tx.Code != 0 {
				// tx was not successful
				continue
			}
			messages := chains.IbcMessagesFromEvents(ccp.log, tx.Events, chainID, heightUint64, base64Encoded)

			for _, m := range messages {
				ccp.handleMessage(ctx, m, ibcMessagesCache)
			}
		}

		newLatestQueriedBlock = i

		if stuckPacket != nil &&
			ccp.chainProvider.ChainId() == stuckPacket.ChainID &&
			newLatestQueriedBlock == int64(stuckPacket.EndHeight) {
			i = persistence.latestHeight
			ccp.log.Debug("Parsed stuck packet height, skipping to current")
		}
	}

	if newLatestQueriedBlock == persistence.latestQueriedBlock {
		return nil
	}

	if !ppChanged {
		if firstTimeInSync {
			for _, pp := range ccp.pathProcessors {
				pp.ProcessBacklogIfReady()
			}
		}

		return nil
	}

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
			IBCMessagesCache:     ibcMessagesCache.Clone(),
			InSync:               ccp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: ccp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    ccp.channelStateCache.FilterForClient(clientID, ccp.channelConnections, ccp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache.Clone(),
		})
	}

	persistence.latestQueriedBlock = newLatestQueriedBlock

	return nil
}

func (ccp *CosmosChainProcessor) CollectMetrics(ctx context.Context, persistence *queryCyclePersistence) {
	ccp.CurrentBlockHeight(ctx, persistence)

	// Wait a while before updating the balance
	if time.Since(persistence.lastBalanceUpdate) > persistence.balanceUpdateWaitDuration {
		ccp.CurrentRelayerBalance(ctx)
		persistence.lastBalanceUpdate = time.Now()
	}
}

func (ccp *CosmosChainProcessor) CurrentBlockHeight(ctx context.Context, persistence *queryCyclePersistence) {
	ccp.metrics.SetLatestHeight(ccp.chainProvider.ChainId(), persistence.latestHeight)
}

func (ccp *CosmosChainProcessor) CurrentRelayerBalance(ctx context.Context) {
	// memoize the current gas prices to only show metrics for "interesting" denoms
	if ccp.parsedGasPrices == nil {
		gp, err := sdk.ParseDecCoins(ccp.chainProvider.PCfg.GasPrices)
		if err != nil {
			ccp.log.Error(
				"Failed to parse gas prices",
				zap.Error(err),
			)
		}
		ccp.parsedGasPrices = &gp
	}

	// Get the balance for the chain provider's key
	relayerWalletBalances, err := ccp.chainProvider.QueryBalance(ctx, ccp.chainProvider.Key())
	if err != nil {
		ccp.log.Error(
			"Failed to query relayer balance",
			zap.Error(err),
		)
	}
	address, err := ccp.chainProvider.Address()
	if err != nil {
		ccp.log.Error(
			"Failed to get relayer bech32 wallet addresss",
			zap.Error(err),
		)
	}
	// Print the relevant gas prices
	for _, gasDenom := range *ccp.parsedGasPrices {
		bal := relayerWalletBalances.AmountOf(gasDenom.Denom)
		// Convert to a big float to get a float64 for metrics
		f, _ := big.NewFloat(0.0).SetInt(bal.BigInt()).Float64()
		ccp.metrics.SetWalletBalance(ccp.chainProvider.ChainId(), ccp.chainProvider.PCfg.GasPrices, ccp.chainProvider.Key(), address, gasDenom.Denom, f)
	}
}
