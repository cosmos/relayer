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
			info:                       pathEnd1,
			messages:                   make(map[string]map[uint64]provider.RelayerMessage),
			messagesLock:               sync.Mutex{},
			inProgressMessageSends:     make(map[string]map[uint64]ibc.InProgressSend),
			inProgressMessageSendsLock: sync.Mutex{},
			maxSequence:                0,
			maxSequenceLock:            sync.Mutex{},
		},
		pathEnd2: &PathEndRuntime{
			info:                       pathEnd2,
			messages:                   make(map[string]map[uint64]provider.RelayerMessage),
			messagesLock:               sync.Mutex{},
			inProgressMessageSends:     make(map[string]map[uint64]ibc.InProgressSend),
			inProgressMessageSendsLock: sync.Mutex{},
			maxSequence:                0,
			maxSequenceLock:            sync.Mutex{},
		},
		processLock: sync.Mutex{},
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

// gets unrelayed packets and acks and cleans up sequence retention for completed packet flows
func (pp *PathProcessor) getUnrelayedPacketsAndAcks(srcPathEnd, dstPathEnd *PathEndRuntime) ([]ibc.IBCMessageWithSequence, []ibc.IBCMessageWithSequence) {
	srcPathEnd.messagesLock.Lock()
	dstPathEnd.messagesLock.Lock()
	srcPathEnd.inProgressMessageSendsLock.Lock()
	dstPathEnd.inProgressMessageSendsLock.Lock()
	defer srcPathEnd.messagesLock.Unlock()
	defer dstPathEnd.messagesLock.Unlock()
	defer srcPathEnd.inProgressMessageSendsLock.Unlock()
	defer dstPathEnd.inProgressMessageSendsLock.Unlock()
	unrelayedPackets := []ibc.IBCMessageWithSequence{}
	unrelayedAcknowledgements := []ibc.IBCMessageWithSequence{}

	if srcPathEnd.messages[ibc.MsgTransfer] != nil {
	MsgTransferLoop:
		for transferSeq, msgTransfer := range srcPathEnd.messages[ibc.MsgTransfer] {
			if srcPathEnd.messages[ibc.MsgAcknowledgement] != nil {
				for ackSeq := range srcPathEnd.messages[ibc.MsgAcknowledgement] {
					if transferSeq == ackSeq {
						// we have an ack for this packet, so packet flow is complete
						// remove all retention of this sequence number
						delete(srcPathEnd.messages[ibc.MsgTransfer], transferSeq)
						delete(dstPathEnd.messages[ibc.MsgRecvPacket], transferSeq)
						delete(srcPathEnd.messages[ibc.MsgAcknowledgement], transferSeq)
						delete(dstPathEnd.inProgressMessageSends[ibc.MsgRecvPacket], ackSeq)
						delete(srcPathEnd.inProgressMessageSends[ibc.MsgAcknowledgement], ackSeq)
						continue MsgTransferLoop
					}
				}
			}
			if srcPathEnd.messages[ibc.MsgTimeout] != nil {
				for timeoutSeq := range srcPathEnd.messages[ibc.MsgTimeout] {
					if transferSeq == timeoutSeq {
						// we have a timeout for this packet, so packet flow is complete
						// remove all retention of this sequence number
						delete(srcPathEnd.messages[ibc.MsgTransfer], transferSeq)
						delete(srcPathEnd.messages[ibc.MsgTimeout], transferSeq)
						delete(srcPathEnd.inProgressMessageSends[ibc.MsgTimeout], timeoutSeq)
						continue MsgTransferLoop
					}
				}
			}
			if srcPathEnd.messages[ibc.MsgTimeoutOnClose] != nil {
				for timeoutOnCloseSeq := range srcPathEnd.messages[ibc.MsgTimeoutOnClose] {
					if transferSeq == timeoutOnCloseSeq {
						// we have a timeout for this packet, so packet flow is complete
						// remove all retention of this sequence number
						delete(srcPathEnd.messages[ibc.MsgTransfer], transferSeq)
						delete(srcPathEnd.messages[ibc.MsgTimeoutOnClose], transferSeq)
						delete(srcPathEnd.inProgressMessageSends[ibc.MsgTimeoutOnClose], timeoutOnCloseSeq)
						continue MsgTransferLoop
					}
				}
			}
			if dstPathEnd.messages[ibc.MsgRecvPacket] != nil {
				for msgRecvSeq, msgAcknowledgement := range dstPathEnd.messages[ibc.MsgRecvPacket] {
					if transferSeq == msgRecvSeq {
						// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
						unrelayedAcknowledgements = append(unrelayedAcknowledgements, ibc.IBCMessageWithSequence{Sequence: msgRecvSeq, Message: msgAcknowledgement})
						continue MsgTransferLoop
					}
				}
			}
			// Packet is not yet relayed! need to relay from src to dst
			unrelayedPackets = append(unrelayedPackets, ibc.IBCMessageWithSequence{Sequence: transferSeq, Message: msgTransfer})
		}
	}
	// now iterate through packet-flow-complete messages and remove any leftover messages if the MsgTransfer or MsgRecvPacket was in a previous block that we did not query
	if srcPathEnd.messages[ibc.MsgAcknowledgement] != nil {
		for ackSeq := range srcPathEnd.messages[ibc.MsgAcknowledgement] {
			delete(srcPathEnd.messages[ibc.MsgTransfer], ackSeq)
			delete(dstPathEnd.messages[ibc.MsgRecvPacket], ackSeq)
			delete(srcPathEnd.messages[ibc.MsgAcknowledgement], ackSeq)
			delete(dstPathEnd.inProgressMessageSends[ibc.MsgRecvPacket], ackSeq)
			delete(srcPathEnd.inProgressMessageSends[ibc.MsgAcknowledgement], ackSeq)
		}
	}
	if srcPathEnd.messages[ibc.MsgTimeout] != nil {
		for timeoutSeq := range srcPathEnd.messages[ibc.MsgTimeout] {
			delete(srcPathEnd.messages[ibc.MsgTransfer], timeoutSeq)
			delete(srcPathEnd.messages[ibc.MsgTimeout], timeoutSeq)
			delete(srcPathEnd.inProgressMessageSends[ibc.MsgTimeout], timeoutSeq)
		}
	}
	if srcPathEnd.messages[ibc.MsgTimeoutOnClose] != nil {
		for timeoutOnCloseSeq := range srcPathEnd.messages[ibc.MsgTimeoutOnClose] {
			delete(srcPathEnd.messages[ibc.MsgTransfer], timeoutOnCloseSeq)
			delete(srcPathEnd.messages[ibc.MsgTimeoutOnClose], timeoutOnCloseSeq)
			delete(srcPathEnd.inProgressMessageSends[ibc.MsgTimeoutOnClose], timeoutOnCloseSeq)
		}
	}
	return unrelayedPackets, unrelayedAcknowledgements
}

// track in progress sends. handle packet retry if necessary. assemble specific messages
func (pp *PathProcessor) assembleAndTrackMessageState(
	pathEnd *PathEndRuntime,
	message string,
	messageSequence uint64,
	preIBCMessage provider.RelayerMessage,
	getMessageFunc func(string, provider.RelayerMessage) (provider.RelayerMessage, error),
	messages *[]provider.RelayerMessage,
	messagesLock *sync.Mutex,
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
		messagesLock.Lock()
		*messages = append(*messages, ibcMessage.Message)
		messagesLock.Unlock()
	}
	return nil
}

// assemble IBC messages in preparation for sending
func (pp *PathProcessor) getIBCMessages(
	pathEndSrc *PathEndRuntime,
	pathEndDst *PathEndRuntime,
	pathEndSrcToDstUnrelayedPackets []ibc.IBCMessageWithSequence,
	pathEndSrcToDstUnrelayedAcknowledgements []ibc.IBCMessageWithSequence,
	srcMessages *[]provider.RelayerMessage,
	dstMessages *[]provider.RelayerMessage,
	srcMessagesLock *sync.Mutex,
	dstMessagesLock *sync.Mutex,
	relayWaitGroup *sync.WaitGroup,
) {
	defer relayWaitGroup.Done()
	for _, msgRecvPacket := range pathEndSrcToDstUnrelayedPackets {
		if err := pathEndDst.chainProcessor.ValidatePacket(msgRecvPacket.Message); err != nil {
			// if timeouts were detected, need to generate msgs for them for src
			switch err.(type) {
			case *ibc.TimeoutError:
				if err := pp.assembleAndTrackMessageState(pathEndSrc, ibc.MsgTimeout, msgRecvPacket.Sequence, msgRecvPacket.Message, pathEndDst.chainProcessor.GetMsgTimeout, srcMessages, srcMessagesLock); err != nil {
					pp.log.Error("error assembling MsgTimeout", zap.Uint64("sequence", msgRecvPacket.Sequence), zap.String("chainID", pathEndSrc.info.ChainID), zap.Error(err))
				}
			case *ibc.TimeoutOnCloseError:
				if err := pp.assembleAndTrackMessageState(pathEndSrc, ibc.MsgTimeoutOnClose, msgRecvPacket.Sequence, msgRecvPacket.Message, pathEndDst.chainProcessor.GetMsgTimeoutOnClose, srcMessages, srcMessagesLock); err != nil {
					pp.log.Error("error assembling MsgTimeoutOnClose", zap.Uint64("sequence", msgRecvPacket.Sequence), zap.String("chainID", pathEndSrc.info.ChainID), zap.Error(err))
				}
			default:
				pp.log.Error("packet is invalid", zap.String("chainID", pathEndSrc.info.ChainID), zap.Error(err))
			}
			continue
		}
		if err := pp.assembleAndTrackMessageState(pathEndDst, ibc.MsgRecvPacket, msgRecvPacket.Sequence, msgRecvPacket.Message, pathEndSrc.chainProcessor.GetMsgRecvPacket, dstMessages, dstMessagesLock); err != nil {
			pp.log.Error("error assembling MsgRecvPacket", zap.Uint64("sequence", msgRecvPacket.Sequence), zap.String("srcChainID", pathEndSrc.info.ChainID), zap.String("dstChainID", pathEndDst.info.ChainID), zap.Error(err))
		}
	}
	for _, msgAcknowledgement := range pathEndSrcToDstUnrelayedAcknowledgements {
		if err := pp.assembleAndTrackMessageState(pathEndDst, ibc.MsgAcknowledgement, msgAcknowledgement.Sequence, msgAcknowledgement.Message, pathEndSrc.chainProcessor.GetMsgAcknowledgement, dstMessages, dstMessagesLock); err != nil {
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
		pathEnd1Messages := []provider.RelayerMessage{}
		pathEnd2Messages := []provider.RelayerMessage{}
		pathEnd1MessagesLock := sync.Mutex{}
		pathEnd2MessagesLock := sync.Mutex{}

		relayWaitGroup := sync.WaitGroup{}
		if havePath1PacketsOrAcksToRelay {
			relayWaitGroup.Add(1)
			go pp.getIBCMessages(
				pp.pathEnd1,
				pp.pathEnd2,
				pathEnd1To2UnrelayedPackets,
				pathEnd1To2UnrelayedAcknowledgements,
				&pathEnd1Messages,
				&pathEnd2Messages,
				&pathEnd1MessagesLock,
				&pathEnd2MessagesLock,
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
				&pathEnd2Messages,
				&pathEnd1Messages,
				&pathEnd2MessagesLock,
				&pathEnd1MessagesLock,
				&relayWaitGroup,
			)
		}
		relayWaitGroup.Wait()

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
