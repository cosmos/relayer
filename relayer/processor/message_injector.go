package processor

import (
	"context"
	"errors"
	"sync"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ MessageSender = (*MessageInjector)(nil)

// MessageInjector is used for broadcasting IBC messages to an RPC endpoint.
type MessageInjector struct {
	log     *zap.Logger
	metrics *PrometheusMetrics

	memo string

	cachedUpdateClient provider.RelayerMessage
	cachedTrackers     []messageToTrack
	mu                 sync.Mutex
}

func NewMessageInjector(
	log *zap.Logger,
	metrics *PrometheusMetrics,
	memo string,
) *MessageInjector {
	return &MessageInjector{
		log:     log,
		metrics: metrics,
		memo:    memo,
	}
}

// trackAndSendMessages will increment attempt counters for each message and send each message.
// Messages will be batched if the broadcast mode is configured to 'batch' and there was not an error
// in a previous batch.
func (mi *MessageInjector) trackAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	msgUpdateClient provider.RelayerMessage,
	trackers []messageToTrack,
	needsClientUpdate bool,
) error {
	mi.mu.Lock()
	defer mi.mu.Unlock()

	mi.cachedUpdateClient = nil

	//broadcastBatch := dst.chainProvider.ProviderConfig().BroadcastMode() == provider.BroadcastModeBatch
	var batch []messageToTrack

	assembledCount := 0

	for _, t := range trackers {
		msg := t.assembledMsg()
		if msg != nil {
			batch = append(batch, t)
			assembledCount++
		}
	}

	mi.cachedTrackers = batch

	if assembledCount > 0 ||
		(needsClientUpdate && msgUpdateClient != nil) {
		mi.cachedUpdateClient = msgUpdateClient
		return nil
	}

	if len(trackers) > 0 {
		return errors.New("all messages failed to assemble")
	}

	return nil
}

// InjectMsgs returns relay messages ready to inject into a proposal.
func (mi *MessageInjector) InjectMsgs(ctx context.Context) []provider.RelayerMessage {
	mi.mu.Lock()
	defer mi.mu.Unlock()

	var msgs []provider.RelayerMessage

	if mi.cachedUpdateClient != nil {
		msgs = append(msgs, mi.cachedUpdateClient)
	}

	for _, t := range mi.cachedTrackers {
		msg := t.assembledMsg()
		if msg != nil {
			msgs = append(msgs, msg)
		}
	}

	if len(msgs) == 0 {
		return nil
	}

	mi.log.Info("Injecting messages", zap.Int("count", len(msgs)))

	return msgs
}
