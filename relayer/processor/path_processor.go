package processor

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

const (
	// durationErrorRetry determines how long to wait before retrying
	// in the case of failure to send transactions with IBC messages.
	durationErrorRetry = 5 * time.Second

	// Amount of time to wait when sending transactions before giving up
	// and continuing on. Messages will be retried later if they are still
	// relevant.
	messageSendTimeout = 60 * time.Second

	// Amount of time to wait for a proof to be queried before giving up.
	// The proof query will be retried later if the message still needs
	// to be relayed.
	packetProofQueryTimeout = 5 * time.Second

	// Amount of time to wait for interchain queries.
	interchainQueryTimeout = 60 * time.Second

	// If message assembly fails from either proof query failure on the source
	// or assembling the message for the destination, how many blocks should pass
	// before retrying.
	blocksToRetryAssemblyAfter = 0

	// If the message was assembled successfully, but sending the message failed,
	// how many blocks should pass before retrying.
	blocksToRetrySendAfter = 5

	// How many times to retry sending a message before giving up on it.
	maxMessageSendRetries = 5

	// How many blocks of history to retain ibc headers in the cache for.
	ibcHeadersToCache = 10

	// How many blocks of history before determining that a query needs to be
	// made to retrieve the client consensus state in order to assemble a
	// MsgUpdateClient message.
	clientConsensusHeightUpdateThresholdBlocks = 2
)

// PathProcessor is a process that handles incoming IBC messages from a pair of chains.
// It determines what messages need to be relayed, and sends them.
type PathProcessor struct {
	log *zap.Logger

	pathEnd1 *pathEndRuntime
	pathEnd2 *pathEndRuntime

	// TODO: Do we need two separate lists?
	hopsPathEnd1to2 []*pathEndRuntime
	hopsPathEnd2to1 []*pathEndRuntime

	memo string

	clientUpdateThresholdTime time.Duration

	messageLifecycle MessageLifecycle

	initialFlushComplete bool
	flushTicker          *time.Ticker
	flushInterval        time.Duration

	// Signals to retry.
	retryProcess chan struct{}

	sentInitialMsg bool

	metrics *PrometheusMetrics
}

// PathProcessors is a slice of PathProcessor instances
type PathProcessors []*PathProcessor

func (p PathProcessors) IsRelayedChannel(k ChannelKey, chainID string) bool {
	for _, pp := range p {
		if pp.IsRelayedChannel(chainID, k) {
			return true
		}
	}
	return false
}

func NewPathProcessor(
	log *zap.Logger,
	pathEnd1 PathEnd,
	pathEnd2 PathEnd,
	hops1to2 []*PathEnd,
	hops2to1 []*PathEnd,
	metrics *PrometheusMetrics,
	memo string,
	clientUpdateThresholdTime time.Duration,
	flushInterval time.Duration,
) *PathProcessor {
	if flushInterval == 0 {
		// "disable" periodic flushing by using a large value.
		flushInterval = 200 * 24 * 365 * time.Hour
	}
	hopsPathEnd1to2 := make([]*pathEndRuntime, len(hops1to2))
	for i, hop := range hops1to2 {
		hopsPathEnd1to2[i] = newPathEndRuntime(log, *hop, metrics)
	}
	hopsPathEnd2to1 := make([]*pathEndRuntime, len(hops2to1))
	for i, hop := range hops1to2 {
		hopsPathEnd2to1[i] = newPathEndRuntime(log, *hop, metrics)
	}
	return &PathProcessor{
		log:                       log,
		pathEnd1:                  newPathEndRuntime(log, pathEnd1, metrics),
		pathEnd2:                  newPathEndRuntime(log, pathEnd2, metrics),
		hopsPathEnd1to2:           hopsPathEnd1to2,
		hopsPathEnd2to1:           hopsPathEnd2to1,
		retryProcess:              make(chan struct{}, 2),
		memo:                      memo,
		clientUpdateThresholdTime: clientUpdateThresholdTime,
		flushInterval:             flushInterval,
		metrics:                   metrics,
	}
}

func (pp *PathProcessor) SetMessageLifecycle(messageLifecycle MessageLifecycle) {
	pp.messageLifecycle = messageLifecycle
}

// counterpartyPathEnds returns the pathEnds that are the counterparty to the given pathEnd. In the multihop case
// the src and dst chains have 2 counterparties:
// - the next/previous hop for clients and connections
// - the dst/src chain for channels and packets
func (pp *PathProcessor) counterpartyPathEnds(pathEnd *pathEndRuntime) []*pathEndRuntime {
	if pathEnd == pp.pathEnd1 {
		counterparty := []*pathEndRuntime{pp.pathEnd2}
		if len(pp.hopsPathEnd2to1) > 0 {
			counterparty = append(counterparty, pp.hopsPathEnd2to1[0])
		}
		return counterparty
	} else if pathEnd == pp.pathEnd2 {
		counterparty := []*pathEndRuntime{pp.pathEnd1}
		if len(pp.hopsPathEnd1to2) > 0 {
			counterparty = append(counterparty, pp.hopsPathEnd2to1[len(pp.hopsPathEnd1to2)-1])
		}
		return counterparty
	}
	for i, p := range pp.hopsPathEnd1to2 {
		if p == pathEnd {
			counterparty := pp.pathEnd2
			if i < len(pp.hopsPathEnd1to2)-1 {
				counterparty = pp.hopsPathEnd2to1[i+1]
			}
			return []*pathEndRuntime{counterparty}
		}
	}
	for i, p := range pp.hopsPathEnd2to1 {
		if p == pathEnd {
			counterparty := pp.pathEnd1
			if i > 0 {
				counterparty = pp.hopsPathEnd2to1[i-1]
			}
			return []*pathEndRuntime{counterparty}
		}
	}
	return nil
}

func (pp *PathProcessor) incomingCacheDataAvailable() bool {
	if len(pp.pathEnd1.incomingCacheData) > 0 || len(pp.pathEnd2.incomingCacheData) > 0 {
		return true
	}
	for _, hop := range pp.hopsPathEnd1to2 {
		if len(hop.incomingCacheData) > 0 {
			return true
		}
	}
	for _, hop := range pp.hopsPathEnd2to1 {
		if len(hop.incomingCacheData) > 0 {
			return true
		}
	}
	return false
}

func (pp *PathProcessor) pathEndsInSync() bool {
	if !pp.pathEnd1.inSync || !pp.pathEnd2.inSync {
		return false
	}
	/* TODO: is this ok?
	for _, hop := range pp.hopsPathEnd1to2 {
		if !hop.inSync {
			return false
		}
	}
	for _, hop := range pp.hopsPathEnd2to1 {
		if !hop.inSync {
			return false
		}
	}
	*/
	return true
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd1Messages(channelKey ChannelKey, message string) PacketSequenceCache {
	return pp.pathEnd1.messageCache.PacketFlow[channelKey][message]
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd2Messages(channelKey ChannelKey, message string) PacketSequenceCache {
	return pp.pathEnd2.messageCache.PacketFlow[channelKey][message]
}

type channelPair struct {
	pathEnd1ChannelKey ChannelKey
	pathEnd2ChannelKey ChannelKey
}

// RelevantClientID returns the relevant client ID or panics
func (pp *PathProcessor) RelevantClientIDs(chainID string) []string {
	clientIDs := []string{}
	if pp.pathEnd1.info.ChainID == chainID {
		clientIDs = append(clientIDs, pp.pathEnd1.info.ClientID)
	}
	if pp.pathEnd2.info.ChainID == chainID {
		clientIDs = append(clientIDs, pp.pathEnd2.info.ClientID)
	}
	for _, hop := range pp.hopsPathEnd1to2 {
		if hop.info.ChainID == chainID {
			clientIDs = append(clientIDs, hop.info.ClientID)
		}
	}
	// TODO: do we need both directions for this?
	for _, hop := range pp.hopsPathEnd2to1 {
		if hop.info.ChainID == chainID {
			clientIDs = append(clientIDs, hop.info.ClientID)
		}
	}
	if len(clientIDs) > 0 {
		return clientIDs
	}
	panic(fmt.Errorf("no relevant client ID for chain ID: %s", chainID))
}

// OnConnectionMessage allows the caller to handle connection handshake messages with a callback.
func (pp *PathProcessor) OnConnectionMessage(chainID string, eventType string, onMsg func(provider.ConnectionInfo)) {
	if pp.pathEnd1.info.ChainID == chainID {
		pp.pathEnd1.connSubscribers[eventType] = append(pp.pathEnd1.connSubscribers[eventType], onMsg)
	} else if pp.pathEnd2.info.ChainID == chainID {
		pp.pathEnd2.connSubscribers[eventType] = append(pp.pathEnd2.connSubscribers[eventType], onMsg)
	}
}

func (pp *PathProcessor) channelPairs() []channelPair {
	// Channel keys are from pathEnd1's perspective
	channels := ChannelStateCache{}
	for k, state := range pp.pathEnd1.channelStateCache {
		channels.Set(k, state)
	}
	for k, open := range pp.pathEnd2.channelStateCache {
		channels[k.Counterparty()] = open
	}
	pairs := make([]channelPair, len(channels))
	i := 0
	for k, _ := range channels {
		pairs[i] = channelPair{
			pathEnd1ChannelKey: k,
			pathEnd2ChannelKey: k.Counterparty(),
		}
		i++
	}
	return pairs
}

// Path Processors are constructed before ChainProcessors, so reference needs to be added afterwards
// This can be done inside the ChainProcessor constructor for simplification
func (pp *PathProcessor) SetChainProviderIfApplicable(chainProvider provider.ChainProvider) bool {
	if chainProvider == nil {
		return false
	}
	if pp.pathEnd1.info.ChainID == chainProvider.ChainId() {
		pp.pathEnd1.chainProvider = chainProvider
		return true
	} else if pp.pathEnd2.info.ChainID == chainProvider.ChainId() {
		pp.pathEnd2.chainProvider = chainProvider
		return true
	}
	var found bool
	for _, pathEnd := range pp.hopsPathEnd1to2 {
		if pathEnd.info.ChainID == chainProvider.ChainId() {
			pathEnd.chainProvider = chainProvider
			found = true
		}
	}
	for _, pathEnd := range pp.hopsPathEnd2to1 {
		if pathEnd.info.ChainID == chainProvider.ChainId() {
			pathEnd.chainProvider = chainProvider
			found = true
		}
	}
	return found
}

func (pp *PathProcessor) IsRelayedChannel(chainID string, channelKey ChannelKey) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ShouldRelayChannel(ChainChannelKey{ChainID: chainID, CounterpartyChainID: pp.pathEnd2.info.ChainID, ChannelKey: channelKey})
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ShouldRelayChannel(ChainChannelKey{ChainID: chainID, CounterpartyChainID: pp.pathEnd1.info.ChainID, ChannelKey: channelKey})
	}
	return false
}

func (pp *PathProcessor) IsRelevantClient(chainID string, clientID string) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ClientID == clientID
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ClientID == clientID
	}
	return false
}

func (pp *PathProcessor) IsRelevantConnection(chainID string, connectionID string) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.isRelevantConnection(connectionID)
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.isRelevantConnection(connectionID)
	}
	return false
}

func (pp *PathProcessor) IsRelevantChannel(chainID string, channelID string) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.isRelevantChannel(channelID)
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.isRelevantChannel(channelID)
	}
	return false
}

// ProcessBacklogIfReady gives ChainProcessors a way to trigger the path processor process
// as soon as they are in sync for the first time, even if they do not have new messages.
func (pp *PathProcessor) ProcessBacklogIfReady() {
	select {
	case pp.retryProcess <- struct{}{}:
		// All good.
	default:
		// Log that the channel is saturated;
		// something is wrong if we are retrying this quickly.
		pp.log.Error("Failed to enqueue path processor retry, retries already scheduled")
	}
}

// ChainProcessors call this method when they have new IBC messages
func (pp *PathProcessor) HandleNewData(chainID, clientID string, cacheData ChainProcessorCacheData) {
	if pp.pathEnd1.info.ChainID == chainID {
		pp.pathEnd1.incomingCacheData <- cacheData
	} else if pp.pathEnd2.info.ChainID == chainID {
		pp.pathEnd2.incomingCacheData <- cacheData
	} else {
		for _, pathEnd := range pp.hopsPathEnd1to2 {
			if pathEnd.info.ChainID == chainID && pathEnd.info.ClientID == clientID {
				pathEnd.incomingCacheData <- cacheData
				return
			}
		}
		for _, pathEnd := range pp.hopsPathEnd2to1 {
			if pathEnd.info.ChainID == chainID && pathEnd.info.ClientID == clientID {
				pathEnd.incomingCacheData <- cacheData
				return
			}
		}
	}
}

// processAvailableSignals will block if signals are not yet available, otherwise it will process one of the available signals.
// It returns whether the pathProcessor should quit.
func (pp *PathProcessor) processAvailableSignals(ctx context.Context, cancel func()) bool {
	cases := make([]reflect.SelectCase, 3)
	doneIndex := 0
	retryIndex := 1
	flushIndex := 2
	cases[doneIndex] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}
	cases[retryIndex] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(pp.retryProcess)}
	cases[flushIndex] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(pp.flushTicker.C)}
	pathEndOffset := len(cases)
	pathEnds := []*pathEndRuntime{pp.pathEnd1, pp.pathEnd2}
	pathEnds = append(pathEnds, pp.hopsPathEnd1to2...)
	pathEnds = append(pathEnds, pp.hopsPathEnd2to1...)
	for _, pathEnd := range pathEnds {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(pathEnd.incomingCacheData)})
	}
	chosen, value, _ := reflect.Select(cases)
	switch chosen {
	case doneIndex:
		pp.log.Debug("Context done, quitting PathProcessor",
			zap.String("chain_id_1", pp.pathEnd1.info.ChainID),
			zap.String("chain_id_2", pp.pathEnd2.info.ChainID),
			zap.String("client_id_1", pp.pathEnd1.info.ClientID),
			zap.String("client_id_2", pp.pathEnd2.info.ClientID),
			zap.Int("hops", len(pp.hopsPathEnd1to2)),
			zap.Error(ctx.Err()),
		)
		return true
	case retryIndex:
		// No new data to merge in, just retry handling.
	case flushIndex:
		// Periodic flush to clear out any old packets
		pp.flush(ctx)
	default:
		// Find the pathEnd that sent the signal
		pathEnd := pathEnds[chosen-pathEndOffset]
		counterpartyPathEnds := pp.counterpartyPathEnds(pathEnd)
		for _, counterparty := range counterpartyPathEnds {
			pathEnd.mergeCacheData(ctx, cancel, value.Interface().(ChainProcessorCacheData),
				counterparty.info.ChainID, counterparty.inSync, pp.messageLifecycle, counterparty)
		}
	}
	return false
}

// Run executes the main path process.
func (pp *PathProcessor) Run(ctx context.Context, cancel func()) {
	var retryTimer *time.Timer

	pp.flushTicker = time.NewTicker(pp.flushInterval)
	defer pp.flushTicker.Stop()

	for {
		// block until we have any signals to process
		if pp.processAvailableSignals(ctx, cancel) {
			return
		}

		for pp.incomingCacheDataAvailable() || len(pp.retryProcess) > 0 {
			// signals are available, so this will not need to block.
			if pp.processAvailableSignals(ctx, cancel) {
				return
			}
		}

		if !pp.pathEndsInSync() {
			continue
		}

		if !pp.initialFlushComplete {
			pp.flush(ctx)
			pp.initialFlushComplete = true
		} else if pp.shouldTerminateForFlushComplete(ctx, cancel) {
			cancel()
			return
		}

		// Process latest message cache state from all pathEnds
		if len(pp.hopsPathEnd1to2) > 0 {
			lastHop := pp.pathEnd1
			for i, hop := range append(pp.hopsPathEnd2to1, pp.pathEnd2) {
				// TODO: only do clients and connections here
				if err := pp.processLatestMessages(ctx, lastHop, hop); err != nil {
					// in case of IBC message send errors, schedule retry after durationErrorRetry
					if retryTimer != nil {
						retryTimer.Stop()
					}
					if ctx.Err() == nil {
						retryTimer = time.AfterFunc(durationErrorRetry, pp.ProcessBacklogIfReady)
					}
				}
				if i < len(pp.hopsPathEnd1to2) {
					lastHop = pp.hopsPathEnd1to2[i]
				}
			}
		}
		// TODO: only do channels and packets here
		if err := pp.processLatestMessages(ctx, pp.pathEnd1, pp.pathEnd2); err != nil {
			// in case of IBC message send errors, schedule retry after durationErrorRetry
			if retryTimer != nil {
				retryTimer.Stop()
			}
			if ctx.Err() == nil {
				retryTimer = time.AfterFunc(durationErrorRetry, pp.ProcessBacklogIfReady)
			}
		}
	}
}
