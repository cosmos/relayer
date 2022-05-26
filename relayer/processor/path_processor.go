package processor

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

// PathProcessor is a process that handles incoming IBC messages from a pair of chains.
// It determines what messages need to be relayed, and sends them.
type PathProcessor interface {

	// SetChainProcessorIfApplicable is used for creating linkage to a ChainProcessor.
	// It returns whether or not the provided chainID is applicable, so that the
	// ChainProcessor can also create a reference to this PathProcessor.
	// PathProcessors need reference to their ChainProcessors and vice-versa, handled by EventProcessorBuilder.Build().
	SetChainProcessorIfApplicable(chainID string, chainProcessor ChainProcessor) bool

	// Run will start the process to begin handling the backlog of IBC messages.
	Run(ctx context.Context)

	// HandleNewMessages will be called by the ChainProcessors to queue applicable messages for processing.
	HandleNewMessages(chainID string, channelKey ChannelKey, messages MessageCache)

	// TEST USE ONLY
	// PathEnd1Messages returns the cached messages after the PathProcessor ctx has been cancelled.
	PathEnd1Messages(message string) SequenceCache
	// PathEnd2Messages returns the cached messages after the PathProcessor ctx has been cancelled.
	PathEnd2Messages(message string) SequenceCache
}

// PathProcessors is a slice of PathProcessor instances
type PathProcessors []PathProcessor

// SequenceCache is used for caching an IBC message for a given packet sequence.
type SequenceCache map[uint64]provider.RelayerMessage

// MessageCache is used for caching a SequenceCache for a given IBC message type.
type MessageCache map[string]SequenceCache

// ChannelMessageCache is used for caching a MessageCache for a given IBC channel.
type ChannelMessageCache map[ChannelKey]MessageCache

// ChannelKey is the key used between ChainProcessor and PathProcessor.
type ChannelKey struct {
	ChannelID             string
	PortID                string
	CounterpartyChannelID string
	CounterpartyPortID    string
}

// Merge will merge an "other" MessageCache into this one.
func (c MessageCache) Merge(other MessageCache) {
	for k, v := range other {
		c[k] = v
	}
}

// Clone will create a deep copy of a MessageCache.
func (c MessageCache) Clone() MessageCache {
	newMessageCache := make(MessageCache)
	for mk, mv := range c {
		newSequenceCache := make(SequenceCache)
		for sk, sv := range mv {
			newSequenceCache[sk] = sv
		}
		newMessageCache[mk] = newSequenceCache
	}
	return newMessageCache
}
