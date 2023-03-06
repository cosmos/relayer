package relayer

import (
	"context"
	"fmt"
	"time"

	"github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"go.uber.org/zap"
)

// ActiveChannel represents an IBC channel and whether there is an active goroutine relaying packets against it.
type ActiveChannel struct {
	channel *types.IdentifiedChannel
	active  bool
}

const (
	ProcessorEvents              string = "events"
	ProcessorLegacy                     = "legacy"
	DefaultClientUpdateThreshold        = 0 * time.Millisecond
	DefaultFlushInterval                = 5 * time.Minute
)

// StartRelayer starts the main relaying loop and returns a channel that will contain any control-flow related errors.
func StartRelayer(
	ctx context.Context,
	log *zap.Logger,
	chains map[string]*Chain,
	paths []NamedPath,
	maxTxSize, maxMsgLength uint64,
	memo string,
	clientUpdateThresholdTime time.Duration,
	flushInterval time.Duration,
	messageLifecycle processor.MessageLifecycle,
	initialBlockHistory uint64,
	metrics *processor.PrometheusMetrics,
) chan error {
	errorChan := make(chan error, 1)

	chainProcessors := make([]processor.ChainProcessor, 0, len(chains))

	for _, chain := range chains {
		chainProcessors = append(chainProcessors, chain.chainProcessor(log, metrics))
	}

	ePaths := make([]path, len(paths))
	for i, np := range paths {
		pathName := np.Name
		p := np.Path

		filter := p.Filter
		var filterSrc, filterDst []processor.ChainChannelKey

		for _, ch := range filter.ChannelList {
			ruleSrc := processor.ChainChannelKey{ChainID: p.Src.ChainID, ChannelKey: processor.ChannelKey{ChannelID: ch}}
			ruleDst := processor.ChainChannelKey{CounterpartyChainID: p.Src.ChainID, ChannelKey: processor.ChannelKey{CounterpartyChannelID: ch}}
			filterSrc = append(filterSrc, ruleSrc)
			filterDst = append(filterDst, ruleDst)
		}
		ePaths[i] = path{
			src: processor.NewPathEnd(pathName, p.Src.ChainID, p.Src.ClientID, filter.Rule, filterSrc),
			dst: processor.NewPathEnd(pathName, p.Dst.ChainID, p.Dst.ClientID, filter.Rule, filterDst),
		}
	}

	go relayerStartEventProcessor(
		ctx,
		log,
		chainProcessors,
		ePaths,
		initialBlockHistory,
		maxTxSize,
		maxMsgLength,
		memo,
		messageLifecycle,
		clientUpdateThresholdTime,
		flushInterval,
		errorChan,
		metrics,
	)
	return errorChan
}

// TODO: intermediate types. Should combine/replace with the relayer.Chain, relayer.Path, and relayer.PathEnd structs
// as the stateless and stateful/event-based relaying mechanisms are consolidated.
type path struct {
	src processor.PathEnd
	dst processor.PathEnd
}

// chainProcessor returns the corresponding ChainProcessor implementation instance for a pathChain.
func (chain *Chain) chainProcessor(log *zap.Logger, metrics *processor.PrometheusMetrics) processor.ChainProcessor {
	// Handle new ChainProcessor implementations as cases here
	switch p := chain.ChainProvider.(type) {
	case *cosmos.CosmosProvider:
		return cosmos.NewCosmosChainProcessor(log, p, metrics)
	default:
		panic(fmt.Errorf("unsupported chain provider type: %T", chain.ChainProvider))
	}
}

// relayerStartEventProcessor is the main relayer process when using the event processor.
func relayerStartEventProcessor(
	ctx context.Context,
	log *zap.Logger,
	chainProcessors []processor.ChainProcessor,
	paths []path,
	initialBlockHistory uint64,
	maxTxSize,
	maxMsgLength uint64,
	memo string,
	messageLifecycle processor.MessageLifecycle,
	clientUpdateThresholdTime time.Duration,
	flushInterval time.Duration,
	errCh chan<- error,
	metrics *processor.PrometheusMetrics,
) {
	defer close(errCh)

	epb := processor.NewEventProcessor().WithChainProcessors(chainProcessors...)

	for _, p := range paths {
		epb = epb.
			WithPathProcessors(processor.NewPathProcessor(
				log,
				p.src,
				p.dst,
				metrics,
				memo,
				clientUpdateThresholdTime,
				flushInterval,
			))
	}

	if messageLifecycle != nil {
		epb = epb.WithMessageLifecycle(messageLifecycle)
	}

	ep := epb.
		WithInitialBlockHistory(initialBlockHistory).
		Build()

	errCh <- ep.Run(ctx)
}
