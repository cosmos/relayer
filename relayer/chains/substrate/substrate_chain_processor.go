package substrate

import (
	"context"
	"errors"
	"fmt"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"sort"
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

const (
	queryTimeout                = 5 * time.Second
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5
	// inSyncNumBlocksThreshold is an estimation of 2 BEEFY finalization rounds. The relayer is not in
	// sync if the last processed block is greater than 2 relay chain finalization rounds.
	// TODO: this should be reviewed when the light client is changed to a GRANDPA client.
	inSyncNumBlocksThreshold = 16
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

type BlockRange struct {
	startBlock uint64
	endBlock   uint64
}

// processCyclePersistence hold the variables that should be retained across queryCycles.
type processCyclePersistence struct {
	startBlockHeight  uint64
	latestBlockHeight uint64
}

// Run starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors.
// The initialBlockHistory parameter determines how many historical blocks should be fetched and processed before continuing with current blocks.
// ChainProcessors should obey the context and return upon context cancellation.
func (scp *SubstrateChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	// this will be used for persistence across even process cycles
	persistence := processCyclePersistence{}

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
		persistence.latestBlockHeight = uint64(latestHeight)
		break
	}

	// this will make initial ibc event processing look back initialBlockHistory blocks in history
	var hasHistoryToProcess bool
	persistence.startBlockHeight = persistence.latestBlockHeight - initialBlockHistory
	if persistence.startBlockHeight > 0 {
		hasHistoryToProcess = true
	}

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

	scp.log.Debug("subscribing and listening to relay chain commitments")
	commitments := make(chan interface{})
	sub, err := scp.chainProvider.RelayRPCClient.Client.Subscribe(
		context.Background(),
		"beefy",
		"subscribeJustifications",
		"unsubscribeJustifications",
		"justifications",
		commitments,
	)
	if err != nil {
		return err
	}

	defer sub.Unsubscribe()

	for {
		if hasHistoryToProcess {
			err := scp.processIBCEvents(ctx, &persistence, nil)
			if err != nil {
				return err
			}
			hasHistoryToProcess = false
		}

		select {
		case msg, ok := <-commitments:
			if !ok {
				return fmt.Errorf("commitments channel is closed")
			}

			compactCommitment := rpcclienttypes.CompactSignedCommitment{}
			err = rpcclienttypes.DecodeFromHex(msg.(string), &compactCommitment)
			if err != nil {
				return err
			}

			err := scp.processIBCEvents(ctx, &persistence, &compactCommitment)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
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

func (scp *SubstrateChainProcessor) processIBCEvents(
	ctx context.Context,
	persistence *processCyclePersistence,
	cc *rpcclienttypes.CompactSignedCommitment,
) error {
	if cc != nil {
		commitment := cc.Unpack()
		blockNumber := uint64(commitment.Commitment.BlockNumber)
		if persistence.startBlockHeight <= 0 {
			// we need a range of finalized relay chain blocks. If the start block is the genesis block,
			// skip event processing cycle
			persistence.startBlockHeight = blockNumber + 1
			return nil
		}

		persistence.latestBlockHeight = blockNumber
	}

	var firstTimeSync bool
	if !scp.inSync {
		if (persistence.latestBlockHeight - persistence.startBlockHeight) > inSyncNumBlocksThreshold {
			scp.inSync = true
			firstTimeSync = true
			scp.log.Info("Chain is in sync")
		}
	}

	header, err := scp.chainProvider.QueryIBCHeaderOverBlocks(persistence.latestBlockHeight, persistence.startBlockHeight)
	if err != nil {
		return nil
	}

	beefyHeader := header.(SubstrateIBCHeader)
	var blockNumbers []rpcclienttypes.BlockNumberOrHash
	for _, head := range beefyHeader.SignedHeader.HeadersWithProof.Headers {
		decodedHeader, err := beefyclienttypes.DecodeParachainHeader(head.ParachainHeader)
		if err != nil {
			return err
		}
		blockNumbers = append(blockNumbers, rpcclienttypes.BlockNumberOrHash{Number: uint32(decodedHeader.Number)})
	}

	sort.SliceStable(blockNumbers, func(i, j int) bool {
		return blockNumbers[i].Number < blockNumbers[j].Number
	})

	ibcEvents, err := scp.chainProvider.RPCClient.RPC.IBC.QueryIbcEvents(ctx, blockNumbers)
	if err != nil {
		return err
	}

	// skip event processing cycle if there are no events to process
	if len(ibcEvents) == 0 {
		if firstTimeSync {
			for _, pp := range scp.pathProcessors {
				pp.ProcessBacklogIfReady()
			}
		}
		return nil
	}

	ibcMessagesCache := processor.NewIBCMessagesCache()
	messages := scp.ibcMessagesFromEvents(ibcEvents)
	for i := 0; i < len(messages); i++ {
		scp.handleMessage(messages[i], ibcMessagesCache)
	}

	chainID := scp.chainProvider.ChainId()
	ibcHeaderCache := make(processor.IBCHeaderCache)
	ibcHeaderCache[persistence.latestBlockHeight] = beefyHeader
	scp.latestBlock = provider.LatestBlock{
		Height: persistence.latestBlockHeight,
		Time:   beefyHeader.SignedHeader.ConsensusState().Timestamp,
	}

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
			LatestHeader:         beefyHeader,
			IBCMessagesCache:     ibcMessagesCache,
			InSync:               scp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: scp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    scp.channelStateCache.FilterForClient(clientID, scp.channelConnections, scp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache,
		})
	}

	// update the start block of the next process cycle to the latest process block height + 1
	persistence.startBlockHeight = persistence.latestBlockHeight + 1
	return nil
}
