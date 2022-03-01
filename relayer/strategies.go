package relayer

import (
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
	go func() {
		for {
			select {
			case <-doneChan:
				return
			default:
				// Query the list of channels on the src & dst connections
				srcChannels, dstChannels, err := QueryChannelsOnBothConnections(src, dst)
				if err != nil {
					return
				}

				// Filter for open channels that are not already in our slices of open channels
				// TODO implement a filter list of channels we want to relay against or a list of channels to ignore
				var srcOpenChannels, dstOpenChannels []*ActiveChannel
				FilterOpenChannels(srcChannels, srcOpenChannels)
				FilterOpenChannels(dstChannels, dstOpenChannels)

				// Spin up a goroutine to relay packets & acks for each channel that isn't already being relayed against
				for _, channel := range srcOpenChannels {
					if !channel.active {
						channel.active = true
						go RelayUnrelayedPacketsAndAcks(src, dst, maxTxSize, maxMsgLength, channel.channel)
					}
				}

				for _, channel := range dstOpenChannels {
					if !channel.active {
						channel.active = true
						go RelayUnrelayedPacketsAndAcks(dst, src, maxTxSize, maxMsgLength, channel.channel)
					}
				}

				// Rerun the entire loop every 2 mins to check for new open channels
				// Ensures we aren't spamming the underlying nodes with `QueryConnectionChannels`
				time.Sleep(120 * time.Second)
			}
		}
	}()
	return func() { doneChan <- struct{}{} }, nil
}

// QueryChannelsOnBothConnections queries all the channels associated with a connection on both the src and dst chains
func QueryChannelsOnBothConnections(src, dst *Chain) ([]*types.IdentifiedChannel, []*types.IdentifiedChannel, error) {
	// Query the latest heights on src & dst
	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return nil, nil, err
	}

	// Query the list of channels for the connection on src
	var srcChannels, dstChannels []*types.IdentifiedChannel

	if err = retry.Do(func() error {
		srcChannels, err = src.ChainProvider.QueryConnectionChannels(srch, src.ConnectionID())
		return err
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.LogRetryQueryConnectionChannels(n, err, src.ConnectionID())
	})); err != nil {
		return nil, nil, err
	}

	// Query the list of channels for the connection on dst
	if err = retry.Do(func() error {
		dstChannels, err = dst.ChainProvider.QueryConnectionChannels(dsth, dst.ConnectionID())
		return err
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		dst.LogRetryQueryConnectionChannels(n, err, dst.ConnectionID())
	})); err != nil {
		return nil, nil, err
	}

	return srcChannels, dstChannels, nil
}

// FilterOpenChannels takes a slice of channels and adds all of the channels with OPEN state to a new slice of channels
// NOTE: channels will not be added to the slice of open channels more than once
func FilterOpenChannels(channels []*types.IdentifiedChannel, openChannels []*ActiveChannel) {
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
}

// RelayUnrelayedPacketsAndAcks will relay all the pending packets and acknowledgements on both the src and dst chains
func RelayUnrelayedPacketsAndAcks(src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) {
	RelayUnrelayedPackets(src, dst, maxTxSize, maxMsgLength, srcChannel)
	RelayUnrelayedAcks(src, dst, maxTxSize, maxMsgLength, srcChannel)
	time.Sleep(100 * time.Millisecond)
}

// RelayUnrelayedPackets fetches unrelayed packet sequence numbers and attempts to relay the associated packets
func RelayUnrelayedPackets(src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) {

	// Fetch any unrelayed sequences depending on the channel order
	sp, err := UnrelayedSequences(src, dst, srcChannel)
	if err != nil {
		src.Log(fmt.Sprintf("unrelayed sequences error: %s", err))
	} else {
		if len(sp.Src) > 0 && src.debug {
			src.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", src.ChainID(), sp.Src))
		}
		if len(sp.Dst) > 0 && dst.debug {
			dst.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", dst.ChainID(), sp.Dst))
		}
		if !sp.Empty() {
			if err = RelayPackets(src, dst, sp, maxTxSize, maxMsgLength, srcChannel); err != nil {
				src.Log(fmt.Sprintf("relay packets error: %s", err))
			}
		}
	}
}

// RelayUnrelayedAcks fetches unrelayed acknowledgements and attempts to relay them
func RelayUnrelayedAcks(src, dst *Chain, maxTxSize, maxMsgLength uint64, srcChannel *types.IdentifiedChannel) {

	// Fetch any unrelayed acks depending on the channel order
	ap, err := UnrelayedAcknowledgements(src, dst, srcChannel)
	if err != nil {
		src.Log(fmt.Sprintf("unrelayed acks error: %s", err))
	} else {
		if len(ap.Src) > 0 && src.debug {
			src.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", src.ChainID(), ap.Src))
		}
		if len(ap.Dst) > 0 && dst.debug {
			dst.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", dst.ChainID(), ap.Dst))
		}
		if !ap.Empty() {
			if err = RelayAcknowledgements(src, dst, ap, maxTxSize, maxMsgLength, srcChannel); err != nil && src.debug {
				src.Log(fmt.Sprintf("relay acks error: %s", err))
			}
		}
	}
}
