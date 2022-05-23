package relayer

import (
	"context"
	"fmt"
	"reflect"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	cosmosProcessor "github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/chains/processor"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/paths"
	cosmosProvider "github.com/cosmos/relayer/v2/relayer/provider/cosmos"
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

func getChainProcessorForProvider(ctx context.Context, log *zap.Logger, chain *Chain, pathProcessor *paths.PathProcessor) processor.ChainProcessor {
	switch chain.ChainProvider.(type) {
	case *cosmosProvider.CosmosProvider:
		chainProcessor, err := cosmosProcessor.NewCosmosChainProcessor(ctx, log, chain.RPCAddr, chain.ChainProvider, pathProcessor)
		if err != nil {
			panic(err)
		}
		return chainProcessor
	default:
		panic(fmt.Errorf("unable to get chain processor, unsupported chain provider type: %s", reflect.TypeOf(chain.ChainProvider)))
	}
}

// relayerMainLoop is the main loop of the relayer.
func relayerMainLoop(ctx context.Context, log *zap.Logger, src, dst *Chain, filter ChannelFilter, maxTxSize, maxMsgLength uint64, errCh chan<- error) {
	defer close(errCh)

	allowedChannelsSrc := []ibc.ChannelKey{}
	allowedChannelsDst := []ibc.ChannelKey{}
	blockedChannelsSrc := []ibc.ChannelKey{}
	blockedChannelsDst := []ibc.ChannelKey{}

	for _, ch := range filter.ChannelList {
		channelRuleSrc := ibc.ChannelKey{ChannelID: ch}
		channelRuleDst := ibc.ChannelKey{CounterpartyChannelID: ch}
		if filter.Rule == allowList {
			allowedChannelsSrc = append(allowedChannelsSrc, channelRuleSrc)
			allowedChannelsDst = append(allowedChannelsDst, channelRuleDst)
		} else if filter.Rule == denyList {
			blockedChannelsSrc = append(blockedChannelsSrc, channelRuleSrc)
			blockedChannelsDst = append(blockedChannelsDst, channelRuleDst)
		}
	}

	pathEnd1 := paths.NewPathEnd(src.ChainID(), src.ClientID(), src.ConnectionID(), allowedChannelsSrc, blockedChannelsSrc)
	pathEnd2 := paths.NewPathEnd(dst.ChainID(), dst.ClientID(), dst.ConnectionID(), allowedChannelsDst, blockedChannelsDst)

	pathProcessor := paths.NewPathProcessor(ctx, log, pathEnd1, pathEnd2)

	chainProcessor1 := getChainProcessorForProvider(ctx, log, src, pathProcessor)
	chainProcessor2 := getChainProcessorForProvider(ctx, log, dst, pathProcessor)

	pathProcessor.SetPathEnd1ChainProcessor(chainProcessor1)
	pathProcessor.SetPathEnd2ChainProcessor(chainProcessor2)

	processor.ChainProcessors([]processor.ChainProcessor{chainProcessor1, chainProcessor2}).Start(ctx, errCh)
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
