package processor

import (
	"context"

	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// DurationErrorRetry determines how long
	DurationErrorRetry = 5 * time.Second
)

// PathProcessor is a process that handles incoming IBC messages from a pair of chains.
// It determines what messages need to be relayed, and sends them.
type PathProcessor struct {
	log *zap.Logger

	pathEnd1 *PathEndRuntime
	pathEnd2 *PathEndRuntime

	// Signals to retry.
	retryProcess chan struct{}

	// TEST USE ONLY
	finalPathEnd1Cache MessageCache
	finalPathEnd2Cache MessageCache
}

// PathProcessors is a slice of PathProcessor instances
type PathProcessors []*PathProcessor

// PathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which will
// be processed during Run(ctx).
type PathEndRuntime struct {
	info PathEnd

	chainProcessor ChainProcessor

	// New messages arriving from the handleNewMessagesForPathEnd method.
	incomingMessages chan MessageCache
}

// IBCMessageWithSequence holds a packet's sequence along with it,
// useful for sending packets around internal to the PathProcessor.
type IBCMessageWithSequence struct {
	Sequence uint64
	Message  provider.RelayerMessage
}

func NewPathProcessor(log *zap.Logger, pathEnd1 PathEnd, pathEnd2 PathEnd) *PathProcessor {
	return &PathProcessor{
		log: log,
		pathEnd1: &PathEndRuntime{
			info:             pathEnd1,
			incomingMessages: make(chan MessageCache, 100),
		},
		pathEnd2: &PathEndRuntime{
			info:             pathEnd2,
			incomingMessages: make(chan MessageCache, 100),
		},
		retryProcess: make(chan struct{}, 8),
	}
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd1Messages(message string) SequenceCache {
	return pp.finalPathEnd1Cache[message]
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd2Messages(message string) SequenceCache {
	return pp.finalPathEnd2Cache[message]
}

// Path Processors are constructed before ChainProcessors, so reference needs to be added afterwards
// This can be done inside the ChainProcessor constructor for simplification
func (pp *PathProcessor) SetChainProcessorIfApplicable(chainID string, chainProcessor ChainProcessor) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		pp.pathEnd1.chainProcessor = chainProcessor
		return true
	} else if pp.pathEnd2.info.ChainID == chainID {
		pp.pathEnd2.chainProcessor = chainProcessor
		return true
	}
	return false
}

// ChainProcessors call this method when they have new IBC messages
func (pp *PathProcessor) HandleNewMessages(chainID string, channelKey ChannelKey, messages MessageCache) {
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
	newMessages MessageCache,
) {
	p.incomingMessages <- newMessages
}

// this contains MsgRecvPacket from same chain
// needs to be transformed into PathEndPacketFlowMessages once counterparty info is available to complete packet flow state for pathEnd
type PathEndMessages struct {
	MsgTransfer        SequenceCache
	MsgRecvPacket      SequenceCache
	MsgAcknowledgement SequenceCache
}

// contains MsgRecvPacket from counterparty
// entire packet flow
type PathEndPacketFlowMessages struct {
	SrcMsgTransfer        SequenceCache
	DstMsgRecvPacket      SequenceCache
	SrcMsgAcknowledgement SequenceCache
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
				toDeleteSrc[MsgTransfer] = append(toDeleteSrc[MsgTransfer], transferSeq)
				toDeleteDst[MsgRecvPacket] = append(toDeleteDst[MsgRecvPacket], transferSeq)
				toDeleteSrc[MsgAcknowledgement] = append(toDeleteSrc[MsgAcknowledgement], transferSeq)
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
		toDeleteSrc[MsgTransfer] = append(toDeleteSrc[MsgTransfer], ackSeq)
		toDeleteDst[MsgRecvPacket] = append(toDeleteDst[MsgRecvPacket], ackSeq)
		toDeleteSrc[MsgAcknowledgement] = append(toDeleteSrc[MsgAcknowledgement], ackSeq)
	}

	// would iterate timeout messages here also

	res.UnrelayedPackets = unrelayedPackets
	res.UnrelayedAcknowledgements = unrelayedAcknowledgements
	res.ToDeleteSrc = toDeleteSrc
	res.ToDeleteDst = toDeleteDst
}

func (pp *PathProcessor) deleteCachedMessages(messages MessageCache, toDelete ...map[string][]uint64) {
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
func (pp *PathProcessor) processLatestMessages(pathEnd1MessageCache MessageCache, pathEnd2MessageCache MessageCache) error {
	var processPacketFlowWaitGroup sync.WaitGroup
	processPacketFlowWaitGroup.Add(2)

	pathEnd1PacketFlowMessages := PathEndPacketFlowMessages{
		SrcMsgTransfer:        pathEnd1MessageCache[MsgTransfer],
		DstMsgRecvPacket:      pathEnd2MessageCache[MsgRecvPacket],
		SrcMsgAcknowledgement: pathEnd1MessageCache[MsgAcknowledgement],
	}
	pathEnd2PacketFlowMessages := PathEndPacketFlowMessages{
		SrcMsgTransfer:        pathEnd2MessageCache[MsgTransfer],
		DstMsgRecvPacket:      pathEnd1MessageCache[MsgRecvPacket],
		SrcMsgAcknowledgement: pathEnd2MessageCache[MsgAcknowledgement],
	}

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
		pp.deleteCachedMessages(pathEnd1MessageCache, pathEnd1ProcessRes.ToDeleteSrc, pathEnd2ProcessRes.ToDeleteDst)
		return nil
	})
	eg.Go(func() error {
		pp.deleteCachedMessages(pathEnd2MessageCache, pathEnd2ProcessRes.ToDeleteSrc, pathEnd1ProcessRes.ToDeleteDst)
		return nil
	})
	eg.Go(func() error { return pp.sendMessages(pp.pathEnd1, pathEnd1Messages) })
	eg.Go(func() error { return pp.sendMessages(pp.pathEnd2, pathEnd2Messages) })

	return eg.Wait()
}

// Run executes the main path process.
func (pp *PathProcessor) Run(ctx context.Context) {

	// keeping backlog of cached IBC messages local to this function
	pathEnd1MessageCache := make(MessageCache)
	pathEnd2MessageCache := make(MessageCache)

	// process latest message cache state from both pathEnds
	processLatestMessages := func() {
		if err := pp.processLatestMessages(pathEnd1MessageCache, pathEnd2MessageCache); err != nil {
			// in case of IBC message send errors, schedule retry after DurationErrorRetry
			time.AfterFunc(DurationErrorRetry, func() {
				pp.retryProcess <- struct{}{}
			})
		}
	}

	for {
		// this is needed if the ctx is cancelled before the chains are in sync.
		if ctx.Err() != nil {
			break
		}
		// don't relay anything until both chains are in sync.
		if !pp.pathEnd1.chainProcessor.InSync() || !pp.pathEnd2.chainProcessor.InSync() {
			continue
		}
		select {
		case <-ctx.Done():
			// this is needed in addition to ctx.Err check above so that shutdown does not hang here if context is cancelled.
			break

		case m := <-pp.pathEnd1.incomingMessages:
			// if new messages are available from pathEnd1, run processLatestMessages
			pathEnd1MessageCache.Merge(m) // Merge incoming messages into the backlog of IBC messages for pathEnd1
			processLatestMessages()

		case m := <-pp.pathEnd2.incomingMessages:
			// if new messages are available from pathEnd2, run processLatestMessages
			pathEnd2MessageCache.Merge(m) // Merge incoming messages into the backlog of IBC messages for pathEnd2
			processLatestMessages()

		case <-pp.retryProcess:
			// No new messages to merge in, just retry handling.
			processLatestMessages()
		}
	}

	// TEST USE ONLY
	// tests can check the state of the message caches after stopping the event processor
	// using PathEnd1Messages() and PathEnd2Messages().
	pp.finalPathEnd1Cache = pathEnd1MessageCache
	pp.finalPathEnd2Cache = pathEnd2MessageCache
}
