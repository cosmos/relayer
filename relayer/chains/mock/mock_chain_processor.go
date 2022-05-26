package mock

import (
	"context"
	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/processor"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"go.uber.org/zap"
)

const (
	minQueryLoopDuration = 1 * time.Second
)

type MockChainProcessor struct {
	log *zap.Logger

	chainID string

	// subscribers to this chain processor, where relevant IBC messages will be published
	pathProcessors []processor.PathProcessor

	// cached latest height of the chain
	latestHeight uint64

	// is the query loop up to date with the latest blocks of the chain
	inSync     bool
	inSyncLock sync.RWMutex

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

func (mcp *MockChainProcessor) InSync() bool {
	mcp.inSyncLock.RLock()
	defer mcp.inSyncLock.RUnlock()
	return mcp.inSync
}

func (mcp *MockChainProcessor) ChainID() string {
	return mcp.chainID
}

func (mcp *MockChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {

	// would be query of latest height, mocking 20
	mcp.latestHeight = 20

	// this will make initial QueryLoop iteration look back initialBlockHistory blocks in history
	latestQueriedBlockInt64 := int64(mcp.latestHeight) - int64(initialBlockHistory)

	var latestQueriedBlock uint64
	if latestQueriedBlockInt64 < 0 {
		latestQueriedBlock = 0
	} else {
		latestQueriedBlock = uint64(latestQueriedBlockInt64)
	}

	mcp.log.Info("entering main query loop", zap.String("chain_id", mcp.chainID))

	// QueryLoop:
	for {
		if ctx.Err() != nil {
			return nil
		}
		cycleTimeStart := time.Now()
		doneWithThisCycle := func() {
			queryDuration := time.Since(cycleTimeStart)
			if queryDuration < minQueryLoopDuration {
				time.Sleep(minQueryLoopDuration - queryDuration)
			}
		}
		// would be query of latest height
		mcp.latestHeight++
		// until in sync, determine if our latest queries are up to date with the current chain height
		// this will cause the PathProcessors to start processing the backlog of message state (once both chainprocessors are in sync)
		mcp.inSyncLock.RLock()
		inSync := mcp.inSync
		mcp.inSyncLock.RUnlock()

		if !inSync {
			if (mcp.latestHeight - latestQueriedBlock) < 2 {
				mcp.inSyncLock.Lock()
				mcp.inSync = true
				mcp.inSyncLock.Unlock()
				mcp.log.Info("chain is in sync", zap.String("chain_id", mcp.chainID))
			} else {
				mcp.log.Warn("chain is not yet in sync",
					zap.String("chain_id", mcp.chainID),
					zap.Uint64("latest_queried_block", latestQueriedBlock),
					zap.Uint64("latest_height", mcp.latestHeight),
				)
			}
		}

		mcp.log.Debug("queried latest height",
			zap.String("chain_id", mcp.chainID),
			zap.Uint64("latest_height", mcp.latestHeight),
		)

		for i := latestQueriedBlock + 1; i <= mcp.latestHeight; i++ {
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
			latestQueriedBlock = i
		}

		doneWithThisCycle()
	}
}
