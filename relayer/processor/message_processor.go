package processor

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
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
}

func newMessageProcessor(
	log *zap.Logger,
	metrics *PrometheusMetrics,
	memo string,
	clientUpdateThresholdTime time.Duration,
) *messageProcessor {
	return &messageProcessor{
		log:                       log,
		metrics:                   metrics,
		memo:                      memo,
		clientUpdateThresholdTime: clientUpdateThresholdTime,
	}
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface
// so that you can use zap.Object("messages", r) when logging.
// This is typically useful when logging details about a partially sent result.
func (mp *messageProcessor) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for i, m := range mp.pktMsgs {
		pfx := "pkt_" + strconv.FormatInt(int64(i), 10) + "_"
		enc.AddString(pfx+"event_type", m.msg.eventType)
		enc.AddString(pfx+"src_chan", m.msg.info.SourceChannel)
		enc.AddString(pfx+"src_port", m.msg.info.SourcePort)
		enc.AddString(pfx+"dst_chan", m.msg.info.DestChannel)
		enc.AddString(pfx+"dst_port", m.msg.info.DestPort)
		enc.AddString(pfx+"data", string(m.msg.info.Data))
	}
	for i, m := range mp.connMsgs {
		pfx := "conn_" + strconv.FormatInt(int64(i), 10) + "_"
		enc.AddString(pfx+"event_type", m.msg.eventType)
		enc.AddString(pfx+"client_id", m.msg.info.ClientID)
		enc.AddString(pfx+"conn_id", m.msg.info.ConnID)
		enc.AddString(pfx+"cntrprty_client_id", m.msg.info.CounterpartyClientID)
		enc.AddString(pfx+"cntrprty_conn_id", m.msg.info.CounterpartyConnID)
	}
	for i, m := range mp.chanMsgs {
		pfx := "chan_" + strconv.FormatInt(int64(i), 10) + "_"
		enc.AddString(pfx+"event_type", m.msg.eventType)
		enc.AddString(pfx+"chan_id", m.msg.info.ChannelID)
		enc.AddString(pfx+"port_id", m.msg.info.PortID)
		enc.AddString(pfx+"cntrprty_chan_id", m.msg.info.CounterpartyChannelID)
		enc.AddString(pfx+"cntrprty_port_id", m.msg.info.CounterpartyPortID)
	}
	return nil
}

func (mp *messageProcessor) processMessages(
	ctx context.Context,
	messages pathEndMessages,
	src, dst *pathEndRuntime,
) error {
	needsClientUpdate, err := mp.shouldUpdateClientNow(ctx, src, dst)
	if err != nil {
		return err
	}

	if err := mp.assembleMsgUpdateClient(ctx, src, dst); err != nil {
		return err
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

	pastTwoThirdsTrustingPeriod := timeSinceLastClientUpdateMs > twoThirdsTrustingPeriodMs

	pastConfiguredClientUpdateThreshold := clientUpdateThresholdMs > 0 &&
		time.Since(consensusHeightTime).Milliseconds() > clientUpdateThresholdMs

	shouldUpdateClientNow := enoughBlocksPassed && (pastTwoThirdsTrustingPeriod || pastConfiguredClientUpdateThreshold)

	if shouldUpdateClientNow {
		mp.log.Info("Client update threshold condition met",
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

	mp.connMsgs = make([]connectionMessageToTrack, len(messages.connectionMessages))
	for i, msg := range messages.connectionMessages {
		wg.Add(1)
		go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
	}

	mp.chanMsgs = make([]channelMessageToTrack, len(messages.channelMessages))
	for i, msg := range messages.channelMessages {
		wg.Add(1)
		go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
	}

	mp.clientICQMsgs = make([]clientICQMessageToTrack, len(messages.clientICQMessages))
	for i, msg := range messages.clientICQMessages {
		wg.Add(1)
		go mp.assembleMessage(ctx, msg, src, dst, i, &wg)
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
	assembled := 0
	for _, m := range mp.connMsgs {
		if m.m != nil {
			assembled++
		}
	}
	for _, m := range mp.chanMsgs {
		if m.m != nil {
			assembled++
		}
	}
	for _, m := range mp.clientICQMsgs {
		if m.m != nil {
			assembled++
		}
	}
	for _, m := range mp.pktMsgs {
		if m.m != nil {
			assembled++
		}
	}

	return assembled
}

// assembleMessage will assemble a specific message based on it's type.
func (mp *messageProcessor) assembleMessage(
	ctx context.Context,
	msg ibcMessage,
	src, dst *pathEndRuntime,
	i int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	var message provider.RelayerMessage
	var err error
	switch m := msg.(type) {
	case packetIBCMessage:
		message, err = mp.assemblePacketMessage(ctx, m, src, dst)
		tracker := packetMessageToTrack{
			msg: m,
		}
		if err == nil {
			tracker.m = message
			mp.pktMsgs[i] = tracker
			dst.log.Debug("Assembled packet message",
				zap.String("event_type", m.eventType),
				zap.Uint64("sequence", m.info.Sequence),
				zap.String("src_channel", m.info.SourceChannel),
				zap.String("src_port", m.info.SourcePort),
				zap.String("dst_channel", m.info.DestChannel),
				zap.String("dst_port", m.info.DestPort),
			)
		}
	case connectionIBCMessage:
		message, err = mp.assembleConnectionMessage(ctx, m, src, dst)
		tracker := connectionMessageToTrack{msg: m}
		if err == nil {
			tracker.m = message
			mp.connMsgs[i] = tracker
			dst.log.Debug("Assembled connection message",
				zap.String("event_type", m.eventType),
				zap.String("connection_id", m.info.ConnID),
			)
		}
	case channelIBCMessage:
		message, err = mp.assembleChannelMessage(ctx, m, src, dst)
		tracker := channelMessageToTrack{msg: m}
		if err == nil {
			tracker.m = message
			mp.chanMsgs[i] = tracker
			dst.log.Debug("Assembled channel message",
				zap.String("event_type", m.eventType),
				zap.String("channel_id", m.info.ChannelID),
				zap.String("port_id", m.info.PortID),
			)
		}
	case clientICQMessage:
		message, err = mp.assembleClientICQMessage(ctx, m, src, dst)
		tracker := clientICQMessageToTrack{msg: m}
		if err == nil {
			tracker.m = message
			mp.clientICQMsgs[i] = tracker
			dst.log.Debug("Assembled ICQ message",
				zap.String("type", m.info.Type),
				zap.String("query_id", string(m.info.QueryID)),
			)
		}
	}
	if err != nil {
		mp.log.Error("Error assembling channel message", zap.Error(err))
		return
	}
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

// assemblePacketMessage executes the appropriate proof query function,
// then, if successful, assembles the message for the destination.
func (mp *messageProcessor) assemblePacketMessage(
	ctx context.Context,
	msg packetIBCMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var packetProof func(context.Context, provider.PacketInfo, uint64) (provider.PacketProof, error)
	var assembleMessage func(provider.PacketInfo, provider.PacketProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case chantypes.EventTypeRecvPacket:
		packetProof = src.chainProvider.PacketCommitment
		assembleMessage = dst.chainProvider.MsgRecvPacket
	case chantypes.EventTypeAcknowledgePacket:
		packetProof = src.chainProvider.PacketAcknowledgement
		assembleMessage = dst.chainProvider.MsgAcknowledgement
	case chantypes.EventTypeTimeoutPacket:
		if msg.info.ChannelOrder == chantypes.ORDERED.String() {
			packetProof = src.chainProvider.NextSeqRecv
		} else {
			packetProof = src.chainProvider.PacketReceipt
		}

		assembleMessage = dst.chainProvider.MsgTimeout
	case chantypes.EventTypeTimeoutPacketOnClose:
		if msg.info.ChannelOrder == chantypes.ORDERED.String() {
			packetProof = src.chainProvider.NextSeqRecv
		} else {
			packetProof = src.chainProvider.PacketReceipt
		}

		assembleMessage = dst.chainProvider.MsgTimeoutOnClose
	default:
		return nil, fmt.Errorf("unexepected packet message eventType for message assembly: %s", msg.eventType)
	}

	ctx, cancel := context.WithTimeout(ctx, packetProofQueryTimeout)
	defer cancel()

	var proof provider.PacketProof
	var err error
	proof, err = packetProof(ctx, msg.info, src.latestBlock.Height)
	if err != nil {
		return nil, fmt.Errorf("error querying packet proof: %w", err)
	}
	return assembleMessage(msg.info, proof)
}

// assembleConnectionMessage executes the appropriate proof query function,
// then, if successful, assembles the message for the destination.
func (mp *messageProcessor) assembleConnectionMessage(
	ctx context.Context,
	msg connectionIBCMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var connProof func(context.Context, provider.ConnectionInfo, uint64) (provider.ConnectionProof, error)
	var assembleMessage func(provider.ConnectionInfo, provider.ConnectionProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case conntypes.EventTypeConnectionOpenInit:
		// don't need proof for this message
		msg.info.CounterpartyCommitmentPrefix = src.chainProvider.CommitmentPrefix()
		assembleMessage = dst.chainProvider.MsgConnectionOpenInit
	case conntypes.EventTypeConnectionOpenTry:
		msg.info.CounterpartyCommitmentPrefix = src.chainProvider.CommitmentPrefix()
		connProof = src.chainProvider.ConnectionHandshakeProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenTry
	case conntypes.EventTypeConnectionOpenAck:
		connProof = src.chainProvider.ConnectionHandshakeProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenAck
	case conntypes.EventTypeConnectionOpenConfirm:
		connProof = src.chainProvider.ConnectionProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenConfirm
	default:
		return nil, fmt.Errorf("unexepected connection message eventType for message assembly: %s", msg.eventType)
	}
	var proof provider.ConnectionProof
	var err error
	if connProof != nil {
		proof, err = connProof(ctx, msg.info, src.latestBlock.Height)
		if err != nil {
			return nil, fmt.Errorf("error querying connection proof: %w", err)
		}
	}
	return assembleMessage(msg.info, proof)
}

// assembleChannelMessage executes the appropriate proof query function,
// then, if successful, assembles the message for the destination.
func (mp *messageProcessor) assembleChannelMessage(
	ctx context.Context,
	msg channelIBCMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var chanProof func(context.Context, provider.ChannelInfo, uint64) (provider.ChannelProof, error)
	var assembleMessage func(provider.ChannelInfo, provider.ChannelProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case chantypes.EventTypeChannelOpenInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgChannelOpenInit
	case chantypes.EventTypeChannelOpenTry:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenTry
	case chantypes.EventTypeChannelOpenAck:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenAck
	case chantypes.EventTypeChannelOpenConfirm:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenConfirm
	case chantypes.EventTypeChannelCloseInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgChannelCloseInit
	case chantypes.EventTypeChannelCloseConfirm:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelCloseConfirm
	default:
		return nil, fmt.Errorf("unexepected channel message eventType for message assembly: %s", msg.eventType)
	}
	var proof provider.ChannelProof
	var err error
	if chanProof != nil {
		proof, err = chanProof(ctx, msg.info, src.latestBlock.Height)
		if err != nil {
			return nil, fmt.Errorf("error querying channel proof: %w", err)
		}
	}
	return assembleMessage(msg.info, proof)
}

// assembleClientICQMessage executes the query against the source chain,
// then, if successful, assembles the response message for the destination.
func (mp *messageProcessor) assembleClientICQMessage(
	ctx context.Context,
	msg clientICQMessage,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, interchainQueryTimeout)
	defer cancel()

	proof, err := src.chainProvider.QueryICQWithProof(ctx, msg.info.Type, msg.info.Request, src.latestBlock.Height-1)
	if err != nil {
		return nil, fmt.Errorf("error during interchain query: %w", err)
	}

	return dst.chainProvider.MsgSubmitQueryResponse(msg.info.Chain, msg.info.QueryID, proof)
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
	batchMsgs := []provider.RelayerMessage{mp.msgUpdateClient}

	for _, t := range mp.connMsgs {
		retries := dst.trackProcessingConnectionMessage(t)
		if t.m == nil {
			continue
		}
		if broadcastBatch && retries == 0 {
			batchMsgs = append(batchMsgs, t.m)
			continue
		}
		go mp.sendConnectionMessage(ctx, src, dst, t)
	}

	for _, t := range mp.chanMsgs {
		retries := dst.trackProcessingChannelMessage(t)
		if t.m == nil {
			continue
		}
		if broadcastBatch && retries == 0 {
			batchMsgs = append(batchMsgs, t.m)
			continue
		}
		go mp.sendChannelMessage(ctx, src, dst, t)
	}

	for _, t := range mp.clientICQMsgs {
		retries := dst.trackProcessingClientICQMessage(t)
		if t.m == nil {
			continue
		}
		if broadcastBatch && retries == 0 {
			batchMsgs = append(batchMsgs, t.m)
			continue
		}
		go mp.sendClientICQMessage(ctx, src, dst, t)

	}

	for _, t := range mp.pktMsgs {
		retries := dst.trackProcessingPacketMessage(t)
		if t.m == nil {
			continue
		}
		if broadcastBatch && retries == 0 {
			batchMsgs = append(batchMsgs, t.m)
			continue
		}
		go mp.sendPacketMessage(ctx, src, dst, t)
	}

	if len(batchMsgs) > 1 {
		go mp.sendBatchMessages(ctx, src, dst, batchMsgs, mp.pktMsgs)
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
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Error(err),
		)
		return
	}
	dst.log.Debug("Client update broadcast completed")
}

// sendBatchMessages will send a batch of messages,
// then increment metrics counters for successful packet messages.
func (mp *messageProcessor) sendBatchMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	msgs []provider.RelayerMessage,
	pktMsgs []packetMessageToTrack,
) {
	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	dst.log.Debug("Will relay batch of messages", zap.Int("count", len(msgs)))

	callback := func(rtr *provider.RelayerTxResponse, err error) {
		// only increment metrics counts for successful packets
		if err != nil || mp.metrics == nil {
			return
		}
		for _, tracker := range pktMsgs {
			var channel, port string
			if tracker.msg.eventType == chantypes.EventTypeRecvPacket {
				channel = tracker.msg.info.DestChannel
				port = tracker.msg.info.DestPort
			} else {
				channel = tracker.msg.info.SourceChannel
				port = tracker.msg.info.SourcePort
			}
			mp.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, tracker.msg.eventType)
		}
	}

	if err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, callback); err != nil {
		errFields := []zapcore.Field{
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
			zap.Error(err),
		}
		if errors.Is(err, chantypes.ErrRedundantTx) {
			mp.log.Debug("Packet(s) already handled by another relayer", errFields...)
			return
		}
		mp.log.Error("Error sending batch of messages", errFields...)
		return
	}
	dst.log.Debug("Batch messages broadcast completed")
}

// sendPacketMessage will send an isolated packet message
// then increment metrics counters if successful.
func (mp *messageProcessor) sendPacketMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	tracker packetMessageToTrack,
) {
	msgs := []provider.RelayerMessage{mp.msgUpdateClient, tracker.m}

	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	packetFields := []zapcore.Field{
		zap.String("event_type", tracker.msg.eventType),
		zap.String("src_port", tracker.msg.info.SourcePort),
		zap.String("src_channel", tracker.msg.info.SourceChannel),
		zap.String("dst_port", tracker.msg.info.DestPort),
		zap.String("dst_channel", tracker.msg.info.DestChannel),
		zap.Uint64("sequence", tracker.msg.info.Sequence),
		zap.String("timeout_height", fmt.Sprintf(
			"%d-%d",
			tracker.msg.info.TimeoutHeight.RevisionNumber,
			tracker.msg.info.TimeoutHeight.RevisionHeight,
		)),
		zap.Uint64("timeout_timestamp", tracker.msg.info.TimeoutTimestamp),
		zap.String("data", base64.StdEncoding.EncodeToString(tracker.msg.info.Data)),
		zap.String("ack", base64.StdEncoding.EncodeToString(tracker.msg.info.Ack)),
	}

	dst.log.Debug("Will relay packet message", packetFields...)

	callback := func(rtr *provider.RelayerTxResponse, err error) {
		// only increment metrics counts for successful packets
		if err != nil || mp.metrics == nil {
			return
		}
		var channel, port string
		if tracker.msg.eventType == chantypes.EventTypeRecvPacket {
			channel = tracker.msg.info.DestChannel
			port = tracker.msg.info.DestPort
		} else {
			channel = tracker.msg.info.SourceChannel
			port = tracker.msg.info.SourcePort
		}
		mp.metrics.IncPacketsRelayed(dst.info.PathName, dst.info.ChainID, channel, port, tracker.msg.eventType)
	}

	err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, callback)
	if err != nil {
		errFields := append([]zapcore.Field{
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
		}, packetFields...)
		errFields = append(errFields, zap.Error(err))

		if errors.Is(err, chantypes.ErrRedundantTx) {
			mp.log.Debug("Packet already handled by another relayer", errFields...)
			return
		}
		mp.log.Error("Error sending packet message", errFields...)
		return
	}
	dst.log.Debug("Packet message broadcast completed", packetFields...)
}

// sendChannelMessage will send an isolated channel handshake message.
func (mp *messageProcessor) sendChannelMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	tracker channelMessageToTrack,
) {
	msgs := []provider.RelayerMessage{mp.msgUpdateClient, tracker.m}

	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	channelFields := []zapcore.Field{
		zap.String("event_type", tracker.msg.eventType),
		zap.String("port_id", tracker.msg.info.PortID),
		zap.String("channel_id", tracker.msg.info.ChannelID),
		zap.String("counterparty_port_id", tracker.msg.info.CounterpartyPortID),
		zap.String("counterparty_channel_id", tracker.msg.info.CounterpartyChannelID),
		zap.String("connection_id", tracker.msg.info.ConnID),
		zap.String("counterparty_connection_id", tracker.msg.info.CounterpartyConnID),
		zap.String("order", tracker.msg.info.Order.String()),
		zap.String("version", tracker.msg.info.Version),
	}

	dst.log.Debug("Will relay channel message", channelFields...)

	err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, nil)
	if err != nil {
		errFields := []zapcore.Field{
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
		}
		errFields = append(errFields, channelFields...)
		errFields = append(errFields, zap.Error(err))
		mp.log.Error("Error sending channel handshake message", errFields...)
		return
	}
	dst.log.Debug("Channel handshake message broadcast completed", channelFields...)
}

// sendConnectionMessage will send an isolated connection handshake message.
func (mp *messageProcessor) sendConnectionMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	tracker connectionMessageToTrack,
) {
	msgs := []provider.RelayerMessage{mp.msgUpdateClient, tracker.m}

	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	connFields := []zapcore.Field{
		zap.String("event_type", tracker.msg.eventType),
		zap.String("client_id", tracker.msg.info.ClientID),
		zap.String("counterparty_client_id", tracker.msg.info.CounterpartyClientID),
		zap.String("connection_id", tracker.msg.info.ConnID),
		zap.String("counterparty_connection_id", tracker.msg.info.CounterpartyConnID),
		zap.String("counterparty_commitment_prefix", tracker.msg.info.CounterpartyCommitmentPrefix.String()),
	}

	dst.log.Debug("Will relay connection message", connFields...)

	err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, nil)
	if err != nil {
		errFields := []zapcore.Field{
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
		}
		errFields = append(errFields, connFields...)
		errFields = append(errFields, zap.Error(err))
		mp.log.Error("Error sending connection handshake message", errFields...)
		return
	}
	dst.log.Debug("Connection handshake message broadcast completed", connFields...)
}

// sendClientICQMessage will send an isolated ICQ message.
func (mp *messageProcessor) sendClientICQMessage(
	ctx context.Context,
	src, dst *pathEndRuntime,
	tracker clientICQMessageToTrack,
) {
	msgs := []provider.RelayerMessage{mp.msgUpdateClient, tracker.m}

	broadcastCtx, cancel := context.WithTimeout(ctx, messageSendTimeout)
	defer cancel()

	icqFields := []zapcore.Field{
		zap.String("type", tracker.msg.info.Type),
		zap.String("query_id", string(tracker.msg.info.QueryID)),
		zap.String("request", string(tracker.msg.info.Request)),
	}

	dst.log.Debug("Will relay Stride ICQ message", icqFields...)

	err := dst.chainProvider.SendMessagesToMempool(broadcastCtx, msgs, mp.memo, ctx, nil)
	if err != nil {
		errFields := []zapcore.Field{
			zap.String("src_chain_id", src.info.ChainID),
			zap.String("dst_chain_id", dst.info.ChainID),
			zap.String("src_client_id", src.info.ClientID),
			zap.String("dst_client_id", dst.info.ClientID),
		}
		errFields = append(errFields, icqFields...)
		errFields = append(errFields, zap.Error(err))
		mp.log.Error("Error sending client ICQ message", errFields...)
		return
	}
	dst.log.Debug("Stride ICQ message broadcast completed", icqFields...)
}
