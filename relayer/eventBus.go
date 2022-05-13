package relayer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ChainEventBus struct {
	chains map[string]*ChainEventBusChain
}

type ChainEventBusChain struct {
	chain             *Chain
	subscribers       []*ChainEventBusSubscriber
	latestHeight      int64
	lastHeightQueried int64
	log               *zap.Logger
}

type ChainEventBusSubscriber struct {
	chainID                 string
	onNewUnrelayedSequences func([]uint64)
}

const (
	minQueryLoopDuration = 3 * time.Second
	heightQueryTimeout   = 5 * time.Second
)

func NewChainEventBus(chains []*Chain, log *zap.Logger) ChainEventBus {
	chainEventBusChains := make(map[string]*ChainEventBusChain)
	for _, chain := range chains {
		chainEventBusChains[chain.ChainID()] = &ChainEventBusChain{
			chain:       chain,
			subscribers: []*ChainEventBusSubscriber{},
			log:         log,
		}
	}
	return ChainEventBus{
		chains: chainEventBusChains,
	}
}

func (ceb *ChainEventBus) Subscribe(srcChainID string, dstChainID string, onNewUnrelayedSequences func([]uint64)) error {
	if _, ok := ceb.chains[srcChainID]; !ok {
		return fmt.Errorf("unable to subscribe, chain id does not exist: %s", srcChainID)
	}
	ceb.chains[srcChainID].subscribers = append(ceb.chains[srcChainID].subscribers, &ChainEventBusSubscriber{
		chainID:                 dstChainID,
		onNewUnrelayedSequences: onNewUnrelayedSequences,
	})
	return nil
}

func (ceb *ChainEventBus) Start(ctx context.Context) {
	wg := sync.WaitGroup{}
	for _, chain := range ceb.chains {
		wg.Add(1)
		go chain.QueryLoop(ctx, &wg)
	}
	// wait for initial height from all chains
	wg.Wait()
}

func (c *ChainEventBusChain) QueryLoop(ctx context.Context, wg *sync.WaitGroup) {
	haveInitialHeight := false
	for {
		cycleTimeStart := time.Now()
		doneWithThisCycle := func() {
			queryDuration := time.Since(cycleTimeStart)
			if queryDuration < minQueryLoopDuration {
				time.Sleep(minQueryLoopDuration - queryDuration)
			}
		}
		latestHeightQueryCtx, latestHeightQueryCtxCancel := context.WithTimeout(ctx, heightQueryTimeout)
		latestHeight, err := c.chain.ChainProvider.QueryLatestHeight(latestHeightQueryCtx)
		latestHeightQueryCtxCancel()
		if err != nil {
			c.log.Warn("failed to query latest height", zap.String("chainID", c.chain.ChainID()), zap.Error(err))
			doneWithThisCycle()
			continue
		}

		c.latestHeight = latestHeight
		// have non-zero height now for this chain ID, so caller can unblock
		if !haveInitialHeight {
			haveInitialHeight = true
			wg.Done()
		}

		// TODO add packet commitment and ack queries
		// limit to blocks since lastHeightQueried, except initial query should be full query
		// call onNewUnrelayedSequences in goroutine for each c.Subscriber

		c.lastHeightQueried = latestHeight

		doneWithThisCycle()
	}
}

func (ceb *ChainEventBus) GetLatestHeight(chainID string) (int64, error) {
	chain, ok := ceb.chains[chainID]
	if !ok {
		return -1, fmt.Errorf("unable to get latest height, chain id does not exist in event bus: %s", chainID)
	}
	return chain.latestHeight, nil
}
