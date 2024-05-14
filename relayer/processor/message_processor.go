package processor

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	legacyerrors "github.com/cosmos/cosmos-sdk/types/errors"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// messageProcessor is used for concurrent IBC message assembly and sending
type messageProcessor struct {
	log     *zap.Logger
	metrics *PrometheusMetrics

	messageSender MessageSender

	memo string

	clientUpdateThresholdTime time.Duration

	pktMsgs       []packetMessageToTrack
	connMsgs      []connectionMessageToTrack
	chanMsgs      []channelMessageToTrack
	clientICQMsgs []clientICQMessageToTrack

	isLocalhost bool
}

// categories of tx errors for a Prometheus counter. If the error doesn't fall into one of the below categories, it is labeled as "Tx Failure"
var promErrorCatagories = []error{
	chantypes.ErrRedundantTx,
	legacyerrors.ErrInsufficientFunds,
	legacyerrors.ErrInvalidCoins,
	legacyerrors.ErrOutOfGas,
	legacyerrors.ErrWrongSequence,
}

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
	messageSender MessageSender,
	clientUpdateThresholdTime time.Duration,
	isLocalhost bool,
) *messageProcessor {
	return &messageProcessor{
		log:                       log,
		metrics:                   metrics,
		messageSender:             messageSender,
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

	var msgUpdateClient provider.RelayerMessage

	// Localhost IBC does not permit client updates
	if !isLocalhostClient(src.clientState.ClientID, dst.clientState.ClientID) {
		var err error
		needsClientUpdate, err = mp.shouldUpdateClientNow(ctx, src, dst)
		if err != nil {
			return err
		}

		msgUpdateClient, err = mp.assembleMsgUpdateClient(ctx, src, dst)
		if err != nil {
			return err
		}
	}

	mp.assembleMessages(ctx, messages, src, dst)

	return mp.messageSender.trackAndSendMessages(ctx, src, dst, msgUpdateClient, mp.trackers(), needsClientUpdate)
}

func isLocalhostClient(srcClientID, dstClientID string) bool {
	if srcClientID == ibcexported.LocalhostClientID && dstClientID == ibcexported.LocalhostConnectionID {
		return true
	}

	return false
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
func (mp *messageProcessor) assembleMsgUpdateClient(ctx context.Context, src, dst *pathEndRuntime) (provider.RelayerMessage, error) {
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
			return nil, fmt.Errorf("observed client trusted height: %d does not equal latest client state height: %d",
				trustedConsensusHeight.RevisionHeight, clientConsensusHeight.RevisionHeight)
		}

		header, err := src.chainProvider.QueryIBCHeader(ctx, int64(clientConsensusHeight.RevisionHeight+1))
		if err != nil {
			return nil, fmt.Errorf("error getting IBC header at height: %d for chain_id: %s, %w",
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
		return nil, fmt.Errorf("latest header height is equal to the client trusted height: %d, "+
			"need to wait for next block's header before we can assemble and send a new MsgUpdateClient",
			trustedConsensusHeight.RevisionHeight)
	}

	msgUpdateClientHeader, err := src.chainProvider.MsgUpdateClientHeader(
		src.latestHeader,
		trustedConsensusHeight,
		dst.clientTrustedState.IBCHeader,
	)
	if err != nil {
		return nil, fmt.Errorf("error assembling new client header: %w", err)
	}

	msgUpdateClient, err := dst.chainProvider.MsgUpdateClient(clientID, msgUpdateClientHeader)
	if err != nil {
		return nil, fmt.Errorf("error assembling MsgUpdateClient: %w", err)
	}

	return msgUpdateClient, nil
}

type PathProcessorMessageResp struct {
	Response         *provider.RelayerTxResponse
	DestinationChain provider.ChainProvider
	SuccessfulTx     bool
	Error            error
}

var PathProcMessageCollector chan *PathProcessorMessageResp
