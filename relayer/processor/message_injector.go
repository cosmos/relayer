package processor

import (
	"context"
	"errors"
	"sync"

	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var _ MessageSender = (*MessageInjector)(nil)

// MessageInjector is used for broadcasting IBC messages to an RPC endpoint.
type MessageInjector struct {
	log     *zap.Logger
	metrics *PrometheusMetrics

	memo string

	dst *pathEndRuntime

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
func (mb *MessageInjector) trackAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	msgUpdateClient provider.RelayerMessage,
	trackers []messageToTrack,
	needsClientUpdate bool,
) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.cachedUpdateClient = msgUpdateClient

	mb.dst = dst

	//broadcastBatch := dst.chainProvider.ProviderConfig().BroadcastMode() == provider.BroadcastModeBatch
	var batch []messageToTrack

	for _, t := range trackers {
		retries := dst.trackProcessingMessage(t)
		if t.assembledMsg() == nil {
			dst.trackFinishedProcessingMessage(t)
			continue
		}

		ordered := false
		if m, ok := t.(packetMessageToTrack); ok && m.msg.info.ChannelOrder == chantypes.ORDERED.String() {
			ordered = true
		}

		//if broadcastBatch && (retries == 0 || ordered) {
		if retries == 0 || ordered {
			batch = append(batch, t)
			continue
		}
		//go mb.sendSingleMessage(ctx, src, dst, t)
	}

	mb.cachedTrackers = batch

	assembledCount := 0
	for _, m := range trackers {
		if m.assembledMsg() != nil {
			assembledCount++
		}
	}

	if assembledCount > 0 {
		return nil
	}

	if needsClientUpdate && msgUpdateClient != nil {
		return nil
	}

	// only msgUpdateClient, don't need to send
	return errors.New("all messages failed to assemble")
}

// InjectMsgs returns relay messages ready to inject into a proposal.
func (mb *MessageInjector) InjectMsgs(ctx context.Context) []provider.RelayerMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	var msgs []provider.RelayerMessage

	if mb.cachedUpdateClient != nil {
		msgs = append(msgs, mb.cachedUpdateClient)
	}

	for _, t := range mb.cachedTrackers {
		msg := t.assembledMsg()
		if msg != nil {
			msgs = append(msgs, msg)
		}
		mb.dst.finishedProcessing <- t
	}

	return msgs
}
