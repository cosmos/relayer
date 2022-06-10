package processor

import (
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
)

// These are the IBC message types that are the possible message actions when parsing tendermint events.
// They are also used as shared message keys between ChainProcessors and PathProcessors.
var (
	// Packet messages
	MsgTransfer        = "/" + proto.MessageName((*transfertypes.MsgTransfer)(nil))
	MsgRecvPacket      = "/" + proto.MessageName((*chantypes.MsgRecvPacket)(nil))
	MsgAcknowledgement = "/" + proto.MessageName((*chantypes.MsgAcknowledgement)(nil))
	MsgTimeout         = "/" + proto.MessageName((*chantypes.MsgTimeout)(nil))
	MsgTimeoutOnClose  = "/" + proto.MessageName((*chantypes.MsgTimeoutOnClose)(nil))

	// Connection messages
	MsgConnectionOpenInit    = "/" + proto.MessageName((*conntypes.MsgConnectionOpenInit)(nil))
	MsgConnectionOpenTry     = "/" + proto.MessageName((*conntypes.MsgConnectionOpenTry)(nil))
	MsgConnectionOpenAck     = "/" + proto.MessageName((*conntypes.MsgConnectionOpenAck)(nil))
	MsgConnectionOpenConfirm = "/" + proto.MessageName((*conntypes.MsgConnectionOpenConfirm)(nil))

	// Channel messages
	MsgChannelOpenInit    = "/" + proto.MessageName((*chantypes.MsgChannelOpenInit)(nil))
	MsgChannelOpenTry     = "/" + proto.MessageName((*chantypes.MsgChannelOpenTry)(nil))
	MsgChannelOpenAck     = "/" + proto.MessageName((*chantypes.MsgChannelOpenAck)(nil))
	MsgChannelOpenConfirm = "/" + proto.MessageName((*chantypes.MsgChannelOpenConfirm)(nil))

	MsgChannelCloseInit    = "/" + proto.MessageName((*chantypes.MsgChannelCloseInit)(nil))
	MsgChannelCloseConfirm = "/" + proto.MessageName((*chantypes.MsgChannelCloseConfirm)(nil))

	// Client messages
	MsgCreateClient       = "/" + proto.MessageName((*clienttypes.MsgCreateClient)(nil))
	MsgUpdateClient       = "/" + proto.MessageName((*clienttypes.MsgUpdateClient)(nil))
	MsgUpgradeClient      = "/" + proto.MessageName((*clienttypes.MsgUpgradeClient)(nil))
	MsgSubmitMisbehaviour = "/" + proto.MessageName((*clienttypes.MsgSubmitMisbehaviour)(nil))
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

// Counterparty flips a ChannelKey for the perspective of the counterparty chain
func (channelKey ChannelKey) Counterparty() ChannelKey {
	return ChannelKey{
		ChannelID:             channelKey.CounterpartyChannelID,
		PortID:                channelKey.CounterpartyPortID,
		CounterpartyChannelID: channelKey.ChannelID,
		CounterpartyPortID:    channelKey.PortID,
	}
}

// ChannelState is used for tracking the most recent state of a channel.
type ChannelState struct {
	// Is the channel currently open, i.e. can packets be relayed on this channel now.
	Open bool
	// Any new IBC messages relevant to the channel so that the PathProcessor
	// can take necessary action such as complete the channel handshake.
	Messages []string
}

// ChannelStateCache maintains channel state for multiple channels.
type ChannelStateCache map[ChannelKey]ChannelState

// Flush is used to make a copy of the ChannelStateCache, helpful for the ChainProcessors
// to clone their local cache so that PathProcessors can have a thread-safe copy.
// It will also empty the messages for the existing channel state.
func (c ChannelStateCache) Flush() ChannelStateCache {
	copy := make(ChannelStateCache)
	for channelKey, existingState := range c {
		copy[channelKey] = existingState
		flushed := existingState
		flushed.Messages = nil
		c[channelKey] = flushed
	}
	return copy
}

// Merge will merge another ChannelStateCache into this one, appending messages and updating the Open state.
func (c ChannelStateCache) Merge(other ChannelStateCache) {
	for channelKey, newState := range other {
		existingState, ok := c[channelKey]
		if !ok {
			c[channelKey] = newState
			return
		}
		existingState.Messages = append(existingState.Messages, newState.Messages...)
		existingState.Open = newState.Open
		c[channelKey] = existingState
	}
}

// ChainProcessorCacheData is the data sent from the ChainProcessors to the PathProcessors
// to keep the PathProcessors up to date with the latest info from the chains.
type ChainProcessorCacheData struct {
	ChannelMessageCache
	InSync bool
	ChannelStateCache
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

// Merge will merge another MessageCache into this one.
func (c ChannelMessageCache) Merge(other ChannelMessageCache) {
	for channelKey, messageCache := range other {
		_, ok := c[channelKey]
		if !ok {
			c[channelKey] = messageCache
		} else {
			c[channelKey].Merge(messageCache)
		}
	}
}

// ShouldRetainSequence returns true if packet is applicable to the channels for path processors that are subscribed to this chain processor
func (c ChannelMessageCache) ShouldRetainSequence(p PathProcessors, k ChannelKey, chainID string, m string, seq uint64) bool {
	if !p.IsRelayedChannel(k, chainID) {
		return false
	}
	if _, ok := c[k]; !ok {
		return true
	}
	if _, ok := c[k][m]; !ok {
		return true
	}
	for sequence := range c[k][m] {
		if sequence == seq {
			// already have this sequence number
			// there can be multiple MsgRecvPacket, MsgAcknowlegement, MsgTimeout, and MsgTimeoutOnClose for the same packet
			// from different relayers.
			return false
		}
	}

	return true
}
