package processor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// messageProcessor is used for concurrent IBC message assembly and sending
type messageProcessor struct {
	log     *zap.Logger
	metrics *PrometheusMetrics

	memo string

	msgUpdateClient           provider.RelayerMessage
	clientUpdateThresholdTime time.Duration

	pktMsgs       []packetMessageToTrack
	connMsgs      []connectionMessageToTrack
	chanMsgs      []channelMessageToTrack
	clientICQMsgs []clientICQMessageToTrack

	isLocalhost bool
}

// catagories of tx errors for a Prometheus counter. If the error doesnt fall into one of the below categories, it is labeled as "Tx Failure"
var promErrorCatagories = []error{chantypes.ErrRedundantTx, sdkerrors.ErrInsufficientFunds, sdkerrors.ErrInvalidCoins, sdkerrors.ErrOutOfGas, sdkerrors.ErrWrongSequence}

// trackMessage stores the message tracker in the correct slice and index based on the type.
func (mp *messageProcessor) trackMessage(tracker messageToTrack, i int) {
	switch t := tracker.(type) {
	case packetMessageToTrack:
		mp.pktMsgs[i] = t
	case channelMessageToTrack:
		mp.chanMsgs[i] = t
	case connectionMessageToTrack:
		mp.connMsgs[i] = t
	case clientICQMessageToTrack:
		mp.clientICQMsgs[i] = t
	}
}

// trackers returns all of the msg trackers for the current set of messages to be sent.
func (mp *messageProcessor) trackers() (trackers []messageToTrack) {
	for _, t := range mp.pktMsgs {
		trackers = append(trackers, t)
	}
	for _, t := range mp.chanMsgs {
		trackers = append(trackers, t)
	}
	for _, t := range mp.connMsgs {
		trackers = append(trackers, t)
	}
	for _, t := range mp.clientICQMsgs {
		trackers = append(trackers, t)
	}
	return trackers
}

func newMessageProcessor(
	log *zap.Logger,
	metrics *PrometheusMetrics,
	memo string,
	clientUpdateThresholdTime time.Duration,
	isLocalhost bool,
) *messageProcessor {
	return &messageProcessor{
		log:                       log,
		metrics:                   metrics,
		memo:                      memo,
		clientUpdateThresholdTime: clientUpdateThresholdTime,
		isLocalhost:               isLocalhost,
	}
}

// processMessages is the entrypoint for the message processor.
// it will assemble and send any pending messages.
func (mp *messageProcessor) processMessages(
	ctx context.Context,
	messages pathEndMessages,
	src, dst *pathEndRuntime,
) error {
	var needsClientUpdate bool

	// Localhost IBC does not permit client updates
	if src.clientState.ClientID != ibcexported.LocalhostClientID && dst.clientState.ClientID != ibcexported.LocalhostConnectionID {
		var err error
		needsClientUpdate, err = mp.shouldUpdateClientNow(ctx, src, dst)
		if err != nil {
			return err
		}

		if err := mp.assembleMsgUpdateClient(ctx, src, dst); err != nil {
			return err
		}
	}

	mp.assembleMessages(ctx, messages, src, dst)

	return mp.trackAndSendMessages(ctx, src, dst, needsClientUpdate)
}

// shouldUpdateClientNow determines if an update client message should be sent
// even if there are no messages to be sent now. It will not be attempted if
// there has not been enough blocks since the last client update attempt.
// Otherwise, it will be attempted if either 2/3 of the trusting period
// or the configured client update threshold duration has passed.
func (mp *messageProcessor) shouldUpdateClientNow(ctx context.Context, src, dst *pathEndRuntime) (bool, error) {
	var consensusHeightTime time.Time
	if dst.clientState.ConsensusTime.IsZero() {
		h, err := src.chainProvider.QueryIBCHeader(ctx, int64(dst.clientState.ConsensusHeight.RevisionHeight))
		if err != nil {
			return false, fmt.Errorf("failed to get header height: %w", err)
		}
		consensusHeightTime = time.Unix(0, int64(h.ConsensusState().GetTimestamp()))
	} else {
		consensusHeightTime = dst.clientState.ConsensusTime
	}

	clientUpdateThresholdMs := mp.clientUpdateThresholdTime.Milliseconds()

	dst.lastClientUpdateHeightMu.Lock()
	enoughBlocksPassed := (dst.latestBlock.Height - blocksToRetrySendAfter) > dst.lastClientUpdateHeight
	dst.lastClientUpdateHeightMu.Unlock()

	twoThirdsTrustingPeriodMs := float64(dst.clientState.TrustingPeriod.Milliseconds()) * 2 / 3
	timeSinceLastClientUpdateMs := float64(time.Since(consensusHeightTime).Milliseconds())

	pastTwoThirdsTrustingPeriod := dst.clientState.TrustingPeriod > 0 &&
		timeSinceLastClientUpdateMs > twoThirdsTrustingPeriodMs

	pastConfiguredClientUpdateThreshold := clientUpdateThresholdMs > 0 &&
		time.Since(consensusHeightTime).Milliseconds() > clientUpdateThresholdMs

	shouldUpdateClientNow := enoughBlocksPassed && (pastTwoThirdsTrustingPeriod || pastConfiguredClientUpdateThreshold)

	if mp.metrics != nil {
		timeToExpiration := dst.clientState.TrustingPeriod - time.Since(consensusHeightTime)
		mp.metrics.SetClientExpiration(src.info.PathName, dst.info.ChainID, dst.clientState.ClientID, fmt.Sprint(dst.clientState.TrustingPeriod.String()), timeToExpiration)
		mp.metrics.SetClientTrustingPeriod(src.info.PathName, dst.info.ChainID, dst.info.ClientID, time.Duration(dst.clientState.TrustingPeriod))
	}

	if shouldUpdateClientNow {
		mp.log.Info("Client update threshold condition met",
			zap.String("path_name", src.info.PathName),
			zap.String("chain_id", dst.info.ChainID),
			zap.String("client_id", dst.info.ClientID),
			zap.Int64("trusting_period", dst.clientState.TrustingPeriod.Milliseconds()),
			zap.Int64("time_since_client_update", time.Since(consensusHeightTime).Milliseconds()),
			zap.Int64("client_threshold_time", mp.clientUpdateThresholdTime.Milliseconds()),
		)
	}

	return shouldUpdateClientNow, nil
}

// assembleMessages will assemble all messages in parallel. This typically involves proof queries for each.
func (mp *messageProcessor) assembleMessages(ctx context.Context, messages pathEndMessages, src, dst *pathEndRuntime) {
	var wg sync.WaitGroup

	if !mp.isLocalhost {
		mp.connMsgs = make([]connectionMessageToTrack, len(messages.connectionMessages))
		for i, msg := range messages.connectionMessages {
			wg.Add(1)
			go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
		}
	}

	mp.chanMsgs = make([]channelMessageToTrack, len(messages.channelMessages))
	for i, msg := range messages.channelMessages {
		wg.Add(1)
		go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
	}

	if !mp.isLocalhost {
		mp.clientICQMsgs = make([]clientICQMessageToTrack, len(messages.clientICQMessages))
		for i, msg := range messages.clientICQMessages {
			wg.Add(1)
			go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
		}
	}

	mp.pktMsgs = make([]packetMessageToTrack, len(messages.packetMessages))
	for i, msg := range messages.packetMessages {
		wg.Add(1)
		go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
	}

	wg.Wait()
}

// assembledCount will return the number of assembled messages.
// This must be called after assembleMessages has completed.
func (mp *messageProcessor) assembledCount() int {
	count := 0
	for _, m := range mp.trackers() {
		if m.assembledMsg() != nil {
			count++
		}
	}

	return count
}

// assembleMessage will assemble a specific message based on it's type.
func (mp *messageProcessor) assembleMessage(
	ctx context.Context,
	msg ibcMessage,
	src, dst *pathEndRuntime,
	i int,
	wg *sync.WaitGroup,
) {
	assembled, err := msg.assemble(ctx, src, dst)
	mp.trackMessage(msg.tracker(assembled), i)
	wg.Done()
	if err != nil {
		dst.log.Error(fmt.Sprintf("Error assembling %s message", msg.msgType()),
			zap.Object("msg", msg),
			zap.Error(err),
		)
		return
	}
	dst.log.Debug(fmt.Sprintf("Assembled %s message", msg.msgType()), zap.Object("msg", msg))
}

// assembleMsgUpdateClient uses the ChainProvider from both pathEnds to assemble the client update header
// from the source and then assemble the update client message in the correct format for the destination.
func (mp *messageProcessor) assembleMsgUpdateClient(ctx context.Context, src, dst *pathEndRuntime) error {
	clientID := dst.info.ClientID
	clientConsensusHeight := dst.clientState.ConsensusHeight
	trustedConsensusHeight := dst.clientTrustedState.ClientState.ConsensusHeight
	var trustedNextValidatorsHash []byte
	if dst.clientTrustedState.IBCHeader != nil {
		trustedNextValidatorsHash = dst.clientTrustedState.IBCHeader.NextValidatorsHash()
	}

	// If the client state height is not equal to the client trusted state height and the client state height is
	// the latest block, we cannot send a MsgUpdateClient until another block is observed on the counterparty.
	// If the client state height is in the past, beyond ibcHeadersToCache, then we need to query for it.
	if !trustedConsensusHeight.EQ(clientConsensusHeight) {
		deltaConsensusHeight := int64(clientConsensusHeight.RevisionHeight) - int64(trustedConsensusHeight.RevisionHeight)
		if trustedConsensusHeight.RevisionHeight != 0 && deltaConsensusHeight <= clientConsensusHeightUpdateThresholdBlocks {
			return fmt.Errorf("observed client trusted height: %d does not equal latest client state height: %d",
				trustedConsensusHeight.RevisionHeight, clientConsensusHeight.RevisionHeight)
		}
		header, err := src.chainProvider.QueryIBCHeader(ctx, int64(clientConsensusHeight.RevisionHeight+1))
		if err != nil {
			return fmt.Errorf("error getting IBC header at height: %d for chain_id: %s, %w",
				clientConsensusHeight.RevisionHeight+1, src.info.ChainID, err)
		}
		mp.log.Debug("Had to query for client trusted IBC header",
			zap.String("path_name", src.info.PathName),
			zap.String("chain_id", src.info.ChainID),
			zap.String("counterparty_chain_id", dst.info.ChainID),
			zap.String("counterparty_client_id", clientID),
			zap.Uint64("height", clientConsensusHeight.RevisionHeight+1),
			zap.Uint64("latest_height", src.latestBlock.Height),
		)
		dst.clientTrustedState = provider.ClientTrustedState{
			ClientState: dst.clientState,
			IBCHeader:   header,
		}
		trustedConsensusHeight = clientConsensusHeight
		trustedNextValidatorsHash = header.NextValidatorsHash()
	}

	if src.latestHeader.Height() == trustedConsensusHeight.RevisionHeight &&
		!bytes.Equal(src.latestHeader.NextValidatorsHash(), trustedNextValidatorsHash) {
		return fmt.Errorf("latest header height is equal to the client trusted height: %d, "+
			"need to wait for next block's header before we can assemble and send a new MsgUpdateClient",
			trustedConsensusHeight.RevisionHeight)
	}

	msgUpdateClientHeader, err := src.chainProvider.MsgUpdateClientHeader(
		src.latestHeader,
		trustedConsensusHeight,
		dst.clientTrustedState.IBCHeader,
	)
	if err != nil {
		return fmt.Errorf("error assembling new client header: %w", err)
	}

	msgUpdateClient, err := dst.chainProvider.MsgUpdateClient(clientID, msgUpdateClientHeader)
	if err != nil {
		return fmt.Errorf("error assembling MsgUpdateClient: %w", err)
	}

	mp.msgUpdateClient = msgUpdateClient

	return nil
}

// trackAndSendMessages will increment attempt counters for each message and send each message.
// Messages will be batched if the broadcast mode is configured to 'batch' and there was not an error
// in a previous batch.
func (mp *messageProcessor) trackAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	needsClientUpdate bool,
) error {
	broadcastBatch := dst.chainProvider.ProviderConfig().BroadcastMode() == provider.BroadcastModeBatch
	var batch []messageToTrack

	for _, t := range mp.trackers() {

		retries := dst.trackProcessingMessage(t)
		if t.assembledMsg() == nil {
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
		go mp.sendSingleMessage(ctx, src, dst, t)
	}

	if len(batch) > 0 {
		go mp.sendBatchMessages(ctx, src, dst, batch)
	}

	if mp.assembledCount() > 0 {
		return nil
	}

	if needsClientUpdate {
		go mp.sendClientUpdate(ctx, src, dst)
		return nil
	}

	// only msgUpdateClient, don't need to send
	return errors.New("all messages failed to assemble")
}

// sendClientUpdate will send an isolated client update message.
func (mp *messageProcessor) sendClientUpdate(
	ctx context.Context,
	src, dst *pathEndRuntime,
) {
	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	dst.log.Debug("Will relay client update")

	dst.lastClientUpdateHeightMu.Lock()
	dst.lastClientUpdateHeight = dst.latestBlock.Height
	dst.lastClientUpdateHeightMu.Unlock()

	msgs := []provider.RelayerMessage{mp.msgUpdateClient}

	if err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, nil); err != nil {
		mp.log.Error("Error sending client update message",
			zap.String("path_name", src.info.PathName),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Error(err),
		)

		for _, promError := range promErrorCatagories {
			if mp.metrics != nil {
				if errors.Is(err, promError) {
					mp.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, promError.Error())
				} else {
					mp.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, "Tx Failure")
				}
			}
		}
		return
	}
	dst.log.Debug("Client update broadcast completed")
}

type PathProcessorMessageResp struct {
	Response         *provider.RelayerTxResponse
	DestinationChain provider.ChainProvider
	SuccessfulTx     bool
	Error            error
}

var PathProcMessageCollector chan *PathProcessorMessageResp

// sendBatchMessages will send a batch of messages,
// then increment metrics counters for successful packet messages.
func (mp *messageProcessor) sendBatchMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	batch []messageToTrack,
) {
	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	var (
		msgs   []provider.RelayerMessage
		fields []zapcore.Field
	)

	if mp.isLocalhost {
		msgs = make([]provider.RelayerMessage, len(batch))
		for i, t := range batch {
			msgs[i] = t.assembledMsg()
			fields = append(fields, zap.Object(fmt.Sprintf("msg_%d", i), t))
		}
	} else {
		// messages are batch with appended MsgUpdateClient
		msgs = make([]provider.RelayerMessage, 1+len(batch))
		msgs[0] = mp.msgUpdateClient
		for i, t := range batch {
			msgs[i+1] = t.assembledMsg()
			fields = append(fields, zap.Object(fmt.Sprintf("msg_%d", i), t))
		}
	}

	dst.log.Debug("Will relay messages", fields...)

	callback := func(_ *provider.RelayerTxResponse, err error) {
		// only increment metrics counts for successful packets
		if err != nil || mp.metrics == nil {
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
			mp.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, t.msg.eventType)
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

	if err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, callbacks); err != nil {
		errFields := []zapcore.Field{
			zap.String("path_name", src.info.PathName),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Error(err),
		}

		for _, promError := range promErrorCatagories {
			if mp.metrics != nil {
				if errors.Is(err, promError) {
					mp.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, promError.Error())
				} else {
					mp.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, "Tx Failure")
				}
			}
		}

		if errors.Is(err, chantypes.ErrRedundantTx) {
			mp.log.Debug("Redundant message(s)", errFields...)
			return
		}
		mp.log.Error("Error sending messages", errFields...)
		return
	}
	dst.log.Debug("Message broadcast completed", fields...)
}

// sendSingleMessage will send an isolated message.
func (mp *messageProcessor) sendSingleMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	tracker messageToTrack,
) {
	var msgs []provider.RelayerMessage

	if mp.isLocalhost {
		msgs = []provider.RelayerMessage{tracker.assembledMsg()}
	} else {
		msgs = []provider.RelayerMessage{mp.msgUpdateClient, tracker.assembledMsg()}
	}

	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	msgType := tracker.msgType()

	dst.log.Debug(fmt.Sprintf("Will broadcast %s message", msgType), zap.Object("msg", tracker))

	// Set callback for packet messages so that we increment prometheus metrics on successful relays.
	callbacks := []func(rtr *provider.RelayerTxResponse, err error){}
	if t, ok := tracker.(packetMessageToTrack); ok {
		callback := func(_ *provider.RelayerTxResponse, err error) {
			// only increment metrics counts for successful packets
			if err != nil || mp.metrics == nil {
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
			mp.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, t.msg.eventType)
		}

		callbacks = append(callbacks, callback)
	}

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

	err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, callbacks)
	if err != nil {
		errFields := []zapcore.Field{
			zap.String("path_name", src.info.PathName),
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
		}

		for _, promError := range promErrorCatagories {
			if mp.metrics != nil {
				if errors.Is(err, promError) {
					mp.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, promError.Error())
				} else {
					mp.metrics.IncTxFailure(src.info.PathName, src.info.ChainID, "Tx Failure")
				}
			}
		}

		errFields = append(errFields, zap.Object("msg", tracker))
		errFields = append(errFields, zap.Error(err))
		if errors.Is(err, chantypes.ErrRedundantTx) {
			mp.log.Debug(fmt.Sprintf("Redundant %s message", msgType), errFields...)
			return
		}
		mp.log.Error(fmt.Sprintf("Error broadcasting %s message", msgType), errFields...)
		return
	}

	dst.log.Debug(fmt.Sprintf("Successfully broadcasted %s message", msgType), zap.Object("msg", tracker))
}
