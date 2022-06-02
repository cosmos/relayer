package mock

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cosmos/relayer/v2/relayer/processor"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"go.uber.org/zap"
)

const (
	minQueryLoopDuration     = 1 * time.Second
	inSyncNumBlocksThreshold = 2
)

type MockChainProcessor struct {
	log *zap.Logger

	chainID string

	// subscribers to this chain processor, where relevant IBC messages will be published
	pathProcessors []*processor.PathProcessor

	// this variable is only written from a single thread (Run function here),
	// but is concurrently read by the PathProcessors
	inSync int32

	getMockMessages func() []TransactionMessage
}

// types used for parsing IBC messages from transactions, then passed to message handlers for mutating the MockChainProcessor state if necessary and retaining applicable messages for sending to the Path Processors
type TransactionMessage struct {
	Action     string
	PacketInfo *chantypes.Packet
}

func NewMockChainProcessor(log *zap.Logger, chainID string, getMockMessages func() []TransactionMessage) *MockChainProcessor {
	return &MockChainProcessor{
		log:             log,
		chainID:         chainID,
		getMockMessages: getMockMessages,
	}
}

func (mcp *MockChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	mcp.pathProcessors = pathProcessors
}

// InSync indicates whether queries are in sync with latest height of the chain.
// The PathProcessors use this as a signal for determining if the backlog of messaged is ready to be processed and relayed.
func (mcp *MockChainProcessor) InSync() bool {
	return atomic.LoadInt32(&mcp.inSync) == 1
}

func (mcp *MockChainProcessor) setInSync() {
	atomic.StoreInt32(&mcp.inSync, 1)
}

func (mcp *MockChainProcessor) ChainID() string {
	return mcp.chainID
}

type queryCyclePersistence struct {
	latestHeight       int64
	latestQueriedBlock int64
}

func (mcp *MockChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	// this will be used for persistence across query cycle loop executions
	persistence := queryCyclePersistence{
		// would be query of latest height, mocking 20
		latestHeight: 20,
	}

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlock := persistence.latestHeight - int64(initialBlockHistory)

	if latestQueriedBlock < 0 {
		persistence.latestQueriedBlock = 0
	} else {
		persistence.latestQueriedBlock = latestQueriedBlock
	}

	mcp.log.Info("entering main query loop", zap.String("chain_id", mcp.chainID))

	ticker := time.NewTicker(minQueryLoopDuration)
	// QueryLoop:
	for {
		mcp.queryCycle(ctx, &persistence)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// minQueryLoopDuration never changes for MockChainProcessor, but it will for CosmosChainProcessor, so mocking that behavior
			ticker.Reset(minQueryLoopDuration)
		}
	}
}

func (mcp *MockChainProcessor) queryCycle(ctx context.Context, persistence *queryCyclePersistence) {
	// would be query of latest height
	persistence.latestHeight++

	if !mcp.InSync() {
		if (persistence.latestHeight - persistence.latestQueriedBlock) < inSyncNumBlocksThreshold {
			mcp.setInSync()
			mcp.log.Info("chain is in sync", zap.String("chain_id", mcp.chainID))
		} else {
			mcp.log.Warn("chain is not yet in sync",
				zap.String("chain_id", mcp.chainID),
				zap.Int64("latest_queried_block", persistence.latestQueriedBlock),
				zap.Int64("latest_height", persistence.latestHeight),
			)
		}
	}

	mcp.log.Debug("queried latest height",
		zap.String("chain_id", mcp.chainID),
		zap.Int64("latest_height", persistence.latestHeight),
	)

	for i := persistence.latestQueriedBlock + 1; i <= persistence.latestHeight; i++ {
		// fetch light block

		// store light block's latest signed header and validatorset on chainProcessor if i == latestHeight, needed for constructing MsgUpdateClient on counterparty chain
		// cache last n validatorsets also since needed for constructing MsgUpdateClient for this chain

		// fetch block

		// used for collecting IBC messages that will be sent to the Path Processors
		foundMessages := make(processor.ChannelMessageCache)

		// iterate through transactions
		// iterate through messages in transactions
		// get slice of all IBC messages in those transactions

		// for _, tx := range blockRes.TxsResults {
		//   if tx.Code != 0 {
		//     // tx was not successful
		//     continue
		//   }
		messages := mcp.getMockMessages()

		// iterate through ibc messages and call specific handler for each
		// will do things like mutate chainprocessor state and add relevant messages to foundMessages
		// this can be parralelized also
		for _, m := range messages {
			if handler, ok := messageHandlers[m.Action]; ok {
				handler(MsgHandlerParams{
					mcp:           mcp,
					PacketInfo:    m.PacketInfo,
					FoundMessages: foundMessages,
				})
			}
		}
		// }

		// now pass foundMessages to the path processors
		for channelKey, messages := range foundMessages {
			// TODO do not relay on closed channels
			for _, pp := range mcp.pathProcessors {
				mcp.log.Info("sending messages to path processor", zap.String("chain_id", mcp.chainID))
				pp.HandleNewMessages(mcp.chainID, channelKey, messages)
			}
		}
		persistence.latestQueriedBlock = i
	}
}
