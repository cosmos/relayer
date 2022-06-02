package relayer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"go.uber.org/zap"
)

// ActiveChannel represents an IBC channel and whether there is an active goroutine relaying packets against it.
type ActiveChannel struct {
	channel *types.IdentifiedChannel
	active  bool
}

// StartRelayer starts the main relaying loop and returns a channel that will contain any control-flow related errors.
func StartRelayer(ctx context.Context, log *zap.Logger, src, dst *Chain, filter ChannelFilter, maxTxSize, maxMsgLength uint64) chan error {
	errorChan := make(chan error, 1)

	go relayerMainLoop(ctx, log, src, dst, filter, maxTxSize, maxMsgLength, errorChan)
	return errorChan
}

// relayerMainLoop is the main loop of the relayer.
func relayerMainLoop(ctx context.Context, log *zap.Logger, src, dst *Chain, filter ChannelFilter, maxTxSize, maxMsgLength uint64, errCh chan<- error) {
	defer close(errCh)

	channels := make(chan *ActiveChannel, 10)
	var (
		srcOpenChannels []*ActiveChannel
		srcChannels     []*types.IdentifiedChannel
		err             error
	)

	for loopCount := 0; true; loopCount++ {
		// query for channel changes every 20 loops, or if the number of open channels is zero (for quicker startup)
		if loopCount%20 == 0 || len(srcOpenChannels) == 0 {
			// Query the list of channels on the src connection
			srcChannels, err = queryChannelsOnConnection(ctx, src)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					errCh <- err
				} else {
					errCh <- fmt.Errorf("error querying all channels on chain{%s}@connection{%s}: %w",
						src.ChainID(), src.ConnectionID(), err)
				}
				return
			}

			src.log.Info("Number of channels", zap.Int("num_channels", len(srcChannels)))

			// Apply the channel filter rule (i.e. build allowlist, denylist or relay on all channels available)
			srcChannels = applyChannelFilterRule(filter, srcChannels)
		}

		// Ugly way to handle opening a channel for interchain accounts
		for _, c := range srcChannels {
			src.log.Info("Inside for loop of channels")
			if c.State == types.INIT {
				src.log.Info("Inside test case for channel state == INIT")
				_, err := src.CreateOpenChannels(ctx, dst, 10, 5*time.Second, c.PortId, c.Counterparty.PortId, StringFromOrder(c.Ordering), c.Version, false)
				if err != nil {
					src.log.Warn("Failed to open channel",
						zap.String("src-channel-id", c.ChannelId),
						zap.String("src-port-id", c.PortId),
						zap.String("dst-channel-id", c.Counterparty.ChannelId),
						zap.String("dst-port-id", c.Counterparty.ChannelId),
						zap.String("channel-version", c.Version),
						zap.String("channel-ordering", c.Ordering.String()),
						zap.Error(err))
				}
			}
		}

		// Filter for open channels that are not already in our slice of open channels
		srcOpenChannels = filterOpenChannels(srcChannels, srcOpenChannels)

		// TODO once upstream changes are merged for emitting the channel version in ibc-go,
		// we will want to add back logic for finishing the channel handshake for interchain accounts.
		// Essentially the interchain accounts module will initiate the handshake and then the relayer finishes it.
		// So we will occasionally query recent txs and check the events for `ChannelOpenInit`, at which point
		// we will attempt to finish opening the channel.

		if len(srcOpenChannels) == 0 {
			continue
		}

		var wg sync.WaitGroup
		// Spin up a goroutine to relay packets & acks for each channel that isn't already being relayed against
		for _, channel := range srcOpenChannels {
			if !channel.active {
				channel.active = true
				wg.Add(1)
				go relayUnrelayedPacketsAndAcks(ctx, log, &wg, src, dst, maxTxSize, maxMsgLength, channel, channels)
			}
		}
		wg.Wait()

		for channel := range channels {
			channel.active = false
			break
		}

		// Make sure we are removing channels no longer in OPEN state from the slice of open channels
		for i, channel := range srcOpenChannels {
			if channel.channel.State != types.OPEN {
				srcOpenChannels[i] = srcOpenChannels[len(srcOpenChannels)-1]
				srcOpenChannels = srcOpenChannels[:len(srcOpenChannels)-1]
			}
		}
	}
}

// queryChannelsOnConnection queries all the channels associated with a connection on the src chain.
func queryChannelsOnConnection(ctx context.Context, src *Chain) ([]*types.IdentifiedChannel, error) {
	// Query the latest heights on src & dst
	srch, err := src.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	// Query the list of channels for the connection on src
	var srcChannels []*types.IdentifiedChannel

	if err = retry.Do(func() error {
		srcChannels, err = src.ChainProvider.QueryConnectionChannels(ctx, srch, src.ConnectionID())
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.log.Info(
			"Failed to query connection channels",
			zap.String("conn_id", src.ConnectionID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return nil, err
	}

	return srcChannels, nil
}

// filterOpenChannels takes a slice of channels and adds all the channels with OPEN state to a new slice of channels.
// NOTE: channels will not be added to the slice of open channels more than once.
func filterOpenChannels(channels []*types.IdentifiedChannel, openChannels []*ActiveChannel) []*ActiveChannel {
	// Filter for open channels
	for _, channel := range channels {
		if channel.State == types.OPEN {
			inSlice := false

			// Check if we have already added this channel to the slice of open channels
			for _, openChannel := range openChannels {
				if channel.ChannelId == openChannel.channel.ChannelId {
					inSlice = true
					break
				}
			}

			// We don't want to add channels to the slice of open channels that have already been added
			if !inSlice {
				openChannels = append(openChannels, &ActiveChannel{
					channel: channel,
					active:  false,
				})
			}
		}
	}

	return openChannels
}

// applyChannelFilterRule will use the given ChannelFilter's rule and channel list to build the appropriate list of
// channels to relay on.
func applyChannelFilterRule(filter ChannelFilter, channels []*types.IdentifiedChannel) []*types.IdentifiedChannel {
	switch filter.Rule {
	case allowList:
		var filteredChans []*types.IdentifiedChannel
		for _, c := range channels {
			if filter.InChannelList(c.ChannelId) {
				filteredChans = append(filteredChans, c)
			}
		}
		return filteredChans
	case denyList:
		var filteredChans []*types.IdentifiedChannel
		for _, c := range channels {
			if filter.InChannelList(c.ChannelId) {
				continue
			}
			filteredChans = append(filteredChans, c)
		}
		return filteredChans
	default:
		// handle all channels on connection
		return channels
	}
}

// relayUnrelayedPacketsAndAcks will relay all the pending packets and acknowledgements on both the src and dst chains.
func relayUnrelayedPacketsAndAcks(ctx context.Context, log *zap.Logger, wg *sync.WaitGroup, src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *ActiveChannel, channels chan<- *ActiveChannel) {
	defer wg.Done()

	// make goroutine signal its death, whether it's a panic or a return
	defer func() {
		channels <- srcChannel
	}()

	for {
		if ok := relayUnrelayedPackets(ctx, log, src, dst, maxTxSize, maxMsgLength, srcChannel.channel); !ok {
			return
		}
		if ok := relayUnrelayedAcks(ctx, log, src, dst, maxTxSize, maxMsgLength, srcChannel.channel); !ok {
			return
		}

		// Wait for a second before continuing, but allow context cancellation to break the flow.
		select {
		case <-time.After(time.Second):
			// Nothing to do.
		case <-ctx.Done():
			return
		}
	}
}

// relayUnrelayedPackets fetches unrelayed packet sequence numbers and attempts to relay the associated packets.
// relayUnrelayedPackets returns true if packets were empty or were successfully relayed.
// Otherwise, it logs the errors and returns false.
func relayUnrelayedPackets(ctx context.Context, log *zap.Logger, src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) bool {
	// Fetch any unrelayed sequences depending on the channel order
	sp := UnrelayedSequences(ctx, src, dst, srcChannel)

	// If there are no unrelayed packets, stop early.
	if sp.Empty() {
		src.log.Debug(
			"No packets in queue",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_channel_id", srcChannel.ChannelId),
			zap.String("src_port_id", srcChannel.PortId),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
			zap.String("dst_port_id", srcChannel.Counterparty.PortId),
		)
		return true
	}

	if len(sp.Src) > 0 {
		src.log.Info(
			"Unrelayed source packets",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_channel_id", srcChannel.ChannelId),
			zap.Uint64s("seqs", sp.Src),
		)
	}

	if len(sp.Dst) > 0 {
		src.log.Info(
			"Unrelayed destination packets",
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
			zap.Uint64s("seqs", sp.Dst),
		)
	}

	if err := RelayPackets(ctx, log, src, dst, sp, maxTxSize, maxMsgLength, srcChannel); err != nil {
		// If there was a context cancellation or deadline while attempting to relay packets,
		// log that and indicate failure.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Warn(
				"Context finished while waiting for RelayPackets to complete",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_channel_id", srcChannel.ChannelId),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
				zap.Error(ctx.Err()),
			)
			return false
		}

		// If we encounter an error that suggest node configuration issues, log a more insightful error message.
		if strings.Contains(err.Error(), "Internal error: transaction indexing is disabled") {
			log.Warn(
				"Remote server needs reconfigured.",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_channel_id", srcChannel.ChannelId),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
				zap.Error(ctx.Err()),
			)
			return false
		}

		// Otherwise, not a context error, but an application-level error.
		log.Warn(
			"Relay packets error",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_channel_id", srcChannel.ChannelId),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
			zap.Error(err),
		)
		// Indicate that we should attempt to keep going.
		return true
	}

	return true
}

// relayUnrelayedAcks fetches unrelayed acknowledgements and attempts to relay them.
// relayUnrelayedAcks returns true if acknowledgements were empty or were successfully relayed.
// Otherwise, it logs the errors and returns false.
func relayUnrelayedAcks(ctx context.Context, log *zap.Logger, src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) bool {
	// Fetch any unrelayed acks depending on the channel order
	ap := UnrelayedAcknowledgements(ctx, src, dst, srcChannel)

	// If there are no unrelayed acks, stop early.
	if ap.Empty() {
		log.Debug(
			"No acknowledgements in queue",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_channel_id", srcChannel.ChannelId),
			zap.String("src_port_id", srcChannel.PortId),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
			zap.String("dst_port_id", srcChannel.Counterparty.PortId),
		)
		return true
	}

	if len(ap.Src) > 0 {
		log.Info(
			"Unrelayed source acknowledgements",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_channel_id", srcChannel.ChannelId),
			zap.Uint64s("acks", ap.Src),
		)
	}

	if len(ap.Dst) > 0 {
		log.Info(
			"Unrelayed destination acknowledgements",
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
			zap.Uint64s("acks", ap.Dst),
		)
	}

	if err := RelayAcknowledgements(ctx, log, src, dst, ap, maxTxSize, maxMsgLength, srcChannel); err != nil {
		// If there was a context cancellation or deadline while attempting to relay acknowledgements,
		// log that and indicate failure.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Warn(
				"Context finished while waiting for RelayAcknowledgements to complete",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_channel_id", srcChannel.ChannelId),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
				zap.Error(ctx.Err()),
			)
			return false
		}

		// Otherwise, not a context error, but an application-level error.
		log.Warn(
			"Relay acknowledgements error",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_channel_id", srcChannel.ChannelId),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_channel_id", srcChannel.Counterparty.ChannelId),
			zap.Error(err),
		)
		return false
	}

	return true
}
