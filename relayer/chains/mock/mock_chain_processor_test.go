package mock_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/mock"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestMockChainAndPathProcessors(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	mockPathName := "mockpath"
	mockChainID1 := "mock-chain-1"
	mockChainID2 := "mock-chain-2"

	pathEnd1 := processor.PathEnd{PathName: mockPathName, ChainID: mockChainID1, ClientID: "mock-client-1"}
	pathEnd2 := processor.PathEnd{PathName: mockPathName, ChainID: mockChainID2, ClientID: "mock-client-2"}

	mockSequence1 := uint64(0)
	mockSequence2 := uint64(0)
	lastSentMockMsgRecvSequence1 := uint64(0)
	lastSentMockMsgRecvSequence2 := uint64(0)
	var mockLock sync.Mutex

	mockChannelKey1 := processor.ChannelKey{
		ChannelID:             "channel-0",
		PortID:                "port-0",
		CounterpartyChannelID: "channel-1",
		CounterpartyPortID:    "port-1",
	}

	mockChannelKey2 := mockChannelKey1.Counterparty()

	getMockMessages1 := func() []mock.TransactionMessage {
		return getMockMessages(mockChannelKey1, &mockSequence1, &mockSequence2, &lastSentMockMsgRecvSequence2, &mockLock)
	}
	getMockMessages2 := func() []mock.TransactionMessage {
		return getMockMessages(mockChannelKey2, &mockSequence2, &mockSequence1, &lastSentMockMsgRecvSequence1, &mockLock)
	}

	log := zaptest.NewLogger(t)

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*20)
	defer ctxCancel()

	metrics := processor.NewPrometheusMetrics()

	clientUpdateThresholdTime := 6 * time.Hour
	flushInterval := 6 * time.Hour

	pathProcessor := processor.NewPathProcessor(log, pathEnd1, pathEnd2, metrics, "",
		clientUpdateThresholdTime, flushInterval, relayer.DefaultMaxMsgLength)

	eventProcessor := processor.NewEventProcessor().
		WithChainProcessors(
			mock.NewMockChainProcessor(ctx, log, mockChainID1, getMockMessages1),
			mock.NewMockChainProcessor(ctx, log, mockChainID2, getMockMessages2),
		).
		WithInitialBlockHistory(100).
		WithPathProcessors(pathProcessor).
		Build()

	err := eventProcessor.Run(ctx)
	require.NoError(t, err, "error running event processor")

	pathEnd1LeftoverMsgTransfer := pathProcessor.PathEnd1Messages(mockChannelKey1, chantypes.EventTypeSendPacket)
	pathEnd1LeftoverMsgRecvPacket := pathProcessor.PathEnd1Messages(mockChannelKey1, chantypes.EventTypeRecvPacket)
	pathEnd1LeftoverMsgAcknowledgement := pathProcessor.PathEnd1Messages(mockChannelKey1, chantypes.EventTypeAcknowledgePacket)

	pathEnd2LeftoverMsgTransfer := pathProcessor.PathEnd2Messages(mockChannelKey2, chantypes.EventTypeSendPacket)
	pathEnd2LeftoverMsgRecvPacket := pathProcessor.PathEnd2Messages(mockChannelKey2, chantypes.EventTypeRecvPacket)
	pathEnd2LeftoverMsgAcknowledgement := pathProcessor.PathEnd2Messages(mockChannelKey2, chantypes.EventTypeAcknowledgePacket)

	log.Debug("leftover",
		zap.Int("pathEnd1MsgTransfer", len(pathEnd1LeftoverMsgTransfer)),
		zap.Int("pathEnd1MsgRecvPacket", len(pathEnd1LeftoverMsgRecvPacket)),
		zap.Int("pathEnd1MsgAcknowledgement", len(pathEnd1LeftoverMsgAcknowledgement)),
		zap.Int("pathEnd2MsgTransfer", len(pathEnd2LeftoverMsgTransfer)),
		zap.Int("pathEnd2MsgRecvPacket", len(pathEnd2LeftoverMsgRecvPacket)),
		zap.Int("pathEnd2MsgAcknowledgement", len(pathEnd2LeftoverMsgAcknowledgement)),
	)

	// at most 3 msg transfer could still be stuck in queue since chain processor was shut down, so msgrecvpacket would never be "received" by counterparty
	require.LessOrEqual(t, len(pathEnd1LeftoverMsgTransfer), 3)
	// at most 2 msgrecvpacket could still be stuck in the queue
	require.LessOrEqual(t, len(pathEnd1LeftoverMsgRecvPacket), 2)
	// at most 1 msgAcknowledgement could still be stuck in the queue
	require.LessOrEqual(t, len(pathEnd1LeftoverMsgAcknowledgement), 1)

	require.LessOrEqual(t, len(pathEnd2LeftoverMsgTransfer), 3)
	require.LessOrEqual(t, len(pathEnd2LeftoverMsgRecvPacket), 2)
	require.LessOrEqual(t, len(pathEnd2LeftoverMsgAcknowledgement), 1)

	require.Equal(t, 6, testutil.CollectAndCount(metrics.PacketObservedCounter))

	countChain1EventSend := testutil.ToFloat64(metrics.PacketObservedCounter.WithLabelValues(mockPathName, mockChainID1, mockChannelKey1.ChannelID, mockChannelKey1.PortID, chantypes.EventTypeSendPacket))
	countChain1EventRecv := testutil.ToFloat64(metrics.PacketObservedCounter.WithLabelValues(mockPathName, mockChainID1, mockChannelKey1.ChannelID, mockChannelKey1.PortID, chantypes.EventTypeRecvPacket))
	countChain1EventAck := testutil.ToFloat64(metrics.PacketObservedCounter.WithLabelValues(mockPathName, mockChainID1, mockChannelKey1.ChannelID, mockChannelKey1.PortID, chantypes.EventTypeAcknowledgePacket))

	require.GreaterOrEqual(t, float64(mockSequence1), countChain1EventSend)
	require.GreaterOrEqual(t, float64(mockSequence2-1), countChain1EventRecv)
	require.GreaterOrEqual(t, float64(mockSequence1-2), countChain1EventAck)

	require.Greater(t, countChain1EventSend, float64(0))
	require.Greater(t, countChain1EventRecv, float64(0))
	require.Greater(t, countChain1EventAck, float64(0))

	countChain2EventSend := testutil.ToFloat64(metrics.PacketObservedCounter.WithLabelValues(mockPathName, mockChainID2, mockChannelKey2.ChannelID, mockChannelKey2.PortID, chantypes.EventTypeSendPacket))
	countChain2EventRecv := testutil.ToFloat64(metrics.PacketObservedCounter.WithLabelValues(mockPathName, mockChainID2, mockChannelKey2.ChannelID, mockChannelKey2.PortID, chantypes.EventTypeRecvPacket))
	countChain2EventAck := testutil.ToFloat64(metrics.PacketObservedCounter.WithLabelValues(mockPathName, mockChainID2, mockChannelKey2.ChannelID, mockChannelKey2.PortID, chantypes.EventTypeAcknowledgePacket))

	require.GreaterOrEqual(t, float64(mockSequence2), countChain2EventSend)
	require.GreaterOrEqual(t, float64(mockSequence1-1), countChain2EventRecv)
	require.GreaterOrEqual(t, float64(mockSequence2-2), countChain2EventAck)

	require.Greater(t, countChain2EventSend, float64(0))
	require.Greater(t, countChain2EventRecv, float64(0))
	require.Greater(t, countChain2EventAck, float64(0))
}

// will send cycles of:
// MsgTransfer
// MsgRecvPacket for counterparty
// MsgAcknowledgement
func getMockMessages(channelKey processor.ChannelKey, mockSequence, mockSequenceCounterparty, lastSentMockMsgRecvCounterparty *uint64, lock *sync.Mutex) []mock.TransactionMessage {
	lock.Lock()
	defer lock.Unlock()
	if int64(*mockSequence)-int64(*mockSequenceCounterparty) > 0 {
		return []mock.TransactionMessage{}
	}
	*mockSequence++
	mockMessages := []mock.TransactionMessage{
		{
			EventType: chantypes.EventTypeSendPacket,
			PacketInfo: &chantypes.Packet{
				Sequence:           *mockSequence,
				SourceChannel:      channelKey.ChannelID,
				SourcePort:         channelKey.PortID,
				DestinationChannel: channelKey.CounterpartyChannelID,
				DestinationPort:    channelKey.CounterpartyPortID,
				Data:               []byte(strconv.FormatUint(*mockSequence, 10)),
				TimeoutHeight: clienttypes.Height{
					RevisionHeight: 1000,
				},
			},
		},
	}
	if *mockSequenceCounterparty > 1 && *lastSentMockMsgRecvCounterparty != *mockSequenceCounterparty {
		*lastSentMockMsgRecvCounterparty = *mockSequenceCounterparty
		mockMessages = append(mockMessages, mock.TransactionMessage{
			EventType: chantypes.EventTypeRecvPacket,
			PacketInfo: &chantypes.Packet{
				Sequence:           *mockSequenceCounterparty - 1,
				SourceChannel:      channelKey.CounterpartyChannelID,
				SourcePort:         channelKey.CounterpartyPortID,
				DestinationChannel: channelKey.ChannelID,
				DestinationPort:    channelKey.PortID,
				Data:               []byte(strconv.FormatUint(*mockSequenceCounterparty, 10)),
				TimeoutHeight: clienttypes.Height{
					RevisionHeight: 1000,
				},
			},
		})
	}
	if *mockSequence > 2 {
		mockMessages = append(mockMessages, mock.TransactionMessage{
			EventType: chantypes.EventTypeAcknowledgePacket,
			PacketInfo: &chantypes.Packet{
				Sequence:           *mockSequence - 2,
				SourceChannel:      channelKey.ChannelID,
				SourcePort:         channelKey.PortID,
				DestinationChannel: channelKey.CounterpartyChannelID,
				DestinationPort:    channelKey.CounterpartyPortID,
				Data:               []byte(strconv.FormatUint(*mockSequence, 10)),
			},
		})
	}
	return mockMessages
}
