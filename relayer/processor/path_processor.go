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
	// DurationErrorRetry determines how long to wait before retrying
	// in the case of failure to send transactions with IBC messages.
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

func (p PathProcessors) IsRelayedChannel(k ChannelKey, chainID string) bool {
	for _, pp := range p {
		if pp.IsRelayedChannel(chainID, k) {
			return true
		}
	}
	return false
}

// PathEndRuntime is used at runtime for each chain involved in the path.
// It holds a channel for incoming messages from the ChainProcessors, which will
// be processed during Run(ctx).
type PathEndRuntime struct {
	info PathEnd

	chainProvider provider.ChainProvider

	messageCache ChannelMessageCache

	channelStateCache ChannelStateCache

	// New messages and other data arriving from the handleNewMessagesForPathEnd method.
	incomingCacheData chan ChainProcessorCacheData

	// inSync indicates whether queries are in sync with latest height of the chain.
	inSync bool
}

func (pathEnd *PathEndRuntime) MergeCacheData(d ChainProcessorCacheData) {
	// TODO make sure passes channel filter for pathEnd1 before calling this
	pathEnd.messageCache.Merge(d.ChannelMessageCache)    // Merge incoming packet IBC messages into the backlog
	pathEnd.channelStateCache.Merge(d.ChannelStateCache) // Merge incoming channel state IBC messages into the backlog
	pathEnd.inSync = d.InSync
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
			channelStateCache: make(ChannelStateCache),
			messageCache:      make(ChannelMessageCache),
		},
		pathEnd2: &PathEndRuntime{
			info:              pathEnd2,
			incomingCacheData: make(chan ChainProcessorCacheData, 100),
			channelStateCache: make(ChannelStateCache),
			messageCache:      make(ChannelMessageCache),
		},
		retryProcess: make(chan struct{}, 8),
	}
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd1Messages(channelKey ChannelKey, message string) SequenceCache {
	return pp.pathEnd1.messageCache[channelKey][message]
}

// TEST USE ONLY
func (pp *PathProcessor) PathEnd2Messages(channelKey ChannelKey, message string) SequenceCache {
	return pp.pathEnd2.messageCache[channelKey][message]
}

type channelPair struct {
	pathEnd1ChannelKey ChannelKey
	pathEnd2ChannelKey ChannelKey
}

func (pp *PathProcessor) channelPairs() []channelPair {
	channelsFromPathEnd1Perspective := make(map[ChannelKey]bool)
	for channelKey, channelState := range pp.pathEnd1.channelStateCache {
		if !channelState.Open {
			continue
		}
		channelsFromPathEnd1Perspective[channelKey] = true
	}
	for channelKey, channelState := range pp.pathEnd2.channelStateCache {
		if !channelState.Open {
			continue
		}
		channelsFromPathEnd1Perspective[channelKey.Counterparty()] = true
	}
	channelPairs := make([]channelPair, len(channelsFromPathEnd1Perspective))
	i := 0
	for channelKey := range channelsFromPathEnd1Perspective {
		channelPairs[i] = channelPair{
			pathEnd1ChannelKey: channelKey,
			pathEnd2ChannelKey: channelKey.Counterparty(),
		}
		i++
	}
	return channelPairs
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

func (pp *PathProcessor) IsRelayedChannel(chainID string, channelKey ChannelKey) bool {
	if pp.pathEnd1.info.ChainID == chainID {
		return pp.pathEnd1.info.ShouldRelayChannel(channelKey)
	} else if pp.pathEnd2.info.ChainID == chainID {
		return pp.pathEnd2.info.ShouldRelayChannel(channelKey)
	}
	return false
}

func (pp *PathProcessor) IsRelevantClient(chainID string, clientID string) bool {
	if pp.pathEnd1.info.ChainID == chainID && pp.pathEnd1.info.ClientID == clientID {
		return true
	} else if pp.pathEnd2.info.ChainID == chainID && pp.pathEnd2.info.ClientID == clientID {
		return true
	}
	return false
}

func (pp *PathProcessor) IsRelevantConnection(chainID string, connectionID string) bool {
	if pp.pathEnd1.info.ChainID == chainID && pp.pathEnd1.info.ConnectionID == connectionID {
		return true
	} else if pp.pathEnd2.info.ChainID == chainID && pp.pathEnd2.info.ConnectionID == connectionID {
		return true
	}
	return false
}

// ProcessBacklogIfReady gives ChainProcessors a way to trigger the path processor process
// as soon as they are in sync for the first time, even if they do not have new messages.
func (pp *PathProcessor) ProcessBacklogIfReady() {
	select {
	case pp.retryProcess <- struct{}{}:
		// All good.
	default:
		// Log that the channel is saturated;
		// something is wrong if we are retrying this quickly.
		pp.log.Info("Failed to enqueue path processor retry")
	}
}

// ChainProcessors call this method when they have new IBC messages
func (pp *PathProcessor) HandleNewData(chainID string, cacheData ChainProcessorCacheData) {
	if pp.pathEnd1.info.ChainID == chainID {
		pp.pathEnd1.incomingCacheData <- cacheData
	} else if pp.pathEnd2.info.ChainID == chainID {
		pp.pathEnd2.incomingCacheData <- cacheData
	}
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

func (m *PathEndProcessedResponse) appendPacket(sequence uint64, msgRecvPacket provider.RelayerMessage) {
	m.UnrelayedPackets = append(m.UnrelayedPackets, IBCMessageWithSequence{Sequence: sequence, Message: msgRecvPacket})
}

func (m *PathEndProcessedResponse) appendAcknowledgement(sequence uint64, msgAcknowledgement provider.RelayerMessage) {
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
	channelPairs := pp.channelPairs()

	// process the packet flows for both packends to determine what needs to be relayed
	pathEnd1ProcessRes := make([]*PathEndProcessedResponse, len(channelPairs))
	pathEnd2ProcessRes := make([]*PathEndProcessedResponse, len(channelPairs))

	var wg sync.WaitGroup

	for i, channelPair := range channelPairs {
		pathEnd1PacketFlowMessages := PathEndPacketFlowMessages{
			SrcMsgTransfer:        pp.pathEnd1.messageCache[channelPair.pathEnd1ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd2.messageCache[channelPair.pathEnd2ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd1.messageCache[channelPair.pathEnd1ChannelKey][MsgAcknowledgement],
		}
		pathEnd2PacketFlowMessages := PathEndPacketFlowMessages{
			SrcMsgTransfer:        pp.pathEnd2.messageCache[channelPair.pathEnd2ChannelKey][MsgTransfer],
			DstMsgRecvPacket:      pp.pathEnd1.messageCache[channelPair.pathEnd1ChannelKey][MsgRecvPacket],
			SrcMsgAcknowledgement: pp.pathEnd2.messageCache[channelPair.pathEnd2ChannelKey][MsgAcknowledgement],
		}

		pathEnd1ProcessRes[i] = new(PathEndProcessedResponse)
		pathEnd2ProcessRes[i] = new(PathEndProcessedResponse)

		wg.Add(2)
		go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd1PacketFlowMessages, &wg, pathEnd1ProcessRes[i])
		go pp.getUnrelayedPacketsAndAcksAndToDelete(pathEnd2PacketFlowMessages, &wg, pathEnd2ProcessRes[i])
	}
	wg.Wait()

	// concatenate applicable messages for pathend
	var pathEnd1Messages, pathEnd2Messages []IBCMessageWithSequence
	for i := 0; i < len(channelPairs); i++ {
		pathEnd1Messages = append(pathEnd1Messages, pathEnd2ProcessRes[i].UnrelayedPackets...)
		pathEnd1Messages = append(pathEnd1Messages, pathEnd1ProcessRes[i].UnrelayedAcknowledgements...)

		pathEnd2Messages = append(pathEnd2Messages, pathEnd1ProcessRes[i].UnrelayedPackets...)
		pathEnd2Messages = append(pathEnd2Messages, pathEnd2ProcessRes[i].UnrelayedAcknowledgements...)

		pp.pathEnd1.messageCache[channelPairs[i].pathEnd1ChannelKey].DeleteCachedMessages(pathEnd1ProcessRes[i].ToDeleteSrc, pathEnd2ProcessRes[i].ToDeleteDst)
		pp.pathEnd2.messageCache[channelPairs[i].pathEnd2ChannelKey].DeleteCachedMessages(pathEnd2ProcessRes[i].ToDeleteSrc, pathEnd1ProcessRes[i].ToDeleteDst)
	}

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
			// we have new data from ChainProcessor for pathEnd1
			pp.pathEnd1.MergeCacheData(d)

		case d := <-pp.pathEnd2.incomingCacheData:
			// we have new data from ChainProcessor for pathEnd2
			pp.pathEnd2.MergeCacheData(d)

		case <-pp.retryProcess:
			// No new data to merge in, just retry handling.
		}

		if !pp.pathEnd1.inSync || !pp.pathEnd2.inSync {
			continue
		}

		// process latest message cache state from both pathEnds
		if err := pp.processLatestMessages(); err != nil {
			// in case of IBC message send errors, schedule retry after DurationErrorRetry
			time.AfterFunc(DurationErrorRetry, pp.ProcessBacklogIfReady)
		}
	}
}
