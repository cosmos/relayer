package paths

import (
	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/processor"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	DurationErrorRetry = 5 * time.Second
)

type PathProcessor struct {
	log *zap.Logger

	pathEnd1 *PathEndRuntime
	pathEnd2 *PathEndRuntime

	processLock                    sync.RWMutex
	isRunning                      bool
	shouldProcessAgainOnceComplete bool
	retryTimer                     *time.Timer
}

type PathEndRuntime struct {
	info PathEnd

	chainProcessor processor.ChainProcessor

	// ibc messages backlog
	// retains minimal state for a given packet
	// packet message history will be held onto until messages come in that signal the packet flow is complete
	// when packet-flow-complete messages are handled (MsgAcknowledgement, MsgTimeout, or MsgTimeoutOnClose),
	// the entire packet history for that sequence number will be cleared out, including counterparty pathEnd
	messages     map[string]map[uint64]provider.RelayerMessage
	messagesLock sync.RWMutex
}

type IBCMessageWithSequence struct {
	Sequence uint64
	Message  provider.RelayerMessage
}

func NewPathProcessor(log *zap.Logger, pathEnd1 PathEnd, pathEnd2 PathEnd) *PathProcessor {
	return &PathProcessor{
		log: log,
		pathEnd1: &PathEndRuntime{
			info:     pathEnd1,
			messages: make(map[string]map[uint64]provider.RelayerMessage),
		},
		pathEnd2: &PathEndRuntime{
			info:     pathEnd2,
			messages: make(map[string]map[uint64]provider.RelayerMessage),
		},
	}
}

// TEST USE ONLY
func (pp *PathProcessor) GetPathEnd1Messages(message string) map[uint64]provider.RelayerMessage {
	pp.pathEnd1.messagesLock.RLock()
	defer pp.pathEnd1.messagesLock.RUnlock()
	return pp.pathEnd1.messages[message]
}

// TEST USE ONLY
func (pp *PathProcessor) GetPathEnd2Messages(message string) map[uint64]provider.RelayerMessage {
	pp.pathEnd2.messagesLock.RLock()
	defer pp.pathEnd2.messagesLock.RUnlock()
	return pp.pathEnd2.messages[message]
}

// Path Processors are constructed before ChainProcessors, so reference needs to be added afterwards
func (pp *PathProcessor) SetPathEnd1ChainProcessor(chainProcessor processor.ChainProcessor) {
	pp.pathEnd1.chainProcessor = chainProcessor
}

func (pp *PathProcessor) SetPathEnd2ChainProcessor(chainProcessor processor.ChainProcessor) {
	pp.pathEnd2.chainProcessor = chainProcessor
}

// ChainProcessors call this method when they have new IBC messages
func (pp *PathProcessor) HandleNewMessages(chainID string, channelKey ibc.ChannelKey, messages map[string]map[uint64]provider.RelayerMessage) {
	if pp.pathEnd1.info.ChainID == chainID {
		// TODO make sure passes channel filter for pathEnd1 before calling this
		pp.handleNewMessagesForPathEnd(pp.pathEnd1, messages)
	} else if pp.pathEnd2.info.ChainID == chainID {
		// TODO make sure passes channel filter for pathEnd2 before calling this
		pp.handleNewMessagesForPathEnd(pp.pathEnd2, messages)
	}
}

func (pp *PathProcessor) handleNewMessagesForPathEnd(
	p *PathEndRuntime,
	newMessages map[string]map[uint64]provider.RelayerMessage,
) {
	p.messagesLock.Lock()
	for msg, sequencesMap := range newMessages {
		if len(sequencesMap) == 0 {
			continue
		}
		for sequence, ibcMsg := range sequencesMap {
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

// run process if not already running
// if process is already running, schedule it to run again immediately after it finishes if shouldProcessAgainOnceComplete is true
func (pp *PathProcessor) ScheduleNextProcess(shouldProcessAgainOnceComplete bool) {
	if pp.isProcessRunning() {
		pp.getShouldProcessAgainOnceCompleteAndUpdateTo(true)
		return
	}
	go pp.process(false)
}

// wait for retry timer to expire, then run process if not already running
func (pp *PathProcessor) scheduleProcessRetry() {
	<-pp.retryTimer.C
	pp.ScheduleNextProcess(false)
}

func (pp *PathProcessor) isProcessRunning() bool {
	pp.processLock.RLock()
	defer pp.processLock.RUnlock()
	return pp.isRunning
}

// this contains MsgRecvPacket from same chain
// needs to be transformed into PathEndPacketFlowMessages once counterparty info is available to complete packet flow state for pathEnd
type PathEndMessages struct {
	MsgTransfer        map[uint64]provider.RelayerMessage
	MsgRecvPacket      map[uint64]provider.RelayerMessage
	MsgAcknowledgement map[uint64]provider.RelayerMessage
}

// contains MsgRecvPacket from counterparty
// entire packet flow
type PathEndPacketFlowMessages struct {
	SrcMsgTransfer        map[uint64]provider.RelayerMessage
	DstMsgRecvPacket      map[uint64]provider.RelayerMessage
	SrcMsgAcknowledgement map[uint64]provider.RelayerMessage
	// TODO SrcTimeout and SrcTimeoutOnClose
}

type PathEndProcessedResponse struct {
	UnrelayedPackets          []IBCMessageWithSequence
	UnrelayedAcknowledgements []IBCMessageWithSequence

	ToDeleteSrc map[string][]uint64
	ToDeleteDst map[string][]uint64
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(pathEndPacketFlowMessages PathEndPacketFlowMessages, waitGroup *sync.WaitGroup, res *PathEndProcessedResponse) {
	defer waitGroup.Done()
	unrelayedPackets := []IBCMessageWithSequence{}
	unrelayedAcknowledgements := []IBCMessageWithSequence{}

	toDeleteSrc := make(map[string][]uint64)
	toDeleteDst := make(map[string][]uint64)

MsgTransferLoop:
	for transferSeq, msgTransfer := range pathEndPacketFlowMessages.SrcMsgTransfer {
		for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
			if transferSeq == ackSeq {
				// we have an ack for this packet, so packet flow is complete
				// remove all retention of this sequence number
				toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], transferSeq)
				toDeleteDst[ibc.MsgRecvPacket] = append(toDeleteDst[ibc.MsgRecvPacket], transferSeq)
				toDeleteSrc[ibc.MsgAcknowledgement] = append(toDeleteSrc[ibc.MsgAcknowledgement], transferSeq)
				continue MsgTransferLoop
			}
		}

		// would iterate timeout messages here also

		for msgRecvSeq, msgAcknowledgement := range pathEndPacketFlowMessages.DstMsgRecvPacket {
			if transferSeq == msgRecvSeq {
				// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
				unrelayedAcknowledgements = append(unrelayedAcknowledgements, IBCMessageWithSequence{Sequence: msgRecvSeq, Message: msgAcknowledgement})
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay from src to dst
		unrelayedPackets = append(unrelayedPackets, IBCMessageWithSequence{Sequence: transferSeq, Message: msgTransfer})
	}

	// now iterate through packet-flow-complete messages and remove any leftover messages if the MsgTransfer or MsgRecvPacket was in a previous block that we did not query
	for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
		toDeleteSrc[ibc.MsgTransfer] = append(toDeleteSrc[ibc.MsgTransfer], ackSeq)
		toDeleteDst[ibc.MsgRecvPacket] = append(toDeleteDst[ibc.MsgRecvPacket], ackSeq)
		toDeleteSrc[ibc.MsgAcknowledgement] = append(toDeleteSrc[ibc.MsgAcknowledgement], ackSeq)
	}

	// would iterate timeout messages here also

	res.UnrelayedPackets = unrelayedPackets
	res.UnrelayedAcknowledgements = unrelayedAcknowledgements
	res.ToDeleteSrc = toDeleteSrc
	res.ToDeleteDst = toDeleteDst
}

func (pp *PathProcessor) deleteCachedMessages(messages map[string]map[uint64]provider.RelayerMessage, messagesLock *sync.RWMutex, toDelete ...map[string][]uint64) {
	messagesLock.Lock()
	defer messagesLock.Unlock()
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, sequence := range toDeleteMessages {
				delete(messages[message], sequence)
			}
		}
	}
}

func (pp *PathProcessor) sendMessages(pathEnd *PathEndRuntime, messages []IBCMessageWithSequence) error {
	if len(messages) == 0 {
		return nil
	}
	// TODO construct MsgUpdateClient for this pathEnd, using the latest trusted IBC header from other pathEnd, prepend messages with the MsgUpdateClient, then send the messages to this pathEnd

	pp.log.Debug("will send", zap.Any("messages", messages))

	return nil
}

// messages from both pathEnds are needed in order to determine what needs to be relayed for a single pathEnd
func (pp *PathProcessor) processLatestMessages() error {

	// get the messages applicable to the packet flows for both pathends
	pathEnd1PacketFlowMessages, pathEnd2PacketFlowMessages := pp.getPathEndPacketFlowMessages()

	var processPacketFlowWaitGroup sync.WaitGroup
	processPacketFlowWaitGroup.Add(2)

	// process the packet flows for both packends to determine what needs to be relayed
	var pathEnd1ProcessRes, pathEnd2ProcessRes PathEndProcessedResponse
	go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd1PacketFlowMessages, &processPacketFlowWaitGroup, &pathEnd1ProcessRes)
	go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd2PacketFlowMessages, &processPacketFlowWaitGroup, &pathEnd2ProcessRes)

	processPacketFlowWaitGroup.Wait()

	// concatenate applicable messages for pathend
	pathEnd1Messages := append(pathEnd2ProcessRes.UnrelayedPackets, pathEnd1ProcessRes.UnrelayedAcknowledgements...)
	pathEnd2Messages := append(pathEnd1ProcessRes.UnrelayedPackets, pathEnd2ProcessRes.UnrelayedAcknowledgements...)

	// now delete all toDelete and send messages in parallel
	// return errors for send messages

	var eg errgroup.Group
	eg.Go(func() error {
		pp.deleteCachedMessages(pp.pathEnd1.messages, &pp.pathEnd1.messagesLock, pathEnd1ProcessRes.ToDeleteSrc, pathEnd2ProcessRes.ToDeleteDst)
		return nil
	})
	eg.Go(func() error {
		pp.deleteCachedMessages(pp.pathEnd2.messages, &pp.pathEnd2.messagesLock, pathEnd2ProcessRes.ToDeleteSrc, pathEnd1ProcessRes.ToDeleteDst)
		return nil
	})
	eg.Go(func() error { return pp.sendMessages(pp.pathEnd1, pathEnd1Messages) })
	eg.Go(func() error { return pp.sendMessages(pp.pathEnd2, pathEnd2Messages) })

	return eg.Wait()
}

// makes copies of messages for the 2 packet flows
// this is done because the messages map is concurrently accessed, so this locks them and copies them
func (pp *PathProcessor) getPathEndPacketFlowMessages() (PathEndPacketFlowMessages, PathEndPacketFlowMessages) {
	pathEnd1Msgs := PathEndMessages{
		MsgTransfer:        make(map[uint64]provider.RelayerMessage),
		MsgRecvPacket:      make(map[uint64]provider.RelayerMessage),
		MsgAcknowledgement: make(map[uint64]provider.RelayerMessage),
	}
	pathEnd2Msgs := PathEndMessages{
		MsgTransfer:        make(map[uint64]provider.RelayerMessage),
		MsgRecvPacket:      make(map[uint64]provider.RelayerMessage),
		MsgAcknowledgement: make(map[uint64]provider.RelayerMessage),
	}

	var copyMessagesWaitGroup sync.WaitGroup
	copyMessagesWaitGroup.Add(2)
	go pp.copyPacketMessages(pp.pathEnd1.messages, &pathEnd1Msgs, &pp.pathEnd1.messagesLock, &copyMessagesWaitGroup)
	go pp.copyPacketMessages(pp.pathEnd2.messages, &pathEnd2Msgs, &pp.pathEnd2.messagesLock, &copyMessagesWaitGroup)
	copyMessagesWaitGroup.Wait()

	return PathEndPacketFlowMessages{
			SrcMsgTransfer:        pathEnd1Msgs.MsgTransfer,
			DstMsgRecvPacket:      pathEnd2Msgs.MsgRecvPacket,
			SrcMsgAcknowledgement: pathEnd1Msgs.MsgAcknowledgement,
		}, PathEndPacketFlowMessages{
			SrcMsgTransfer:        pathEnd2Msgs.MsgTransfer,
			DstMsgRecvPacket:      pathEnd1Msgs.MsgRecvPacket,
			SrcMsgAcknowledgement: pathEnd2Msgs.MsgAcknowledgement,
		}
}

func (pp *PathProcessor) copyPacketMessages(src map[string]map[uint64]provider.RelayerMessage, dst *PathEndMessages, srcLock *sync.RWMutex, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	srcLock.RLock()
	defer srcLock.RUnlock()
	copyMapIfMessageExists(ibc.MsgTransfer, src, dst.MsgTransfer)
	copyMapIfMessageExists(ibc.MsgRecvPacket, src, dst.MsgRecvPacket)
	copyMapIfMessageExists(ibc.MsgAcknowledgement, src, dst.MsgAcknowledgement)
	// TODO timeout messages
}

func copyMapIfMessageExists(message string, src map[string]map[uint64]provider.RelayerMessage, dst map[uint64]provider.RelayerMessage) {
	if src[message] == nil {
		return
	}
	for seq, msg := range src[message] {
		dst[seq] = msg
	}
}

func (pp *PathProcessor) updateIsRunning(isRunning bool) {
	pp.processLock.Lock()
	defer pp.processLock.Unlock()
	pp.isRunning = isRunning
}

func (pp *PathProcessor) getShouldProcessAgainOnceCompleteAndUpdateTo(shouldProcessAgainOnceComplete bool) bool {
	pp.processLock.Lock()
	defer pp.processLock.Unlock()
	existing := pp.shouldProcessAgainOnceComplete
	pp.shouldProcessAgainOnceComplete = shouldProcessAgainOnceComplete
	return existing
}

// main path process
// get unrelayed packets and acks
// if packets were timed out, construct those timeouts
// attempt to send those messages, prepending an update client
// if IBC messages fail to send, schedule retry after DurationErrorRetry
func (pp *PathProcessor) process(fromSelf bool) {
	if !fromSelf {
		pp.updateIsRunning(true)
	}

	err := pp.processLatestMessages()

	if pp.getShouldProcessAgainOnceCompleteAndUpdateTo(false) {
		go pp.process(true)
	} else {
		// if errors were found with sending IBC messages, and process is not going to run again right away, schedule retry
		// helps in case process does not run automatically from new IBC messages for this path within DurationErrorRetry
		if err != nil {
			if pp.retryTimer != nil {
				pp.retryTimer.Stop()
			}
			pp.retryTimer = time.NewTimer(DurationErrorRetry)
			go pp.scheduleProcessRetry()
		}

		pp.updateIsRunning(false)
	}
}
