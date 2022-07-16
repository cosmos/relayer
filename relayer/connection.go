package relayer

import (
	"context"
	"time"

	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// CreateOpenConnections runs the connection creation messages on timeout until they pass.
// The returned boolean indicates that the path end has been modified.
func (c *Chain) CreateOpenConnections(
	ctx context.Context,
	dst *Chain,
	maxRetries uint64,
	timeout time.Duration,
	memo string,
) (modified bool, err error) {
	// client identifiers must be filled in
	if err = ValidateClientPaths(c, dst); err != nil {
		return modified, err
	}

	srcpathChain := pathChain{
		provider: c.ChainProvider,
		pathEnd:  processor.NewPathEnd(c.PathEnd.ChainID, c.PathEnd.ClientID, "", []processor.ChannelKey{}),
	}
	dstpathChain := pathChain{
		provider: dst.ChainProvider,
		pathEnd:  processor.NewPathEnd(dst.PathEnd.ChainID, dst.PathEnd.ClientID, "", []processor.ChannelKey{}),
	}

	// Timeout is per message. Four connection handshake messages, allowing maxRetries for each.
	processorTimeout := timeout * 4 * time.Duration(maxRetries)

	ctx, cancel := context.WithTimeout(ctx, processorTimeout)
	defer cancel()

	pp := processor.NewPathProcessor(
		c.log,
		srcpathChain.pathEnd,
		dstpathChain.pathEnd,
		memo,
	)

	pp.OnConnectionMessage(dst.PathEnd.ChainID, conntypes.EventTypeConnectionOpenConfirm, func(ci provider.ConnectionInfo) {
		dst.PathEnd.ConnectionID = ci.ConnID
		c.PathEnd.ConnectionID = ci.CounterpartyConnID
		modified = true
	})

	c.log.Info("Starting event processor for connection handshake",
		zap.String("src_chain_id", c.PathEnd.ChainID),
		zap.String("src_client_id", c.PathEnd.ClientID),
		zap.String("dst_chain_id", dst.PathEnd.ChainID),
		zap.String("dst_client_id", dst.PathEnd.ClientID),
	)

	return modified, processor.NewEventProcessor().
		WithChainProcessors(
			srcpathChain.chainProcessor(c.log),
			dstpathChain.chainProcessor(c.log),
		).
		WithPathProcessors(pp).
		WithInitialBlockHistory(0).
		WithMessageLifecycle(&processor.ConnectionMessageLifecycle{
			Initial: &processor.ConnectionMessage{
				ChainID:   c.PathEnd.ChainID,
				EventType: conntypes.EventTypeConnectionOpenInit,
				Info: provider.ConnectionInfo{
					ClientID:             c.PathEnd.ClientID,
					CounterpartyClientID: dst.PathEnd.ClientID,
				},
			},
			Termination: &processor.ConnectionMessage{
				ChainID:   dst.PathEnd.ChainID,
				EventType: conntypes.EventTypeConnectionOpenConfirm,
				Info: provider.ConnectionInfo{
					ClientID:             dst.PathEnd.ClientID,
					CounterpartyClientID: c.PathEnd.ClientID,
				},
			},
		}).
		Build().
		Run(ctx)
}
