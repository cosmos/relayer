package cosmos

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/danwt/gerr/gerr"
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
		log:                  log.With(zap.String("chain_id", provider.ChainId())),
		chainProvider:        provider,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
		connectionClients:    make(map[string]string),
		channelConnections:   make(map[string]string),
		metrics:              metrics,
		rotErr:               make(chan struct{}),
	}
}

const (
	queryTimeout                    = 5 * time.Second
	queryStateTimeout               = 60 * time.Second
	defaultBlockResultsQueryTimeout = 2 * time.Minute
	latestHeightQueryRetryDelay     = 1 * time.Second
	latestHeightQueryRetries        = 5

	defaultMinQueryLoopDuration      = 1 * time.Second
	defaultBalanceUpdateWaitDuration = 60 * time.Second
	defaultInSyncNumBlocksThreshold  = 2
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
				"Query client state to get trusting period.",
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
		ccp.log.Debug(
			"Retrying query latest height.",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
	}))
}

// nodeStatusWithRetry will query for the latest node status, retrying in case of failure.
// It will delay by latestHeightQueryRetryDelay between attempts, up to latestHeightQueryRetries.
func (ccp *CosmosChainProcessor) nodeStatusWithRetry(ctx context.Context) (status *coretypes.ResultStatus, err error) {
	return status, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(ctx, queryTimeout)
		defer cancelLatestHeightQueryCtx()
		var err error
		status, err = ccp.chainProvider.QueryStatus(latestHeightQueryCtx)
		return err
	}, retry.Context(ctx), retry.Attempts(latestHeightQueryRetries), retry.Delay(latestHeightQueryRetryDelay), retry.LastErrorOnly(true), retry.OnRetry(func(n uint, err error) {
		ccp.log.Debug(
			"Retrying query node status.",
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
	// the latest known height of the chain
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
				"Query latest height after max attempts",
				zap.Uint("attempts", latestHeightQueryRetries),
				zap.Error(err),
			)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			continue
		}
		persistence.latestHeight = status.SyncInfo.LatestBlockHeight
		break
	}

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlock := persistence.latestHeight - int64(initialBlockHistory)

	if latestQueriedBlock < 0 {
		latestQueriedBlock = 0
	}

	afterUnstuck := latestQueriedBlock
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

	ccp.log.Debug("Entering main query loop.")

	ticker := time.NewTicker(persistence.minQueryLoopDuration)
	defer ticker.Stop()

	for {
		if err := ccp.queryCycle(ctx, &persistence, stuckPacket, afterUnstuck); err != nil {
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
		return fmt.Errorf("querying connections: %w", err)
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	channels, err := ccp.chainProvider.QueryChannels(ctx)
	if err != nil {
		return fmt.Errorf("querying channels: %w", err)
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

func (ccp *CosmosChainProcessor) queryCycle(
	ctx context.Context,
	persistence *queryCyclePersistence,
	stuckPacket *processor.StuckPacket,
	afterUnstuck int64,
) error {
	status, err := ccp.nodeStatusWithRetry(ctx)
	if err != nil {
		// don't want to cause CosmosChainProcessor to quit here, can retry again next cycle.
		ccp.log.Error(
			"Query node status after max attempts.",
			zap.Uint("attempts", latestHeightQueryRetries),
			zap.Error(err),
		)
		return nil
	}

	persistence.latestHeight = status.SyncInfo.LatestBlockHeight

	if ccp.metrics != nil {
		ccp.CollectMetrics(ctx, persistence)
	}

	// used at the end of the cycle to send signal to path processors to start processing if both chains are in sync and no new messages came in this cycle
	firstTimeInSync := false

	if !ccp.inSync {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < int64(defaultInSyncNumBlocksThreshold) {
			ccp.inSync = true
			firstTimeInSync = true
			ccp.log.Info("Chain in sync.", zap.Bool("first time", firstTimeInSync))
		} else {
			ccp.log.Info("Chain not in sync.",
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

	firstHeightToQuery := persistence.latestQueriedBlock
	// On the first ever update, we want to make sure we propagate the block info to the path processor
	// Afterward, we only want to query new blocks
	if ccp.inSync && !firstTimeInSync {
		firstHeightToQuery++
	}

	for heightToQuery := firstHeightToQuery; heightToQuery <= persistence.latestHeight; heightToQuery++ {
		var (
			eg        errgroup.Group
			blockRes  *coretypes.ResultBlockResults
			ibcHeader provider.IBCHeader
		)

		heightToQuery := heightToQuery

		eg.Go(func() (err error) {
			// there is no need for an explicit timeout here, since the rpc client already embeds a timeout
			x := time.Now()
			c := make(chan struct{})
			go func() {
				t := time.NewTicker(30 * time.Second)
				y := time.Now()
				for {
					select {
					case <-t.C:
						ccp.log.Debug("Long running block results query is still ongoing", zap.Any("elapsed", time.Since(y)), zap.Any("chain", chainID))
					case <-c:
						t.Stop()
						return
					}
				}
			}()
			blockRes, err = ccp.chainProvider.RPCClient.BlockResults(ctx, &heightToQuery)
			if err != nil && ccp.metrics != nil {
				ccp.metrics.IncBlockQueryFailure(chainID, "RPC Client")
			}
			if err != nil && strings.Contains(err.Error(), "unmarshal") {
				err = fmt.Errorf("%w:%w", gerr.ErrDataLoss, err)
			}
			if err != nil {
				return fmt.Errorf("block results: elapsed: %s: %w", time.Since(x), err)
			}
			close(c)
			return nil
		})

		eg.Go(func() (err error) {
			queryCtx, cancelQueryCtx := context.WithTimeout(ctx, queryTimeout)
			defer cancelQueryCtx()

			ibcHeader, err = ccp.chainProvider.QueryIBCHeader(queryCtx, heightToQuery)
			if err != nil && ccp.metrics != nil {
				ccp.metrics.IncBlockQueryFailure(chainID, "IBC Header")
			}

			if err != nil {
				return fmt.Errorf("query ibc header: %w", err)
			}
			return nil
		})

		if err := eg.Wait(); err != nil {
			ccp.log.DPanic(
				"Querying block data.",
				zap.Int64("height", heightToQuery),
				zap.Error(err),
			)

			persistence.retriesAtLatestQueriedBlock++
			if errors.Is(err, gerr.ErrDataLoss) || persistence.retriesAtLatestQueriedBlock >= blockMaxRetries {
				ccp.log.Error("Reached max retries or got unrecoverable error querying for block, skipping.", zap.Int64("height", heightToQuery))
				// skip this block. now depends on flush to pickup anything missed in the block.
				persistence.latestQueriedBlock = heightToQuery
				persistence.retriesAtLatestQueriedBlock = 0
				continue
			}
			break
		}

		persistence.retriesAtLatestQueriedBlock = 0

		latestHeader = ibcHeader.(provider.TendermintIBCHeader)

		heightToQueryU := uint64(heightToQuery)

		ccp.latestBlock = provider.LatestBlock{
			Height: heightToQueryU,
			Time:   latestHeader.SignedHeader.Time,
		}

		ibcHeaderCache[heightToQueryU] = latestHeader
		ppChanged = true

		messages := chains.IbcMessagesFromEvents(ccp.log, blockRes.FinalizeBlockEvents, chainID, heightToQueryU)

		for _, m := range messages {
			ccp.handleMessage(ctx, m, ibcMessagesCache)
		}

		for _, tx := range blockRes.TxsResults {
			if tx.Code != 0 {
				// tx was not successful
				continue
			}

			messages := chains.IbcMessagesFromEvents(ccp.log, tx.Events, chainID, heightToQueryU)

			for _, m := range messages {
				switch t := m.Info.(type) {
				case *chains.PacketInfo:
					if stuckPacket != nil && ccp.chainProvider.ChainId() == stuckPacket.ChainID && int64(stuckPacket.StartHeight) <= heightToQuery && heightToQuery <= int64(stuckPacket.EndHeight) {
						ccp.log.Info("Found stuck packet message.", zap.Any("seq", t.Sequence), zap.Any("height", t.Height))
					}
				}
				ccp.handleMessage(ctx, m, ibcMessagesCache)
			}

		}

		newLatestQueriedBlock = heightToQuery

		if stuckPacket != nil &&
			ccp.chainProvider.ChainId() == stuckPacket.ChainID &&
			newLatestQueriedBlock == int64(stuckPacket.EndHeight) {

			heightToQuery = persistence.latestHeight

			newLatestQueriedBlock = afterUnstuck
			ccp.log.Info("Parsed stuck packet height, skipping to current.", zap.Any("new latest queried block", newLatestQueriedBlock))
		}
	}

	if (ccp.inSync && !firstTimeInSync) && newLatestQueriedBlock == persistence.latestQueriedBlock {
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
			ccp.log.Error("Fetching client state.",
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
				"Parse gas prices.",
				zap.Error(err),
			)
		}
		ccp.parsedGasPrices = &gp
	}

	// Get the balance for the chain provider's key
	relayerWalletBalances, err := ccp.chainProvider.QueryBalance(ctx, ccp.chainProvider.Key())
	if err != nil {
		ccp.log.Error(
			"Query relayer balance.",
			zap.Error(err),
		)
	}
	address, err := ccp.chainProvider.Address()
	if err != nil {
		ccp.log.Error(
			"Get relayer bech32 wallet address.",
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
