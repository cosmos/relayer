package relayer

import (
	"context"
	"time"

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
) (modified bool, err error) {
	// client identifiers must be filled in
	if err = ValidateClientPaths(c, dst); err != nil {
		return modified, err
	}

	srcPathChain := PathChain{
		Provider: c.ChainProvider,
		PathEnd:  processor.NewPathEnd(c.PathEnd.ChainID, c.PathEnd.ClientID, "", []processor.ChannelKey{}),
	}
	dstPathChain := PathChain{
		Provider: dst.ChainProvider,
		PathEnd:  processor.NewPathEnd(dst.PathEnd.ChainID, dst.PathEnd.ClientID, "", []processor.ChannelKey{}),
	}

	processorCtx, processorCtxCancel := context.WithTimeout(ctx, timeout)
	defer processorCtxCancel()

	pp := processor.NewPathProcessor(
		c.log,
		srcPathChain.PathEnd,
		dstPathChain.PathEnd,
	)

	pp.OnConnectionMessage(dst.PathEnd.ChainID, processor.MsgConnectionOpenConfirm, func(ci provider.ConnectionInfo) {
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
			srcPathChain.ChainProcessor(c.log),
			dstPathChain.ChainProcessor(c.log)).
		WithPathProcessors(pp).
		WithInitialBlockHistory(0).
		WithMessageLifecycle(&processor.ConnectionMessageLifecycle{
			Initial: &processor.ConnectionMessage{
				ChainID: c.PathEnd.ChainID,
				Action:  processor.MsgConnectionOpenInit,
				Info: provider.ConnectionInfo{
					ClientID:             c.PathEnd.ClientID,
					CounterpartyClientID: dst.PathEnd.ClientID,
				},
			},
			Termination: &processor.ConnectionMessage{
				ChainID: dst.PathEnd.ChainID,
				Action:  processor.MsgConnectionOpenConfirm,
				Info: provider.ConnectionInfo{
					ClientID:             dst.PathEnd.ClientID,
					CounterpartyClientID: c.PathEnd.ClientID,
				},
			},
		}).
		Build().
		Run(processorCtx)
}
