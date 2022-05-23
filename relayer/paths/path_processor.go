package paths

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/processor"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type PathProcessor struct {
	ctx context.Context
	log *zap.Logger

	pathEnd1 *PathEndRuntime
	pathEnd2 *PathEndRuntime

	processLock                    sync.Mutex
	shouldProcessAgainOnceComplete bool
	retryTimer                     *time.Timer
}

type PathEndRuntime struct {
	info PathEnd

	chainProcessor processor.ChainProcessor

	// ibc Messages backlog
	// retains minimal state for a given packet
	// packet message history will be held onto until messages come in that signal the packet flow is complete
	// when packet-flow-complete messages are handled (MsgAcknowledgement, MsgTimeout, or MsgTimeoutOnClose),
	// the entire packet history for that sequence number will be cleared out
	messages     map[string]map[uint64]provider.RelayerMessage
	messagesLock sync.Mutex

	// ibc Messages that are in progress sending
	// Used so that cycles in the `process` method do not attempt to send the same messages twice
	// also adds packet retry if a given packet fails to send after the threshold # blocks
	inProgressMessageSends     map[string]map[uint64]ibc.InProgressSend
	inProgressMessageSendsLock sync.Mutex

	maxSequence     uint64
	maxSequenceLock sync.Mutex
}

func (per *PathEndRuntime) updateMaxSequence(sequence uint64) {
	if sequence > per.maxSequence {
		per.maxSequence = sequence
	}
}

func NewPathProcessor(ctx context.Context, log *zap.Logger, pathEnd1 PathEnd, pathEnd2 PathEnd) *PathProcessor {
	return &PathProcessor{
		ctx: ctx,
		log: log,
		pathEnd1: &PathEndRuntime{
			info:                   pathEnd1,
			messages:               make(map[string]map[uint64]provider.RelayerMessage),
			inProgressMessageSends: make(map[string]map[uint64]ibc.InProgressSend),
		},
		pathEnd2: &PathEndRuntime{
			info:                   pathEnd2,
			messages:               make(map[string]map[uint64]provider.RelayerMessage),
			inProgressMessageSends: make(map[string]map[uint64]ibc.InProgressSend),
		},
	}
}

const (
	BlocksToRetryPacketAfter = 5
	PacketMaxRetries         = 5
	DurationErrorRetry       = 5 * time.Second
)

func (pp *PathProcessor) SetPathEnd1ChainProcessor(chainProcessor processor.ChainProcessor) {
	pp.pathEnd1.chainProcessor = chainProcessor
}

func (pp *PathProcessor) SetPathEnd2ChainProcessor(chainProcessor processor.ChainProcessor) {
	pp.pathEnd2.chainProcessor = chainProcessor
}

func (pp *PathProcessor) HandleNewMessages(chainID string, channelKey ibc.ChannelKey, messages map[string]map[uint64]provider.RelayerMessage) {
	if pp.pathEnd1.info.ChainID == chainID {
		if pp.pathEnd1.info.ShouldRelayChannel(channelKey) {
			pp.handleNewMessagesForPathEnd(pp.pathEnd1, messages)
		}
	} else if pp.pathEnd2.info.ChainID == chainID {
		if pp.pathEnd2.info.ShouldRelayChannel(channelKey) {
			pp.handleNewMessagesForPathEnd(pp.pathEnd2, messages)
		}
	}
}

func (pp *PathProcessor) handleNewMessagesForPathEnd(
	p *PathEndRuntime,
	newMessages map[string]map[uint64]provider.RelayerMessage,
) {
	// fmt.Printf("{%s} New messages: +%v\n", p.info.ChainID, newMessages)
	p.messagesLock.Lock()
	for msg, sequencesMap := range newMessages {
		if len(sequencesMap) == 0 {
			continue
		}
		for sequence, ibcMsg := range sequencesMap {
			// update max sequence for MsgTransfer messages
			if msg == ibc.MsgTransfer {
				p.maxSequenceLock.Lock()
				p.updateMaxSequence(sequence)
				p.maxSequenceLock.Unlock()
			}
			if _, ok := p.messages[msg]; !ok {
				p.messages[msg] = make(map[uint64]provider.RelayerMessage)
			}
			p.messages[msg][sequence] = ibcMsg
		}
	}
	p.messagesLock.Unlock()
	// only start processing packets if querying latest heights for both chains
	if pp.pathEnd1.chainProcessor.InSync() && pp.pathEnd2.chainProcessor.InSync() {
		pp.ScheduleNextProcess(true)
	}
}

func (pp *PathProcessor) IsRelevantChannel(chainID string, channelKey ibc.ChannelKey) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ShouldRelayChannel(channelKey)
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ShouldRelayChannel(channelKey)
	}
	return false
}

func (pp *PathProcessor) IsRelevantClient(clientID string) bool {
	return pp.pathEnd1.info.ClientID == clientID || pp.pathEnd2.info.ClientID == clientID
}

func (pp *PathProcessor) GetRelevantConnectionID(chainID string) (string, error) {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ConnectionID, nil
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ConnectionID, nil
	}
	return "", errors.New("irrelevant chain processor provided")
}

func deleteLocked[K comparable, L comparable, T any](messages map[L]map[K]T, message L, sequence K, lock *sync.Mutex) {
	lock.Lock()
	defer lock.Unlock()
	delete(messages[message], sequence)
}

// gets unrelayed packets and acks and cleans up sequence retention for completed packet flows
func (pp *PathProcessor) getUnrelayedPacketsAndAcks(srcPathEnd, dstPathEnd *PathEndRuntime) ([]ibc.IBCMessageWithSequence, []ibc.IBCMessageWithSequence) {
	// need to copy maps for reading
	srcMsgTransferMessages := make(map[uint64]provider.RelayerMessage)
	srcMsgAcknowledgementMessages := make(map[uint64]provider.RelayerMessage)
	srcMsgTimeoutMessages := make(map[uint64]provider.RelayerMessage)
	srcMsgTimeoutOnCloseMessages := make(map[uint64]provider.RelayerMessage)
	dstMsgRecvPacketMessages := make(map[uint64]provider.RelayerMessage)

	srcPathEnd.messagesLock.Lock()
	if srcPathEnd.messages[ibc.MsgTransfer] != nil {
		for transferSeq, msgTransfer := range srcPathEnd.messages[ibc.MsgTransfer] {
			srcMsgTransferMessages[transferSeq] = msgTransfer
		}
	}
	if srcPathEnd.messages[ibc.MsgAcknowledgement] != nil {
		for ackSeq, msgAcknowledgement := range srcPathEnd.messages[ibc.MsgAcknowledgement] {
			srcMsgAcknowledgementMessages[ackSeq] = msgAcknowledgement
		}
	}
	if srcPathEnd.messages[ibc.MsgTimeout] != nil {
		for timeoutSeq, msgTimeout := range srcPathEnd.messages[ibc.MsgTimeout] {
			srcMsgTimeoutMessages[timeoutSeq] = msgTimeout
		}
	}
	if srcPathEnd.messages[ibc.MsgTimeoutOnClose] != nil {
		for timeoutSeq, msgTimeout := range srcPathEnd.messages[ibc.MsgTimeoutOnClose] {
			srcMsgTimeoutOnCloseMessages[timeoutSeq] = msgTimeout
		}
	}
	srcPathEnd.messagesLock.Unlock()

	dstPathEnd.messagesLock.Lock()
	if dstPathEnd.messages[ibc.MsgRecvPacket] != nil {
		for msgRecvSeq, msgAcknowledgement := range dstPathEnd.messages[ibc.MsgRecvPacket] {
			dstMsgRecvPacketMessages[msgRecvSeq] = msgAcknowledgement
		}
	}
	dstPathEnd.messagesLock.Unlock()

	unrelayedPackets := []ibc.IBCMessageWithSequence{}
	unrelayedAcknowledgements := []ibc.IBCMessageWithSequence{}

	toDeleteSrc := make(map[string][]uint64)
	toDeleteSrcInProgress := make(map[string][]uint64)
	toDeleteDst := make(map[string][]uint64)
	toDeleteDstInProgress := make(map[string][]uint64)

MsgTransferLoop:
	for transferSeq, msgTransfer := range srcMsgTransferMessages {
		for ackSeq := range srcMsgAcknowledgementMessages {
			if transferSeq == ackSeq {
				// we have an ack for this packet, so packet flow is complete
				// remove all retention of this sequence number
				toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], transferSeq)
				toDeleteDst[ibc.MsgRecvPacket] = append(toDeleteDst[ibc.MsgRecvPacket], transferSeq)
				toDeleteSrc[ibc.MsgAcknowledgement] = append(toDeleteSrc[ibc.MsgAcknowledgement], transferSeq)
				toDeleteDstInProgress[ibc.MsgRecvPacket] = append(toDeleteDstInProgress[ibc.MsgRecvPacket], transferSeq)
				toDeleteSrcInProgress[ibc.MsgAcknowledgement] = append(toDeleteSrcInProgress[ibc.MsgAcknowledgement], transferSeq)
				continue MsgTransferLoop
			}
		}
		for timeoutSeq := range srcMsgTimeoutMessages {
			if transferSeq == timeoutSeq {
				// we have a timeout for this packet, so packet flow is complete
				// remove all retention of this sequence number
				toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], transferSeq)
				toDeleteSrc[ibc.MsgTimeout] = append(toDeleteSrc[ibc.MsgTimeout], transferSeq)
				toDeleteSrcInProgress[ibc.MsgTimeout] = append(toDeleteSrcInProgress[ibc.MsgTimeout], transferSeq)
				continue MsgTransferLoop
			}
		}
		for timeoutOnCloseSeq := range srcMsgTimeoutOnCloseMessages {
			if transferSeq == timeoutOnCloseSeq {
				// we have a timeout for this packet, so packet flow is complete
				// remove all retention of this sequence number
				toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], transferSeq)
				toDeleteSrc[ibc.MsgTimeoutOnClose] = append(toDeleteSrc[ibc.MsgTimeoutOnClose], transferSeq)
				toDeleteSrcInProgress[ibc.MsgTimeoutOnClose] = append(toDeleteSrcInProgress[ibc.MsgTimeoutOnClose], transferSeq)
				continue MsgTransferLoop
			}
		}
		for msgRecvSeq, msgAcknowledgement := range dstMsgRecvPacketMessages {
			if transferSeq == msgRecvSeq {
				// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
				unrelayedAcknowledgements = append(unrelayedAcknowledgements, ibc.IBCMessageWithSequence{Sequence: msgRecvSeq, Message: msgAcknowledgement})
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay from src to dst
		unrelayedPackets = append(unrelayedPackets, ibc.IBCMessageWithSequence{Sequence: transferSeq, Message: msgTransfer})
	}

	// now iterate through packet-flow-complete messages and remove any leftover messages if the MsgTransfer or MsgRecvPacket was in a previous block that we did not query
	for ackSeq := range srcMsgAcknowledgementMessages {
		toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], ackSeq)
		toDeleteDst[ibc.MsgRecvPacket] = append(toDeleteDst[ibc.MsgRecvPacket], ackSeq)
		toDeleteSrc[ibc.MsgAcknowledgement] = append(toDeleteSrc[ibc.MsgAcknowledgement], ackSeq)
		toDeleteDstInProgress[ibc.MsgRecvPacket] = append(toDeleteDstInProgress[ibc.MsgRecvPacket], ackSeq)
		toDeleteSrcInProgress[ibc.MsgAcknowledgement] = append(toDeleteSrcInProgress[ibc.MsgAcknowledgement], ackSeq)
	}

	for timeoutSeq := range srcMsgTimeoutMessages {
		toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], timeoutSeq)
		toDeleteSrc[ibc.MsgTimeout] = append(toDeleteSrc[ibc.MsgTimeout], timeoutSeq)
		toDeleteSrcInProgress[ibc.MsgTimeout] = append(toDeleteSrcInProgress[ibc.MsgTimeout], timeoutSeq)
	}

	for timeoutOnCloseSeq := range srcMsgTimeoutOnCloseMessages {
		toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], timeoutOnCloseSeq)
		toDeleteSrc[ibc.MsgTimeoutOnClose] = append(toDeleteSrc[ibc.MsgTimeoutOnClose], timeoutOnCloseSeq)
		toDeleteSrcInProgress[ibc.MsgTimeoutOnClose] = append(toDeleteSrcInProgress[ibc.MsgTimeoutOnClose], timeoutOnCloseSeq)
	}

	// now delete all toDelete

	srcPathEnd.messagesLock.Lock()
	for message, messages := range toDeleteSrc {
		for _, sequence := range messages {
			delete(srcPathEnd.messages[message], sequence)
		}
	}
	srcPathEnd.messagesLock.Unlock()

	srcPathEnd.inProgressMessageSendsLock.Lock()
	for message, messages := range toDeleteSrcInProgress {
		for _, sequence := range messages {
			delete(srcPathEnd.inProgressMessageSends[message], sequence)
		}
	}
	srcPathEnd.inProgressMessageSendsLock.Unlock()

	dstPathEnd.messagesLock.Lock()
	for message, messages := range toDeleteDst {
		for _, sequence := range messages {
			delete(dstPathEnd.messages[message], sequence)
		}
	}
	dstPathEnd.messagesLock.Unlock()

	dstPathEnd.inProgressMessageSendsLock.Lock()
	for message, messages := range toDeleteDstInProgress {
		for _, sequence := range messages {
			delete(dstPathEnd.inProgressMessageSends[message], sequence)
		}
	}
	dstPathEnd.inProgressMessageSendsLock.Unlock()

	return unrelayedPackets, unrelayedAcknowledgements
}

// track in progress sends. handle packet retry if necessary. assemble specific messages
// assumes non-concurrent access for messages
// TODO optimize inProgressMessageSends lock
func (pp *PathProcessor) assembleAndTrackMessageState(
	pathEnd *PathEndRuntime,
	message string,
	messageSequence uint64,
	preIBCMessage provider.RelayerMessage,
	getMessageFunc func(string, provider.RelayerMessage) (provider.RelayerMessage, error),
	messages *[]provider.RelayerMessage,
) error {
	pathEnd.inProgressMessageSendsLock.Lock()
	defer pathEnd.inProgressMessageSendsLock.Unlock()
	inProgress := false
	var retryCount uint64
	var ibcMessage *ibc.InProgressSend
	if _, ok := pathEnd.inProgressMessageSends[message]; ok {
		if inProgressSend, ok2 := pathEnd.inProgressMessageSends[message][messageSequence]; ok2 {
			currentHeight := pathEnd.chainProcessor.Latest().Height
			if currentHeight-inProgressSend.SendHeight < BlocksToRetryPacketAfter {
				inProgress = true
			} else {
				existingRetryCount := pathEnd.inProgressMessageSends[message][messageSequence].RetryCount
				if existingRetryCount == PacketMaxRetries {
					pp.log.Error("giving up on packet", zap.String("chainID", pathEnd.info.ChainID), zap.Uint64("sequence", messageSequence))
					delete(pathEnd.inProgressMessageSends[message], messageSequence)
					// set in progress to true so that it is not sent again
					inProgress = true
				} else {
					retryCount = existingRetryCount + 1
					existingMessage := pathEnd.inProgressMessageSends[message][messageSequence]
					ibcMessage = &existingMessage
					pp.log.Warn("retrying packet", zap.String("chainID", pathEnd.info.ChainID), zap.Uint64("sequence", messageSequence), zap.Uint64("retryCount", retryCount))
				}
			}
		}
	}
	if !inProgress {
		if _, ok := pathEnd.inProgressMessageSends[message]; !ok {
			pathEnd.inProgressMessageSends[message] = make(map[uint64]ibc.InProgressSend)
		}
		if ibcMessage == nil {
			signer, err := pathEnd.chainProcessor.Provider().Address()
			if err != nil {
				return fmt.Errorf("error getting signer account address for {%s}: %w\n", pathEnd.info.ChainID, err)
			}
			newIBCMessage, err := getMessageFunc(signer, preIBCMessage)
			if err != nil {
				return fmt.Errorf("error assembling %s for {%s}: %w\n", message, pathEnd.info.ChainID, err)
			}
			ibcMessage = &ibc.InProgressSend{
				SendHeight: pathEnd.chainProcessor.Latest().Height,
				Message:    newIBCMessage,
				RetryCount: retryCount,
			}
		} else {
			ibcMessage.SendHeight = pathEnd.chainProcessor.Latest().Height
			ibcMessage.RetryCount = retryCount
		}
		pp.log.Debug("will send message", zap.String("chainID", pathEnd.info.ChainID), zap.String("message", message), zap.Uint64("sequence", messageSequence))
		pathEnd.inProgressMessageSends[message][messageSequence] = *ibcMessage
		*messages = append(*messages, ibcMessage.Message)
	}
	return nil
}

// assemble IBC messages in preparation for sending
// assumes non-concurrent access to srcMessages and dstMessages
func (pp *PathProcessor) getIBCMessages(
	pathEndSrc *PathEndRuntime,
	pathEndDst *PathEndRuntime,
	pathEndSrcToDstUnrelayedPackets []ibc.IBCMessageWithSequence,
	pathEndSrcToDstUnrelayedAcknowledgements []ibc.IBCMessageWithSequence,
	srcMessages *[]provider.RelayerMessage,
	dstMessages *[]provider.RelayerMessage,
	relayWaitGroup *sync.WaitGroup,
) {
	defer relayWaitGroup.Done()
	for _, msgRecvPacket := range pathEndSrcToDstUnrelayedPackets {
		if err := pathEndDst.chainProcessor.ValidatePacket(msgRecvPacket.Message); err != nil {
			// if timeouts were detected, need to generate msgs for them for src
			switch err.(type) {
			case *ibc.TimeoutError:
				if err := pp.assembleAndTrackMessageState(pathEndSrc, ibc.MsgTimeout, msgRecvPacket.Sequence, msgRecvPacket.Message, pathEndDst.chainProcessor.GetMsgTimeout, srcMessages); err != nil {
					pp.log.Error("error assembling MsgTimeout", zap.Uint64("sequence", msgRecvPacket.Sequence), zap.String("chainID", pathEndSrc.info.ChainID), zap.Error(err))
				}
			case *ibc.TimeoutOnCloseError:
				if err := pp.assembleAndTrackMessageState(pathEndSrc, ibc.MsgTimeoutOnClose, msgRecvPacket.Sequence, msgRecvPacket.Message, pathEndDst.chainProcessor.GetMsgTimeoutOnClose, srcMessages); err != nil {
					pp.log.Error("error assembling MsgTimeoutOnClose", zap.Uint64("sequence", msgRecvPacket.Sequence), zap.String("chainID", pathEndSrc.info.ChainID), zap.Error(err))
				}
			default:
				pp.log.Error("packet is invalid", zap.String("chainID", pathEndSrc.info.ChainID), zap.Error(err))
			}
			continue
		}
		if err := pp.assembleAndTrackMessageState(pathEndDst, ibc.MsgRecvPacket, msgRecvPacket.Sequence, msgRecvPacket.Message, pathEndSrc.chainProcessor.GetMsgRecvPacket, dstMessages); err != nil {
			pp.log.Error("error assembling MsgRecvPacket", zap.Uint64("sequence", msgRecvPacket.Sequence), zap.String("srcChainID", pathEndSrc.info.ChainID), zap.String("dstChainID", pathEndDst.info.ChainID), zap.Error(err))
		}
	}
	for _, msgAcknowledgement := range pathEndSrcToDstUnrelayedAcknowledgements {
		if err := pp.assembleAndTrackMessageState(pathEndDst, ibc.MsgAcknowledgement, msgAcknowledgement.Sequence, msgAcknowledgement.Message, pathEndSrc.chainProcessor.GetMsgAcknowledgement, dstMessages); err != nil {
			pp.log.Error("error assembling MsgAcknowledgement", zap.Uint64("sequence", msgAcknowledgement.Sequence), zap.String("srcChainID", pathEndSrc.info.ChainID), zap.String("dstChainID", pathEndDst.info.ChainID), zap.Error(err))
		}
	}
}

// assumes non-empty messages list. Will create an update client message for dst and send along with messages
func (pp *PathProcessor) sendIBCMessagesWithUpdateClient(messages []provider.RelayerMessage, src, dst *PathEndRuntime) error {
	dstClientLatestHeight, err := dst.chainProcessor.ClientHeight(dst.info.ClientID)
	if err != nil {
		return fmt.Errorf("error getting client height for dst: %w", err)
	}
	srcLatest, err := src.chainProcessor.LatestHeaderWithTrustedVals(dstClientLatestHeight.RevisionHeight)
	if err != nil {
		return fmt.Errorf("error getting header for src: %w", err)
	}
	msgUpdateClient, err := dst.chainProcessor.GetMsgUpdateClient(dst.info.ClientID, srcLatest)
	if err != nil {
		return fmt.Errorf("error getting MsgUpdateClient for dst: %w", err)
	}
	messages = append([]provider.RelayerMessage{msgUpdateClient}, messages...)
	pp.log.Debug("Sending messages", zap.String("chainID", dst.info.ChainID), zap.Int("count", len(messages)))
	if _, _, err := dst.chainProcessor.Provider().SendMessages(pp.ctx, messages); err != nil {
		return fmt.Errorf("error sending IBC messages: %w", err)
	}
	return nil
}

// run process if not already running
// if process is already running, schedule it to run again immediately after it finishes if shouldProcessAgainOnceComplete is true
func (pp *PathProcessor) ScheduleNextProcess(shouldProcessAgainOnceComplete bool) {
	if pp.isProcessLocked() {
		if shouldProcessAgainOnceComplete {
			pp.shouldProcessAgainOnceComplete = true
		}
		return
	}
	go pp.process(false)
}

// wait for retry timer to expire, then run process if not already running
func (pp *PathProcessor) scheduleProcessRetry() {
	<-pp.retryTimer.C
	pp.ScheduleNextProcess(false)
}

func (pp *PathProcessor) isProcessLocked() bool {
	processLockState := reflect.ValueOf(&pp.processLock).Elem().FieldByName("state")
	return processLockState.Int()&1 == 1
}

// main path process
// get unrelayed packets and acks
// attempt to relay those packets and acks
// if packets were failed to relay due to timeouts, send those timeouts
func (pp *PathProcessor) process(fromSelf bool) {
	if !fromSelf {
		pp.processLock.Lock()
	}

	// notice variable names here. packets (first return var) are from src to dst, and acks (second return var) are from dst to src
	pathEnd1To2UnrelayedPackets, pathEnd2To1UnrelayedAcknowledgements := pp.getUnrelayedPacketsAndAcks(pp.pathEnd1, pp.pathEnd2)
	pathEnd2To1UnrelayedPackets, pathEnd1To2UnrelayedAcknowledgements := pp.getUnrelayedPacketsAndAcks(pp.pathEnd2, pp.pathEnd1)

	havePath1PacketsOrAcksToRelay := len(pathEnd1To2UnrelayedPackets) > 0 || len(pathEnd1To2UnrelayedAcknowledgements) > 0
	havePath2PacketsOrAcksToRelay := len(pathEnd2To1UnrelayedPackets) > 0 || len(pathEnd2To1UnrelayedAcknowledgements) > 0

	foundError := false

	// relay packets and/or acks if applicable
	if havePath1PacketsOrAcksToRelay || havePath2PacketsOrAcksToRelay {
		pathEnd1Messages1 := []provider.RelayerMessage{}
		pathEnd1Messages2 := []provider.RelayerMessage{}
		pathEnd2Messages1 := []provider.RelayerMessage{}
		pathEnd2Messages2 := []provider.RelayerMessage{}

		relayWaitGroup := sync.WaitGroup{}
		if havePath1PacketsOrAcksToRelay {
			relayWaitGroup.Add(1)
			go pp.getIBCMessages(
				pp.pathEnd1,
				pp.pathEnd2,
				pathEnd1To2UnrelayedPackets,
				pathEnd1To2UnrelayedAcknowledgements,
				&pathEnd1Messages1,
				&pathEnd2Messages1,
				&relayWaitGroup,
			)
		}
		if havePath2PacketsOrAcksToRelay {
			relayWaitGroup.Add(1)
			go pp.getIBCMessages(
				pp.pathEnd2,
				pp.pathEnd1,
				pathEnd2To1UnrelayedPackets,
				pathEnd2To1UnrelayedAcknowledgements,
				&pathEnd2Messages2,
				&pathEnd1Messages2,
				&relayWaitGroup,
			)
		}
		relayWaitGroup.Wait()

		pathEnd1Messages := append(pathEnd1Messages1, pathEnd1Messages2...)
		pathEnd2Messages := append(pathEnd2Messages1, pathEnd2Messages2...)

		// send messages if applicable
		var eg errgroup.Group

		if len(pathEnd1Messages) > 0 {
			eg.Go(func() error {
				return pp.sendIBCMessagesWithUpdateClient(pathEnd1Messages, pp.pathEnd2, pp.pathEnd1)
			})
		}
		if len(pathEnd2Messages) > 0 {
			eg.Go(func() error {
				return pp.sendIBCMessagesWithUpdateClient(pathEnd2Messages, pp.pathEnd1, pp.pathEnd2)
			})
		}
		if err := eg.Wait(); err != nil {
			foundError = true
		}
	}

	if pp.shouldProcessAgainOnceComplete {
		pp.shouldProcessAgainOnceComplete = false
		go pp.process(true)
	} else {
		// if errors were found with sending IBC messages, and process is not going to run again right away, schedule retry
		// helps in case process does not run automatically from new IBC messages for this path within DurationErrorRetry
		if foundError {
			if pp.retryTimer != nil {
				pp.retryTimer.Stop()
			}
			pp.retryTimer = time.NewTimer(DurationErrorRetry)
			go pp.scheduleProcessRetry()
		}
		pp.processLock.Unlock()
	}
}
