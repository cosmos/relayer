package icon

import (
	"context"

	"go.uber.org/zap"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type IconChainProcessor struct {
	log           *zap.Logger
	chainProvider *IconProvider

	pathProcessors processor.PathProcessors

	inSync bool

	// map of connection ID to client ID
	connectionClients map[string]string

	// map of channel ID to connection ID
	channelConnections map[string]string

	// metrics to monitor lifetime of processor
	metrics *processor.PrometheusMetrics
}

func NewIconChainProcessor(log *zap.Logger, provider *IconProvider, metrics *processor.PrometheusMetrics) *IconChainProcessor {
	return &IconChainProcessor{
		log:           log.With(zap.String("chain_name", "Icon")),
		chainProvider: provider,
		metrics:       metrics,
	}
}

func (icp *IconChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	return nil
}

func (icp *IconChainProcessor) Provider() provider.ChainProvider {
	return icp.chainProvider
}

func (icp *IconChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {}

func (icp *IconChainProcessor) queryCycle(ctx context.Context) {}
