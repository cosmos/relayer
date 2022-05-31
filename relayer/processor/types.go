package processor

import "github.com/cosmos/relayer/v2/relayer/provider"

// These are IBC message types used as shared message keys between ChainProcessors and PathProcessors.
const (
	// Packet messages
	MsgTransfer        = "/ibc.applications.transfer.v1.MsgTransfer"
	MsgRecvPacket      = "/ibc.core.channel.v1.MsgRecvPacket"
	MsgAcknowledgement = "/ibc.core.channel.v1.MsgAcknowledgement"
	MsgTimeout         = "/ibc.core.channel.v1.MsgTimeout"
	MsgTimeoutOnClose  = "/ibc.core.channel.v1.MsgTimeoutOnClose"

	// Connection messages
	MsgConnectionOpenInit    = "/ibc.core.connection.v1.MsgConnectionOpenInit"
	MsgConnectionOpenTry     = "/ibc.core.connection.v1.MsgConnectionOpenTry"
	MsgConnectionOpenAck     = "/ibc.core.connection.v1.MsgConnectionOpenAck"
	MsgConnectionOpenConfirm = "/ibc.core.connection.v1.MsgConnectionOpenConfirm"

	// Channel messages
	MsgChannelOpenInit    = "/ibc.core.channel.v1.MsgChannelOpenInit"
	MsgChannelOpenTry     = "/ibc.core.channel.v1.MsgChannelOpenTry"
	MsgChannelOpenAck     = "/ibc.core.channel.v1.MsgChannelOpenAck"
	MsgChannelOpenConfirm = "/ibc.core.channel.v1.MsgChannelOpenConfirm"

	MsgChannelCloseInit    = "/ibc.core.channel.v1.MsgChannelCloseInit"
	MsgChannelCloseConfirm = "/ibc.core.channel.v1.MsgChannelCloseConfirm"

	// Client messages
	MsgCreateClient       = "/ibc.core.client.v1.MsgCreateClient"
	MsgUpdateClient       = "/ibc.core.client.v1.MsgUpdateClient"
	MsgUpgradeClient      = "/ibc.core.client.v1.MsgUpgradeClient"
	MsgSubmitMisbehaviour = "/ibc.core.client.v1.MsgSubmitMisbehaviour"
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

func (c MessageCache) DeleteCachedMessages(toDelete ...map[string][]uint64) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, sequence := range toDeleteMessages {
				delete(c[message], sequence)
			}
		}
	}
}
