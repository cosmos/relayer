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
}

// PathProcessors is a slice of PathProcessor instances
type PathProcessors []*PathProcessor

// PathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which will
// be processed during Run(ctx).
type PathEndRuntime struct {
	info PathEnd

	chainProvider provider.ChainProvider

	messageCache MessageCache

	// New messages and other data arriving from the handleNewMessagesForPathEnd method.
	incomingCacheData chan ChainProcessorCacheData

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
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
			info:              pathEnd1,
			incomingCacheData: make(chan ChainProcessorCacheData, 100),
			messageCache:      make(MessageCache),
		},
		pathEnd2: &PathEndRuntime{
			info:              pathEnd2,
			incomingCacheData: make(chan ChainProcessorCacheData, 100),
			messageCache:      make(MessageCache),
		},
		retryProcess: make(chan struct{}, 8),
	}
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd1Messages(message string) SequenceCache {
	return pp.pathEnd1.messageCache[message]
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd2Messages(message string) SequenceCache {
	return pp.pathEnd2.messageCache[message]
}

// Path Processors are constructed before ChainProcessors, so reference needs to be added afterwards
// This can be done inside the ChainProcessor constructor for simplification
func (pp *PathProcessor) SetChainProviderIfApplicable(chainProvider provider.ChainProvider) bool {
	if chainProvider == nil {
		return false
	}
	if pp.pathEnd1.info.ChainID == chainProvider.ChainId() {
		pp.pathEnd1.chainProvider = chainProvider
		return true
	} else if pp.pathEnd2.info.ChainID == chainProvider.ChainId() {
		pp.pathEnd2.chainProvider = chainProvider
		return true
	}
	return false
}

// ProcessBacklogIfReady gives ChainProcessors a way to trigger the path processor process
// as soon as they are in sync for the first time, even if they do not have new messages.
func (pp *PathProcessor) ProcessBacklogIfReady() {
	pp.retryProcess <- struct{}{}
}

// ChainProcessors call this method when they have new IBC messages
func (pp *PathProcessor) HandleNewMessages(chainID string, channelKey ChannelKey, cacheData ChainProcessorCacheData) {
	if pp.pathEnd1.info.ChainID == chainID {
		// TODO make sure passes channel filter for pathEnd1 before calling this
		pp.handleNewCacheDataForPathEnd(pp.pathEnd1, cacheData)
	} else if pp.pathEnd2.info.ChainID == chainID {
		// TODO make sure passes channel filter for pathEnd2 before calling this
		pp.handleNewCacheDataForPathEnd(pp.pathEnd2, cacheData)
	}
}

func (pp *PathProcessor) handleNewCacheDataForPathEnd(
	p *PathEndRuntime,
	cacheData ChainProcessorCacheData,
) {
	p.incomingCacheData <- cacheData
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

func (m PathEndProcessedResponse) appendPacket(sequence uint64, msgRecvPacket provider.RelayerMessage) {
	m.UnrelayedPackets = append(m.UnrelayedPackets, IBCMessageWithSequence{Sequence: sequence, Message: msgRecvPacket})
}

func (m PathEndProcessedResponse) appendAcknowledgement(sequence uint64, msgAcknowledgement provider.RelayerMessage) {
	m.UnrelayedAcknowledgements = append(m.UnrelayedAcknowledgements, IBCMessageWithSequence{Sequence: sequence, Message: msgAcknowledgement})
}

func (pp *PathProcessor) getUnrelayedPacketsAndAcksAndToDelete(pathEndPacketFlowMessages PathEndPacketFlowMessages, wg *sync.WaitGroup, res *PathEndProcessedResponse) {
	defer wg.Done()
	res.UnrelayedPackets = nil
	res.UnrelayedAcknowledgements = nil
	res.ToDeleteSrc = make(map[string][]uint64)
	res.ToDeleteDst = make(map[string][]uint64)

MsgTransferLoop:
	for transferSeq, msgTransfer := range pathEndPacketFlowMessages.SrcMsgTransfer {
		for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
			if transferSeq == ackSeq {
				// we have an ack for this packet, so packet flow is complete
				// remove all retention of this sequence number
				res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], transferSeq)
				res.ToDeleteDst[MsgRecvPacket] = append(res.ToDeleteDst[MsgRecvPacket], transferSeq)
				res.ToDeleteSrc[MsgAcknowledgement] = append(res.ToDeleteSrc[MsgAcknowledgement], transferSeq)
				continue MsgTransferLoop
			}
		}

		// would iterate timeout messages here also

		for msgRecvSeq, msgAcknowledgement := range pathEndPacketFlowMessages.DstMsgRecvPacket {
			if transferSeq == msgRecvSeq {
				// msg is received by dst chain, but no ack yet. Need to relay ack from dst to src!
				res.appendAcknowledgement(msgRecvSeq, msgAcknowledgement)
				continue MsgTransferLoop
			}
		}
		// Packet is not yet relayed! need to relay from src to dst
		res.appendPacket(transferSeq, msgTransfer)
	}

	// now iterate through packet-flow-complete messages and remove any leftover messages if the MsgTransfer or MsgRecvPacket was in a previous block that we did not query
	for ackSeq := range pathEndPacketFlowMessages.SrcMsgAcknowledgement {
		res.ToDeleteSrc[MsgTransfer] = append(res.ToDeleteSrc[MsgTransfer], ackSeq)
		res.ToDeleteDst[MsgRecvPacket] = append(res.ToDeleteDst[MsgRecvPacket], ackSeq)
		res.ToDeleteSrc[MsgAcknowledgement] = append(res.ToDeleteSrc[MsgAcknowledgement], ackSeq)
	}

	// would iterate timeout messages here also
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
	pathEnd1PacketFlowMessages := PathEndPacketFlowMessages{
		SrcMsgTransfer:        pp.pathEnd1.messageCache[MsgTransfer],
		DstMsgRecvPacket:      pp.pathEnd2.messageCache[MsgRecvPacket],
		SrcMsgAcknowledgement: pp.pathEnd1.messageCache[MsgAcknowledgement],
	}
	pathEnd2PacketFlowMessages := PathEndPacketFlowMessages{
		SrcMsgTransfer:        pp.pathEnd2.messageCache[MsgTransfer],
		DstMsgRecvPacket:      pp.pathEnd1.messageCache[MsgRecvPacket],
		SrcMsgAcknowledgement: pp.pathEnd2.messageCache[MsgAcknowledgement],
	}

	// process the packet flows for both packends to determine what needs to be relayed
	var pathEnd1ProcessRes, pathEnd2ProcessRes PathEndProcessedResponse
	var wg sync.WaitGroup
	wg.Add(2)
	go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd1PacketFlowMessages, &wg, &pathEnd1ProcessRes)
	go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd2PacketFlowMessages, &wg, &pathEnd2ProcessRes)
	wg.Wait()

	// concatenate applicable messages for pathend
	pathEnd1Messages := append(pathEnd2ProcessRes.UnrelayedPackets, pathEnd1ProcessRes.UnrelayedAcknowledgements...)
	pathEnd2Messages := append(pathEnd1ProcessRes.UnrelayedPackets, pathEnd2ProcessRes.UnrelayedAcknowledgements...)

	pp.pathEnd1.messageCache.DeleteCachedMessages(pathEnd1ProcessRes.ToDeleteSrc, pathEnd2ProcessRes.ToDeleteDst)
	pp.pathEnd2.messageCache.DeleteCachedMessages(pathEnd2ProcessRes.ToDeleteSrc, pathEnd1ProcessRes.ToDeleteDst)

	// now send messages in parallel
	var eg errgroup.Group
	eg.Go(func() error { return pp.sendMessages(pp.pathEnd1, pathEnd1Messages) })
	eg.Go(func() error { return pp.sendMessages(pp.pathEnd2, pathEnd2Messages) })
	return eg.Wait()
}

// Run executes the main path process.
func (pp *PathProcessor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case d := <-pp.pathEnd1.incomingCacheData:
			// if new messages are available from pathEnd1, run processLatestMessages
			pp.pathEnd1.messageCache.Merge(d.NewMessages) // Merge incoming messages into the backlog of IBC messages for pathEnd1
			pp.pathEnd1.inSync = d.InSync

		case d := <-pp.pathEnd2.incomingCacheData:
			// if new messages are available from pathEnd2, run processLatestMessages
			pp.pathEnd2.messageCache.Merge(d.NewMessages) // Merge incoming messages into the backlog of IBC messages for pathEnd2
			pp.pathEnd2.inSync = d.InSync

		case <-pp.retryProcess:
			// No new messages to merge in, just retry handling.
		}

		if !pp.pathEnd1.inSync || !pp.pathEnd2.inSync {
			continue
		}
		// process latest message cache state from both pathEnds
		if err := pp.processLatestMessages(); err != nil {
			// in case of IBC message send errors, schedule retry after DurationErrorRetry
			time.AfterFunc(DurationErrorRetry, func() {
				pp.retryProcess <- struct{}{}
			})
		}
	}
}
