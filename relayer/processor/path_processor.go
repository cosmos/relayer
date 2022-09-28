package processor

import (
	"context"
	"fmt"
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

	memo string

	clientUpdateThresholdTime time.Duration

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
	metrics *PrometheusMetrics,
	memo string,
	clientUpdateThresholdTime time.Duration,
) *PathProcessor {
	return &PathProcessor{
		log:                       log,
		pathEnd1:                  newPathEndRuntime(log, pathEnd1, metrics),
		pathEnd2:                  newPathEndRuntime(log, pathEnd2, metrics),
		retryProcess:              make(chan struct{}, 2),
		memo:                      memo,
		clientUpdateThresholdTime: clientUpdateThresholdTime,
		metrics:                   metrics,
	}
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
func (pp *PathProcessor) RelevantClientID(chainID string) string {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ClientID
	}
	if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ClientID
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
	channels := make(map[ChannelKey]bool)
	for k, open := range pp.pathEnd1.channelStateCache {
		channels[k] = open
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
	return false
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
func (pp *PathProcessor) HandleNewData(chainID string, cacheData ChainProcessorCacheData) {
	if pp.pathEnd1.info.ChainID == chainID {
		pp.pathEnd1.incomingCacheData <- cacheData
	} else if pp.pathEnd2.info.ChainID == chainID {
		pp.pathEnd2.incomingCacheData <- cacheData
	}
}

// processAvailableSignals will block if signals are not yet available, otherwise it will process one of the available signals.
// It returns whether or not the pathProcessor should quit.
func (pp *PathProcessor) processAvailableSignals(ctx context.Context, cancel func(), messageLifecycle MessageLifecycle) bool {
	select {
	case <-ctx.Done():
		pp.log.Debug("Context done, quitting PathProcessor",
			zap.String("chain_id_1", pp.pathEnd1.info.ChainID),
			zap.String("chain_id_2", pp.pathEnd2.info.ChainID),
			zap.String("client_id_1", pp.pathEnd1.info.ClientID),
			zap.String("client_id_2", pp.pathEnd2.info.ClientID),
			zap.Error(ctx.Err()),
		)
		return true
	case d := <-pp.pathEnd1.incomingCacheData:
		// we have new data from ChainProcessor for pathEnd1
		pp.pathEnd1.mergeCacheData(ctx, cancel, d, pp.pathEnd2.info.ChainID, pp.pathEnd2.inSync, messageLifecycle, pp.pathEnd2)

	case d := <-pp.pathEnd2.incomingCacheData:
		// we have new data from ChainProcessor for pathEnd2
		pp.pathEnd2.mergeCacheData(ctx, cancel, d, pp.pathEnd1.info.ChainID, pp.pathEnd1.inSync, messageLifecycle, pp.pathEnd1)

	case <-pp.retryProcess:
		// No new data to merge in, just retry handling.
	}
	return false
}

// Run executes the main path process.
func (pp *PathProcessor) Run(ctx context.Context, cancel func(), messageLifecycle MessageLifecycle) {
	var retryTimer *time.Timer
	for {
		// block until we have any signals to process
		if pp.processAvailableSignals(ctx, cancel, messageLifecycle) {
			return
		}

		for len(pp.pathEnd1.incomingCacheData) > 0 || len(pp.pathEnd2.incomingCacheData) > 0 || len(pp.retryProcess) > 0 {
			// signals are available, so this will not need to block.
			if pp.processAvailableSignals(ctx, cancel, messageLifecycle) {
				return
			}
		}

		if !pp.pathEnd1.inSync || !pp.pathEnd2.inSync {
			continue
		}

		// process latest message cache state from both pathEnds
		if err := pp.processLatestMessages(ctx, messageLifecycle); err != nil {
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
