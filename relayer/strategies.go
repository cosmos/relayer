package relayer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

// ActiveChannel represents an IBC channel and whether there is an active goroutine relaying packets against it.
type ActiveChannel struct {
	channel *types.IdentifiedChannel
	active  bool
}

// StartRelayer starts the main relaying loop.
func StartRelayer(ctx context.Context, src, dst *Chain, filter *ChannelFilter, maxTxSize, maxMsgLength uint64) chan error {
	errorChan := make(chan error)
	channels := make(chan *ActiveChannel, 10)
	var srcOpenChannels []*ActiveChannel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Query the list of channels on the src connection
				srcChannels, err := queryChannelsOnConnection(src)
				if err != nil {
					errorChan <- fmt.Errorf("error querying all channels on chain{%s}@connection{%s}: %v \n",
						src.ChainID(), src.ConnectionID(), err)
					return
				}

				// Apply the channel filter rule (i.e. build allowlist, denylist or relay on all channels available)
				srcChannels = applyChannelFilterRule(filter, srcChannels)

				// Filter for open channels that are not already in our slice of open channels
				srcOpenChannels = filterOpenChannels(srcChannels, srcOpenChannels)

				// Spin up a goroutine to relay packets & acks for each channel that isn't already being relayed against
				for _, channel := range srcOpenChannels {
					if !channel.active {
						channel.active = true
						go relayUnrelayedPacketsAndAcks(ctx, src, dst, maxTxSize, maxMsgLength, channel, channels)
					}
				}

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
	}()
	return errorChan
}

// queryChannelsOnConnection queries all the channels associated with a connection on the src chain.
func queryChannelsOnConnection(src *Chain) ([]*types.IdentifiedChannel, error) {
	// Query the latest heights on src & dst
	srch, err := src.ChainProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	// Query the list of channels for the connection on src
	var srcChannels []*types.IdentifiedChannel

	if err = retry.Do(func() error {
		srcChannels, err = src.ChainProvider.QueryConnectionChannels(srch, src.ConnectionID())
		return err
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.LogRetryQueryConnectionChannels(n, err, src.ConnectionID())
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
func applyChannelFilterRule(filter *ChannelFilter, channels []*types.IdentifiedChannel) []*types.IdentifiedChannel {
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
func relayUnrelayedPacketsAndAcks(ctx context.Context, src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *ActiveChannel, channels chan<- *ActiveChannel) {
	// make goroutine signal its death, whether it's a panic or a return
	defer func() {
		channels <- srcChannel
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := relayUnrelayedPackets(ctx, src, dst, maxTxSize, maxMsgLength, srcChannel.channel); err != nil {
				return
			}
			if err := relayUnrelayedAcks(ctx, src, dst, maxTxSize, maxMsgLength, srcChannel.channel); err != nil {
				return
			}

			time.Sleep(1000 * time.Millisecond)
		}
	}
}

// relayUnrelayedPackets fetches unrelayed packet sequence numbers and attempts to relay the associated packets.
func relayUnrelayedPackets(ctx context.Context, src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) error {
	childCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Fetch any unrelayed sequences depending on the channel order
	sp, err := UnrelayedSequences(src, dst, srcChannel)
	if err != nil {
		src.Log(fmt.Sprintf("unrelayed sequences error: %s", err))
		return err
	}

	if len(sp.Src) > 0 && src.debug {
		src.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", src.ChainID(), sp.Src))
	}

	if len(sp.Dst) > 0 && dst.debug {
		dst.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", dst.ChainID(), sp.Dst))
	}

	if !sp.Empty() {
		go func() {
			err := RelayPackets(childCtx, src, dst, sp, maxTxSize, maxMsgLength, srcChannel)
			if err != nil {
				src.Log(fmt.Sprintf("relay packets error: %s", err))
			}
			cancel()
		}()

		// Wait until the context is cancelled (i.e. RelayPackets() finishes) or the context times out
		<-childCtx.Done()
		if !errors.Is(childCtx.Err(), context.Canceled) {
			src.Log(fmt.Sprintf("relay packets error: %s", childCtx.Err()))
			return childCtx.Err()
		}

	} else {
		src.Log(fmt.Sprintf("- No packets in the queue between [%s]port{%s} and [%s]port{%s}",
			src.ChainID(), srcChannel.PortId, dst.ChainID(), srcChannel.Counterparty.PortId))
	}

	return nil
}

// relayUnrelayedAcks fetches unrelayed acknowledgements and attempts to relay them.
func relayUnrelayedAcks(ctx context.Context, src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) error {
	childCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Fetch any unrelayed acks depending on the channel order
	ap, err := UnrelayedAcknowledgements(src, dst, srcChannel)
	if err != nil {
		src.Log(fmt.Sprintf("unrelayed acks error: %s", err))
		return err
	}

	if len(ap.Src) > 0 && src.debug {
		src.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", src.ChainID(), ap.Src))
	}

	if len(ap.Dst) > 0 && dst.debug {
		dst.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", dst.ChainID(), ap.Dst))
	}

	if !ap.Empty() {
		go func() {
			err := RelayAcknowledgements(childCtx, src, dst, ap, maxTxSize, maxMsgLength, srcChannel)
			if err != nil {
				src.Log(fmt.Sprintf("relay acks error: %s", err))
			}
			cancel()
		}()

		// Wait until the context is cancelled (i.e. RelayAcknowledgements() finishes) or the context times out
		<-childCtx.Done()
		if !errors.Is(childCtx.Err(), context.Canceled) {
			src.Log(fmt.Sprintf("relay acks error: %s", childCtx.Err()))
			return childCtx.Err()
		}

	} else {
		src.Log(fmt.Sprintf("- No acks in the queue between [%s]port{%s} and [%s]port{%s}",
			src.ChainID(), srcChannel.PortId, dst.ChainID(), srcChannel.Counterparty.PortId))
	}

	return nil
}
