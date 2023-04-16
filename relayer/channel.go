package relayer

import (
	"context"
	"fmt"
	"time"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

func newPathEnd(pathName, chainID, clientID string) processor.PathEnd {
	return processor.NewPathEnd(pathName, chainID, clientID, "", []processor.ChainChannelKey{})
}

func newRelayPathEnds(pathName string, hops []*Chain) ([]*processor.PathEnd, []*processor.PathEnd) {
	relayPathEndsSrcToDst := make([]*processor.PathEnd, len(hops))
	relayPathEndsDstToSrc := make([]*processor.PathEnd, len(hops))
	// RelayPathEnds are set in user friendly order so they're just listed as they appear left to right without
	// acounting for directionality. So for a 1 hop case they would look like this:
	// A -> B (BA, BC) -> C
	// Here we want to account for directionality left to right so we want to return:
	// BC, BA
	// Hence the index reversal in the call to newPathEnd().
	for i, hop := range hops {
		pathEnd1 := newPathEnd(pathName, hop.RelayPathEnds[1].ChainID, hop.RelayPathEnds[1].ClientID)
		pathEnd2 := newPathEnd(pathName, hop.RelayPathEnds[0].ChainID, hop.RelayPathEnds[0].ClientID)
		relayPathEndsSrcToDst[i] = &pathEnd1
		relayPathEndsDstToSrc[i] = &pathEnd2
	}
	return relayPathEndsSrcToDst, relayPathEndsDstToSrc
}

// CreateOpenChannels runs the channel creation messages on timeout until they pass.
func (c *Chain) CreateOpenChannels(
	ctx context.Context,
	dst *Chain,
	hops []*Chain,
	maxRetries uint64,
	timeout time.Duration,
	srcPortID, dstPortID, order, version string,
	override bool,
	memo string,
	pathName string,
) error {
	// client and connection identifiers must be filled in
	if err := ValidateConnectionPaths(c, dst); err != nil {
		return err
	}

	// port identifiers and channel ORDER must be valid
	if err := ValidateChannelParams(srcPortID, dstPortID, order); err != nil {
		return err
	}

	if !override {
		channel, err := QueryPortChannel(ctx, c, srcPortID)
		if err == nil && channel != nil {
			return fmt.Errorf("channel {%s} with port {%s} already exists on chain {%s}", channel.ChannelId, channel.PortId, c.ChainID())
		}

		channel, err = QueryPortChannel(ctx, dst, dstPortID)
		if err == nil && channel != nil {
			return fmt.Errorf("channel {%s} with port {%s} already exists on chain {%s}", channel.ChannelId, channel.PortId, dst.ChainID())
		}
	}

	// Timeout is per message. Four channel handshake messages, allowing maxRetries for each.
	processorTimeout := timeout * 4 * time.Duration(maxRetries)

	ctx, cancel := context.WithTimeout(ctx, processorTimeout)
	defer cancel()

	relayPathEndsSrcToDst, relayPathEndsDstToSrc := newRelayPathEnds(pathName, hops)
	pp := processor.NewPathProcessor(
		c.log,
		newPathEnd(pathName, c.PathEnd.ChainID, c.PathEnd.ClientID),
		newPathEnd(pathName, dst.PathEnd.ChainID, dst.PathEnd.ClientID),
		relayPathEndsSrcToDst,
		relayPathEndsDstToSrc,
		nil,
		memo,
		DefaultClientUpdateThreshold,
		DefaultFlushInterval,
	)

	c.log.Info("Starting event processor for channel handshake",
		zap.String("src_chain_id", c.PathEnd.ChainID),
		zap.String("src_port_id", srcPortID),
		zap.String("dst_chain_id", dst.PathEnd.ChainID),
		zap.String("dst_port_id", dstPortID),
	)
	hopConnectionIDs := make([]string, len(hops)+1)
	hopConnectionIDs[0] = c.PathEnd.ConnectionID
	for i, hop := range hops {
		hopConnectionIDs[i+1] = hop.RelayPathEnds[1].ConnectionID
	}
	connectionHops := chantypes.FormatConnectionID(hopConnectionIDs)
	openInitMsg := &processor.ChannelMessage{
		ChainID:   c.PathEnd.ChainID,
		EventType: chantypes.EventTypeChannelOpenInit,
		Info: provider.ChannelInfo{
			PortID:             srcPortID,
			CounterpartyPortID: dstPortID,
			ConnID:             connectionHops,
			Version:            version,
			Order:              OrderFromString(order),
		},
	}
	c.log.Info("Initializing channel",
		zap.String("chain_id", c.PathEnd.ChainID),
		zap.String("port_id", srcPortID),
		zap.String("conn_id", connectionHops),
	)
	chainProcessors := []processor.ChainProcessor{
		c.chainProcessor(c.log, nil),
		dst.chainProcessor(c.log, nil),
	}
	for _, hop := range hops {
		chainProcessors = append(chainProcessors, hop.chainProcessor(c.log, nil))
	}
	return processor.NewEventProcessor().
		WithChainProcessors(chainProcessors...).
		WithPathProcessors(pp).
		WithInitialBlockHistory(0).
		WithMessageLifecycle(&processor.ChannelMessageLifecycle{
			Initial: openInitMsg,
			Termination: &processor.ChannelMessage{
				ChainID:   dst.PathEnd.ChainID,
				EventType: chantypes.EventTypeChannelOpenConfirm,
				Info: provider.ChannelInfo{
					PortID:             dstPortID,
					CounterpartyPortID: srcPortID,
				},
			},
		}).
		Build().
		Run(ctx)
}

// CloseChannel runs the channel closing messages on timeout until they pass.
func (c *Chain) CloseChannel(
	ctx context.Context,
	dst *Chain,
	hops []*Chain,
	maxRetries uint64,
	timeout time.Duration,
	srcChanID,
	srcPortID string,
	memo string,
	pathName string,
) error {
	// Timeout is per message. Two close channel handshake messages, allowing maxRetries for each.
	processorTimeout := timeout * 2 * time.Duration(maxRetries)

	ctx, cancel := context.WithTimeout(ctx, processorTimeout)
	defer cancel()
	relayPathEndsSrcToDst, relayPathEndsDstToSrc := newRelayPathEnds(pathName, hops)
	chainProcessors := []processor.ChainProcessor{
		c.chainProcessor(c.log, nil),
		dst.chainProcessor(c.log, nil),
	}
	for _, hop := range hops {
		chainProcessors = append(chainProcessors, hop.chainProcessor(c.log, nil))
	}
	return processor.NewEventProcessor().
		WithChainProcessors(chainProcessors...).
		WithPathProcessors(processor.NewPathProcessor(
			c.log,
			newPathEnd(pathName, c.PathEnd.ChainID, c.PathEnd.ClientID),
			newPathEnd(pathName, dst.PathEnd.ChainID, dst.PathEnd.ClientID),
			relayPathEndsSrcToDst,
			relayPathEndsDstToSrc,
			nil,
			memo,
			DefaultClientUpdateThreshold,
			DefaultFlushInterval,
		)).
		WithInitialBlockHistory(0).
		WithMessageLifecycle(&processor.ChannelMessageLifecycle{
			Initial: &processor.ChannelMessage{
				ChainID:   c.PathEnd.ChainID,
				EventType: chantypes.EventTypeChannelCloseInit,
				Info: provider.ChannelInfo{
					PortID:    srcPortID,
					ChannelID: srcChanID,
				},
			},
			Termination: &processor.ChannelMessage{
				ChainID:   dst.PathEnd.ChainID,
				EventType: chantypes.EventTypeChannelCloseConfirm,
				Info: provider.ChannelInfo{
					CounterpartyPortID:    srcPortID,
					CounterpartyChannelID: srcChanID,
				},
			},
		}).
		Build().
		Run(ctx)
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
