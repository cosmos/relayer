package processor

import "github.com/cosmos/relayer/v2/relayer/provider"

// These are IBC message types used as shared message keys between ChainProcessors and PathProcessors.
const (
	MsgTransfer        = "/ibc.applications.transfer.v1.MsgTransfer"
	MsgRecvPacket      = "/ibc.core.channel.v1.MsgRecvPacket"
	MsgAcknowledgement = "/ibc.core.channel.v1.MsgAcknowledgement"
)

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

// Merge will merge another MessageCache into this one.
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
