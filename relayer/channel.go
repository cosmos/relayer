package relayer

import (
	"context"
	"fmt"
	"time"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// CreateOpenChannels runs the channel creation messages on timeout until they pass.
func (c *Chain) CreateOpenChannels(
	ctx context.Context,
	dst *Chain,
	timeout time.Duration,
	srcPortID, dstPortID, order, version string,
) (modified bool, err error) {
	// client and connection identifiers must be filled in
	if err := ValidateConnectionPaths(c, dst); err != nil {
		return modified, err
	}

	// port identifiers and channel ORDER must be valid
	if err := ValidateChannelParams(srcPortID, dstPortID, order); err != nil {
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

	pp.OnChannelMessage(dst.PathEnd.ChainID, processor.MsgChannelOpenConfirm, func(ci provider.ChannelInfo) {
		// TODO write config changes
		modified = true
	})

	c.log.Info("Starting event processor for channel handshake",
		zap.String("src_chain_id", c.PathEnd.ChainID),
		zap.String("src_port_id", srcPortID),
		zap.String("dst_chain_id", dst.PathEnd.ChainID),
		zap.String("dst_port_id", dstPortID),
	)

	return modified, processor.NewEventProcessor().
		WithChainProcessors(
			srcPathChain.ChainProcessor(c.log),
			dstPathChain.ChainProcessor(c.log)).
		WithPathProcessors(pp).
		WithInitialBlockHistory(0).
		WithMessageLifecycle(&processor.ChannelMessageLifecycle{
			Initial: processor.ChannelMessage{
				ChainID: c.PathEnd.ChainID,
				Action:  processor.MsgChannelOpenInit,
				Info: provider.ChannelInfo{
					PortID:             srcPortID,
					CounterpartyPortID: dstPortID,
					ConnectionID:       c.PathEnd.ConnectionID,
					Version:            version,
					Order:              OrderFromString(order),
				},
			},
			Termination: processor.ChannelMessage{
				ChainID: dst.PathEnd.ChainID,
				Action:  processor.MsgChannelOpenConfirm,
				Info: provider.ChannelInfo{
					PortID:             dstPortID,
					CounterpartyPortID: srcPortID,
				},
			},
		}).
		Build().
		Run(processorCtx)
}

// CloseChannel runs the channel closing messages on timeout until they pass.
// TODO: add max retries or something to this function
func (c *Chain) CloseChannel(ctx context.Context, dst *Chain, to time.Duration, srcChanID, srcPortID string) error {
	srcPathChain := PathChain{
		Provider: c.ChainProvider,
		PathEnd:  processor.NewPathEnd(c.PathEnd.ChainID, c.PathEnd.ClientID, "", []processor.ChannelKey{}),
	}
	dstPathChain := PathChain{
		Provider: dst.ChainProvider,
		PathEnd:  processor.NewPathEnd(dst.PathEnd.ChainID, dst.PathEnd.ClientID, "", []processor.ChannelKey{}),
	}

	processorCtx, processorCtxCancel := context.WithTimeout(ctx, to)
	defer processorCtxCancel()

	return processor.NewEventProcessor().
		WithChainProcessors(
			srcPathChain.ChainProcessor(c.log),
			dstPathChain.ChainProcessor(c.log)).
		WithPathProcessors(processor.NewPathProcessor(
			c.log,
			srcPathChain.PathEnd,
			dstPathChain.PathEnd,
		)).
		WithInitialBlockHistory(0).
		WithMessageLifecycle(&processor.ChannelMessageLifecycle{
			Initial: processor.ChannelMessage{
				ChainID: c.PathEnd.ChainID,
				Action:  processor.MsgChannelCloseInit,
				Info: provider.ChannelInfo{
					PortID:    srcPortID,
					ChannelID: srcChanID,
				},
			},
			Termination: processor.ChannelMessage{
				ChainID: dst.PathEnd.ChainID,
				Action:  processor.MsgChannelCloseConfirm,
				Info: provider.ChannelInfo{
					CounterpartyPortID:    srcPortID,
					CounterpartyChannelID: srcChanID,
				},
			},
		}).
		Build().
		Run(processorCtx)
}

// ValidateChannelParams validates a set of port-ids as well as the order.
func ValidateChannelParams(srcPortID, dstPortID, order string) error {
	if err := host.PortIdentifierValidator(srcPortID); err != nil {
		return err
	}
	if err := host.PortIdentifierValidator(dstPortID); err != nil {
		return err
	}
	if (OrderFromString(order) == chantypes.ORDERED) || (OrderFromString(order) == chantypes.UNORDERED) {
		return nil
	}
	return fmt.Errorf("invalid order input (%s), order must be 'ordered' or 'unordered'", order)
}
