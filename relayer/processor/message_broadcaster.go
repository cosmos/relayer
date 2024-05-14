package processor

import (
	"context"
	"errors"
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ MessageSender = (*MessageBroadcaster)(nil)

// MessageBroadcaster is used for broadcasting IBC messages to an RPC endpoint.
type MessageBroadcaster struct {
	log     *zap.Logger
	metrics *PrometheusMetrics

	memo string
}

func NewMessageBroadcaster(
	log *zap.Logger,
	metrics *PrometheusMetrics,
	memo string,
) *MessageBroadcaster {
	return &MessageBroadcaster{
		log:     log,
		metrics: metrics,
		memo:    memo,
	}
}

// trackAndSendMessages will increment attempt counters for each message and send each message.
// Messages will be batched if the broadcast mode is configured to 'batch' and there was not an error
// in a previous batch.
func (mb *MessageBroadcaster) trackAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	msgUpdateClient provider.RelayerMessage,
	trackers []messageToTrack,
	needsClientUpdate bool,
) error {
	broadcastBatch := dst.chainProvider.ProviderConfig().BroadcastMode() == provider.BroadcastModeBatch
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

		if broadcastBatch && (retries == 0 || ordered) {
			batch = append(batch, t)
			continue
		}
		go mb.sendSingleMessage(ctx, msgUpdateClient, src, dst, t)
	}

	if len(batch) > 0 {
		go mb.sendBatchMessages(ctx, msgUpdateClient, src, dst, batch)
	}

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
		go mb.sendClientUpdate(ctx, msgUpdateClient, src, dst)
		return nil
	}

	// only msgUpdateClient, don't need to send
	return errors.New("all messages failed to assemble")
}

// sendClientUpdate will send an isolated client update message.
func (mb *MessageBroadcaster) sendClientUpdate(
	ctx context.Context,
	msgUpdateClient provider.RelayerMessage,
	src, dst *pathEndRuntime,
) {
	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	dst.log.Debug("Will relay client update")

	dst.lastClientUpdateHeightMu.Lock()
	dst.lastClientUpdateHeight = dst.latestBlock.Height
	dst.lastClientUpdateHeightMu.Unlock()

	msgs := []provider.RelayerMessage{msgUpdateClient}

	if err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mb.memo, ctx, nil); err != nil {
		mb.log.Error("Error sending client update message",
			zap.String("path_name", src.info.PathName),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Error(err),
		)

		mb.metricParseTxFailureCatagory(err, src)
		return
	}
	dst.log.Debug("Client update broadcast completed")
}

// sendBatchMessages will send a batch of messages,
// then increment metrics counters for successful packet messages.
func (mb *MessageBroadcaster) sendBatchMessages(
	ctx context.Context,
	msgUpdateClient provider.RelayerMessage,
	src, dst *pathEndRuntime,
	batch []messageToTrack,
) {
	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	var (
		msgs   []provider.RelayerMessage
		fields []zapcore.Field
	)

	if msgUpdateClient == nil {
		msgs = make([]provider.RelayerMessage, len(batch))
		for i, t := range batch {
			msgs[i] = t.assembledMsg()
			fields = append(fields, zap.Object(fmt.Sprintf("msg_%d", i), t))
		}
	} else {
		// messages are batch with appended MsgUpdateClient
		msgs = make([]provider.RelayerMessage, 1+len(batch))
		msgs[0] = msgUpdateClient

		for i, t := range batch {
			msgs[i+1] = t.assembledMsg()
			fields = append(fields, zap.Object(fmt.Sprintf("msg_%d", i), t))
		}
	}

	dst.log.Debug("Will relay messages", fields...)

	callback := func(_ *provider.RelayerTxResponse, err error) {
		for _, t := range batch {
			dst.finishedProcessing <- t
		}
		// only increment metrics counts for successful packets
		if err != nil || mb.metrics == nil {
			return
		}
		for _, tracker := range batch {
			t, ok := tracker.(packetMessageToTrack)
			if !ok {
				continue
			}
			var channel, port string
			if t.msg.eventType == chantypes.EventTypeRecvPacket {
				channel = t.msg.info.DestChannel
				port = t.msg.info.DestPort
			} else {
				channel = t.msg.info.SourceChannel
				port = t.msg.info.SourcePort
			}
			mb.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, t.msg.eventType)
		}
	}
	callbacks := []func(rtr *provider.RelayerTxResponse, err error){callback}

	//During testing, this adds a callback so our test case can inspect the TX results
	if PathProcMessageCollector != nil {
		testCallback := func(rtr *provider.RelayerTxResponse, err error) {
			msgResult := &PathProcessorMessageResp{
				DestinationChain: dst.chainProvider,
				Response:         rtr,
				SuccessfulTx:     err == nil,
				Error:            err,
			}
			PathProcMessageCollector <- msgResult
		}
		callbacks = append(callbacks, testCallback)
	}

	if err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mb.memo, ctx, callbacks); err != nil {
		for _, t := range batch {
			dst.finishedProcessing <- t
		}
		errFields := []zapcore.Field{
			zap.String("path_name", src.info.PathName),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Error(err),
		}

		mb.metricParseTxFailureCatagory(err, src)

		if errors.Is(err, chantypes.ErrRedundantTx) {
			mb.log.Debug("Redundant message(s)", errFields...)
			return
		}
		mb.log.Error("Error sending messages", errFields...)
		return
	}
	dst.log.Debug("Message broadcast completed", fields...)
}

// sendSingleMessage will send an isolated message.
func (mb *MessageBroadcaster) sendSingleMessage(
	ctx context.Context,
	msgUpdateClient provider.RelayerMessage,
	src, dst *pathEndRuntime,
	tracker messageToTrack,
) {
	var msgs []provider.RelayerMessage

	if msgUpdateClient == nil {
		msgs = []provider.RelayerMessage{tracker.assembledMsg()}
	} else {
		msgs = []provider.RelayerMessage{msgUpdateClient, tracker.assembledMsg()}
	}

	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	msgType := tracker.msgType()

	dst.log.Debug(fmt.Sprintf("Will broadcast %s message", msgType), zap.Object("msg", tracker))

	// Set callback for packet messages so that we increment prometheus metrics on successful relays.
	callbacks := []func(rtr *provider.RelayerTxResponse, err error){}

	callback := func(_ *provider.RelayerTxResponse, err error) {
		dst.finishedProcessing <- tracker

		t, ok := tracker.(packetMessageToTrack)
		if !ok {
			return
		}
		// only increment metrics counts for successful packets
		if err != nil || mb.metrics == nil {
			return
		}
		var channel, port string
		if t.msg.eventType == chantypes.EventTypeRecvPacket {
			channel = t.msg.info.DestChannel
			port = t.msg.info.DestPort
		} else {
			channel = t.msg.info.SourceChannel
			port = t.msg.info.SourcePort
		}
		mb.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, t.msg.eventType)
	}

	callbacks = append(callbacks, callback)

	//During testing, this adds a callback so our test case can inspect the TX results
	if PathProcMessageCollector != nil {
		testCallback := func(rtr *provider.RelayerTxResponse, err error) {
			msgResult := &PathProcessorMessageResp{
				DestinationChain: dst.chainProvider,
				Response:         rtr,
				SuccessfulTx:     err == nil,
				Error:            err,
			}
			PathProcMessageCollector <- msgResult
		}
		callbacks = append(callbacks, testCallback)
	}

	err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mb.memo, ctx, callbacks)
	if err != nil {
		dst.finishedProcessing <- tracker
		errFields := []zapcore.Field{
			zap.String("path_name", src.info.PathName),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
		}

		mb.metricParseTxFailureCatagory(err, src)

		errFields = append(errFields, zap.Object("msg", tracker))
		errFields = append(errFields, zap.Error(err))
		if errors.Is(err, chantypes.ErrRedundantTx) {
			mb.log.Debug(fmt.Sprintf("Redundant %s message", msgType), errFields...)
			return
		}
		mb.log.Error(fmt.Sprintf("Error broadcasting %s message", msgType), errFields...)
		return
	}

	dst.log.Debug(fmt.Sprintf("Successfully broadcasted %s message", msgType), zap.Object("msg", tracker))
}

func (mb *MessageBroadcaster) metricParseTxFailureCatagory(err error, src *pathEndRuntime) {
	if mb.metrics == nil {
		return
	}

	for _, promError := range promErrorCatagories {
		if errors.Is(err, promError) {
			mb.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, promError.Error())
			return
		}
	}
	mb.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, "Tx Failure")
}
