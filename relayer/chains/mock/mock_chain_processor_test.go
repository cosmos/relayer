package mock_test

import (
	"context"
	"sync"
	"testing"
	"time"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/mock"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestMockChainAndPathProcessors(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	mockChainID1 := "mock-chain-1"
	mockChainID2 := "mock-chain-2"

	pathEnd1 := processor.PathEnd{ChainID: mockChainID1}
	pathEnd2 := processor.PathEnd{ChainID: mockChainID2}

	mockSequence1 := uint64(0)
	mockSequence2 := uint64(0)
	lastSentMockMsgRecvSequence1 := uint64(0)
	lastSentMockMsgRecvSequence2 := uint64(0)
	var mockLock sync.Mutex

	getMockMessages1 := func() []mock.TransactionMessage {
		return getMockMessages(&mockSequence1, &mockSequence2, &lastSentMockMsgRecvSequence2, &mockLock)
	}
	getMockMessages2 := func() []mock.TransactionMessage {
		return getMockMessages(&mockSequence2, &mockSequence1, &lastSentMockMsgRecvSequence1, &mockLock)
	}

	log := zaptest.NewLogger(t)

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*20)
	defer ctxCancel()

	pathProcessor := processor.NewPathProcessor(log, pathEnd1, pathEnd2)

	eventProcessor := processor.NewEventProcessor().
		WithChainProcessors(
			mock.NewMockChainProcessor(log, mockChainID1, getMockMessages1),
			mock.NewMockChainProcessor(log, mockChainID2, getMockMessages2),
		).
		WithInitialBlockHistory(100).
		WithPathProcessors(pathProcessor).
		Build()

	err := eventProcessor.Run(ctx)
	require.NoError(t, err, "error running event processor")

	pathEnd1LeftoverMsgTransfer := pathProcessor.PathEnd1Messages(processor.MsgTransfer)
	pathEnd1LeftoverMsgRecvPacket := pathProcessor.PathEnd1Messages(processor.MsgRecvPacket)
	pathEnd1LeftoverMsgAcknowledgement := pathProcessor.PathEnd1Messages(processor.MsgAcknowledgement)

	pathEnd2LeftoverMsgTransfer := pathProcessor.PathEnd2Messages(processor.MsgTransfer)
	pathEnd2LeftoverMsgRecvPacket := pathProcessor.PathEnd2Messages(processor.MsgRecvPacket)
	pathEnd2LeftoverMsgAcknowledgement := pathProcessor.PathEnd2Messages(processor.MsgAcknowledgement)

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
}

// will send cycles of:
// MsgTransfer
// MsgRecvPacket for counterparty
// MsgAcknowledgement
func getMockMessages(mockSequence, mockSequenceCounterparty, lastSentMockMsgRecvCounterparty *uint64, lock *sync.Mutex) []mock.TransactionMessage {
	lock.Lock()
	defer lock.Unlock()
	*mockSequence++
	mockMessages := []mock.TransactionMessage{
		{
			Action:     processor.MsgTransfer,
			PacketInfo: &chantypes.Packet{Sequence: *mockSequence},
		},
	}
	if *mockSequenceCounterparty > 1 && *lastSentMockMsgRecvCounterparty != *mockSequenceCounterparty {
		*lastSentMockMsgRecvCounterparty = *mockSequenceCounterparty
		mockMessages = append(mockMessages, mock.TransactionMessage{
			Action:     processor.MsgRecvPacket,
			PacketInfo: &chantypes.Packet{Sequence: *mockSequenceCounterparty - 1},
		})
	}
	if *mockSequence > 2 {
		mockMessages = append(mockMessages, mock.TransactionMessage{
			Action:     processor.MsgAcknowledgement,
			PacketInfo: &chantypes.Packet{Sequence: *mockSequence - 2},
		})
	}
	return mockMessages
}
