package relayer

import (
	"context"
	"time"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
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
	initialBlockHistory uint64,
	pathName string,
) (string, string, error) {
	// client identifiers must be filled in
	if err := ValidateClientPaths(c, dst); err != nil {
		return "", "", err
	}

	// Timeout is per message. Four connection handshake messages, allowing maxRetries for each.
	processorTimeout := timeout * 4 * time.Duration(maxRetries)

	ctx, cancel := context.WithTimeout(ctx, processorTimeout)
	defer cancel()

	pp := processor.NewPathProcessor(
		c.log,
		processor.NewPathEnd(pathName, c.PathEnd.ChainID, c.PathEnd.ClientID, "", []processor.ChainChannelKey{}),
		processor.NewPathEnd(pathName, dst.PathEnd.ChainID, dst.PathEnd.ClientID, "", []processor.ChainChannelKey{}),
		nil,
		memo,
		DefaultClientUpdateThreshold,
		DefaultFlushInterval,
		DefaultMaxMsgLength,
	)

	var connectionSrc, connectionDst string

	pp.OnConnectionMessage(dst.PathEnd.ChainID, conntypes.EventTypeConnectionOpenConfirm, func(ci provider.ConnectionInfo) {
		dst.PathEnd.ConnectionID = ci.ConnID
		c.PathEnd.ConnectionID = ci.CounterpartyConnID
		connectionSrc = ci.CounterpartyConnID
		connectionDst = ci.ConnID
	})

	c.log.Info("Starting event processor for connection handshake",
		zap.String("src_chain_id", c.PathEnd.ChainID),
		zap.String("src_client_id", c.PathEnd.ClientID),
		zap.String("dst_chain_id", dst.PathEnd.ChainID),
		zap.String("dst_client_id", dst.PathEnd.ClientID),
	)

	return connectionSrc, connectionDst, processor.NewEventProcessor().
		WithChainProcessors(
			c.chainProcessor(c.log, nil),
			dst.chainProcessor(c.log, nil),
		).
		WithPathProcessors(pp).
		WithInitialBlockHistory(initialBlockHistory).
		WithMessageLifecycle(&processor.ConnectionMessageLifecycle{
			Initial: &processor.ConnectionMessage{
				ChainID:   c.PathEnd.ChainID,
				EventType: conntypes.EventTypeConnectionOpenInit,
				Info: provider.ConnectionInfo{
					ClientID:                     c.PathEnd.ClientID,
					CounterpartyClientID:         dst.PathEnd.ClientID,
					CounterpartyCommitmentPrefix: dst.ChainProvider.CommitmentPrefix(),
				},
			},
			Termination: &processor.ConnectionMessage{
				ChainID:   dst.PathEnd.ChainID,
				EventType: conntypes.EventTypeConnectionOpenConfirm,
				Info: provider.ConnectionInfo{
					ClientID:                     dst.PathEnd.ClientID,
					CounterpartyClientID:         c.PathEnd.ClientID,
					CounterpartyCommitmentPrefix: c.ChainProvider.CommitmentPrefix(),
				},
			},
		}).
		Build().
		Run(ctx)
}
