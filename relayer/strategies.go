package relayer

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

// ActiveChannel represents an IBC channel and whether there is an active goroutine relaying packets against it
type ActiveChannel struct {
	channel *types.IdentifiedChannel
	active  bool
}

// StartRelayer starts the main relaying loop
func StartRelayer(src, dst *Chain, maxTxSize, maxMsgLength uint64) (func(), error) {
	doneChan := make(chan struct{})
	activeChan := make(chan *ActiveChannel, 10)
	var srcOpenChannels []*ActiveChannel

	go func() {
		for {
			select {
			case <-doneChan:
				return
			default:
				// Query the list of channels on the src connection
				srcChannels, err := QueryChannelsOnConnection(src)
				if err != nil {
					return
				}

				// Filter for open channels that are not already in our slices of open channels
				// TODO implement a filter list of channels we want to relay against or a list of channels to ignore
				srcOpenChannels = FilterOpenChannels(srcChannels, srcOpenChannels)

				// Spin up a goroutine to relay packets & acks for each channel that isn't already being relayed against
				for _, channel := range srcOpenChannels {
					if !channel.active {
						channel.active = true
						fmt.Printf("STARTING GOROUTINE FOR CHANNEL ----- %s \n", channel.channel.ChannelId)
						go RelayUnrelayedPacketsAndAcks(src, dst, maxTxSize, maxMsgLength, channel, activeChan)
					}
				}

				for channel := range activeChan {
					// when a goroutine exits we need to see if that channel is still OPEN
					// we need to ensure the slice of open channels is maintained
					fmt.Printf("STOPPING GOROUTINE FOR CHANNEL ----- %s \n", channel.channel.ChannelId)
					channel.active = false
					break
				}

				// Make sure we are removing channels no longer in open state from the slice of open channels
				for i, channel := range srcOpenChannels {
					if channel.channel.State != types.OPEN {
						srcOpenChannels[i] = srcOpenChannels[len(srcOpenChannels)-1]
						srcOpenChannels = srcOpenChannels[:len(srcOpenChannels)-1]
					}
				}
			}
		}
	}()
	return func() { doneChan <- struct{}{} }, nil
}

// QueryChannelsOnConnection queries all the channels associated with a connection on the src chain
func QueryChannelsOnConnection(src *Chain) ([]*types.IdentifiedChannel, error) {
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

// FilterOpenChannels takes a slice of channels and adds all of the channels with OPEN state to a new slice of channels
// NOTE: channels will not be added to the slice of open channels more than once
func FilterOpenChannels(channels []*types.IdentifiedChannel, openChannels []*ActiveChannel) []*ActiveChannel {
	inSlice := false

	// Filter for open channels
	for _, channel := range channels {
		if channel.State == types.OPEN {

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

			inSlice = false
		}
	}

	return openChannels
}

// RelayUnrelayedPacketsAndAcks will relay all the pending packets and acknowledgements on both the src and dst chains
func RelayUnrelayedPacketsAndAcks(src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *ActiveChannel, activeChan chan<- *ActiveChannel) {
	// make goroutine signal its death, whether it's a panic or a return
	defer func() {
		activeChan <- srcChannel
	}()

	for {
		if err := RelayUnrelayedPackets(src, dst, maxTxSize, maxMsgLength, srcChannel.channel); err != nil {
			return
		}
		if err := RelayUnrelayedAcks(src, dst, maxTxSize, maxMsgLength, srcChannel.channel); err != nil {
			return
		}

		time.Sleep(1000 * time.Millisecond)
	}
}

// RelayUnrelayedPackets fetches unrelayed packet sequence numbers and attempts to relay the associated packets
func RelayUnrelayedPackets(src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch any unrelayed sequences depending on the channel order
	sp, err := UnrelayedSequences(src, dst, srcChannel)
	if err != nil {
		src.Log(fmt.Sprintf("unrelayed sequences error: %s", err))
		return err
	} else {
		if len(sp.Src) > 0 && src.debug {
			src.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", src.ChainID(), sp.Src))
		}
		if len(sp.Dst) > 0 && dst.debug {
			dst.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", dst.ChainID(), sp.Dst))
		}
		if !sp.Empty() {
			go RelayPackets(src, dst, sp, maxTxSize, maxMsgLength, srcChannel, ctx)

			select {
			case <-ctx.Done():
				src.Log(fmt.Sprintf("relay packets error: %s", ctx.Err()))
				return ctx.Err()
			}

		} else {
			src.Log(fmt.Sprintf("- No packets in the queue between [%s]port{%s} and [%s]port{%s}",
				src.ChainID(), srcChannel.PortId, dst.ChainID(), srcChannel.Counterparty.PortId))
		}
	}

	return nil
}

// RelayUnrelayedAcks fetches unrelayed acknowledgements and attempts to relay them
func RelayUnrelayedAcks(src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch any unrelayed acks depending on the channel order
	ap, err := UnrelayedAcknowledgements(src, dst, srcChannel)
	if err != nil {
		src.Log(fmt.Sprintf("unrelayed acks error: %s", err))
		return err
	} else {
		if len(ap.Src) > 0 && src.debug {
			src.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", src.ChainID(), ap.Src))
		}
		if len(ap.Dst) > 0 && dst.debug {
			dst.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", dst.ChainID(), ap.Dst))
		}
		if !ap.Empty() {
			go RelayAcknowledgements(src, dst, ap, maxTxSize, maxMsgLength, srcChannel, ctx)

			select {
			case <-ctx.Done():
				src.Log(fmt.Sprintf("relay acks error: %s", ctx.Err()))
				return ctx.Err()
			}

		} else {
			src.Log(fmt.Sprintf("- No acks in the queue between [%s]port{%s} and [%s]port{%s}",
				src.ChainID(), srcChannel.PortId, dst.ChainID(), srcChannel.Counterparty.PortId))
		}
	}

	return nil
}
