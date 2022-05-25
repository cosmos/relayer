package mock_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/mock"
	"github.com/cosmos/relayer/v2/relayer/chains/processor"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/paths"
	"github.com/stretchr/testify/require"
)

func TestMockChainAndPathProcessors(t *testing.T) {
	mockChainID1 := "mock-chain-1"
	mockChainID2 := "mock-chain-2"

	pathEnd1 := paths.PathEnd{ChainID: mockChainID1}
	pathEnd2 := paths.PathEnd{ChainID: mockChainID2}

	config := zap.NewDevelopmentConfig()
	log, err := config.Build()
	require.NoError(t, err, "error building zap logger")

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*20)
	defer ctxCancel()

	pathProcessor := paths.NewPathProcessor(log, pathEnd1, pathEnd2)

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

	chainProcessor1 := mock.NewMockChainProcessor(ctx, log, mockChainID1, getMockMessages1, pathProcessor)
	chainProcessor2 := mock.NewMockChainProcessor(ctx, log, mockChainID2, getMockMessages2, pathProcessor)

	pathProcessor.SetPathEnd1ChainProcessor(chainProcessor1)
	pathProcessor.SetPathEnd2ChainProcessor(chainProcessor2)

	initialBlockHistory := uint64(100)
	errChan := make(chan error, 1)
	processor.Start(ctx, initialBlockHistory, errChan, chainProcessor1, chainProcessor2)

	// wait for context to finish
	<-ctx.Done()

	// give path processor time to flush
	time.Sleep(time.Second * 5)

	pathEnd1LeftoverMsgTransfer := pathProcessor.GetPathEnd1Messages(ibc.MsgTransfer)
	pathEnd1LeftoverMsgRecvPacket := pathProcessor.GetPathEnd1Messages(ibc.MsgRecvPacket)
	pathEnd1LeftoverMsgAcknowledgement := pathProcessor.GetPathEnd1Messages(ibc.MsgAcknowledgement)

	pathEnd2LeftoverMsgTransfer := pathProcessor.GetPathEnd2Messages(ibc.MsgTransfer)
	pathEnd2LeftoverMsgRecvPacket := pathProcessor.GetPathEnd2Messages(ibc.MsgRecvPacket)
	pathEnd2LeftoverMsgAcknowledgement := pathProcessor.GetPathEnd2Messages(ibc.MsgAcknowledgement)

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
			Action:     ibc.MsgTransfer,
			PacketInfo: &chantypes.Packet{Sequence: *mockSequence},
		},
	}
	if *mockSequenceCounterparty > 1 && *lastSentMockMsgRecvCounterparty != *mockSequenceCounterparty {
		*lastSentMockMsgRecvCounterparty = *mockSequenceCounterparty
		mockMessages = append(mockMessages, mock.TransactionMessage{
			Action:     ibc.MsgRecvPacket,
			PacketInfo: &chantypes.Packet{Sequence: *mockSequenceCounterparty - 1},
		})
	}
	if *mockSequence > 2 {
		mockMessages = append(mockMessages, mock.TransactionMessage{
			Action:     ibc.MsgAcknowledgement,
			PacketInfo: &chantypes.Packet{Sequence: *mockSequence - 2},
		})
	}
	return mockMessages
}
