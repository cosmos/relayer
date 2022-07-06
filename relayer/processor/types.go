package processor

import (
	"fmt"
	"sort"
	"strings"

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

// ShortAction returns the short name for an IBC message action.
type ShortAction string

func (a ShortAction) String() string {
	split := strings.Split(string(a), ".")
	return split[len(split)-1]
}

type IBCMessagesCache struct {
	PacketFlow          ChannelPacketMessagesCache
	ConnectionHandshake ConnectionMessagesCache
	ChannelHandshake    ChannelMessagesCache
}

func NewIBCMessagesCache() IBCMessagesCache {
	return IBCMessagesCache{
		PacketFlow:          make(ChannelPacketMessagesCache),
		ConnectionHandshake: make(ConnectionMessagesCache),
		ChannelHandshake:    make(ChannelMessagesCache),
	}
}

// ChannelPacketMessagesCache is used for caching a PacketMessagesCache for a given IBC channel.
type ChannelPacketMessagesCache map[ChannelKey]PacketMessagesCache

// PacketMessagesCache is used for caching a PacketSequenceCache for a given IBC message type.
type PacketMessagesCache map[string]PacketSequenceCache

// PacketSequenceCache is used for caching an IBC message for a given packet sequence.
type PacketSequenceCache map[uint64]provider.PacketInfo

// ChannelMessagesCache is used for caching a ChannelMessageCache for a given IBC message type.
type ChannelMessagesCache map[string]ChannelMessageCache

// ChannelMessageCache is used for caching channel handshake IBC messages for a given IBC channel.
type ChannelMessageCache map[ChannelKey]provider.ChannelInfo

// ConnectionMessagesCache is used for caching a ConnectionMessageCache for a given IBC message type.
type ConnectionMessagesCache map[string]ConnectionMessageCache

// ConnectionMessageCache is used for caching connection handshake IBC messages for a given IBC connection.
type ConnectionMessageCache map[ConnectionKey]provider.ConnectionInfo

// ChannelKey is the key used for identifying channels between ChainProcessor and PathProcessor.
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

// msgInitKey is used for comparing MsgChannelOpenInit keys with other connection
// handshake messages. MsgChannelOpenInit does not have CounterpartyChannelID.
func (channelKey ChannelKey) msgInitKey() ChannelKey {
	return ChannelKey{
		ChannelID:             channelKey.ChannelID,
		PortID:                channelKey.PortID,
		CounterpartyChannelID: "",
		CounterpartyPortID:    channelKey.CounterpartyPortID,
	}
}

// ConnectionKey is the key used for identifying connections between ChainProcessor and PathProcessor.
type ConnectionKey struct {
	ClientID             string
	ConnectionID         string
	CounterpartyClientID string
	CounterpartyConnID   string
}

// Counterparty flips a ConnectionKey for the perspective of the counterparty chain
func (connectionKey ConnectionKey) Counterparty() ConnectionKey {
	return ConnectionKey{
		ClientID:             connectionKey.CounterpartyClientID,
		ConnectionID:         connectionKey.CounterpartyConnID,
		CounterpartyClientID: connectionKey.ClientID,
		CounterpartyConnID:   connectionKey.ConnectionID,
	}
}

// msgInitKey is used for comparing MsgConnectionOpenInit keys with other connection
// handshake messages. MsgConnectionOpenInit does not have CounterpartyConnectionID.
func (connectionKey ConnectionKey) msgInitKey() ConnectionKey {
	return ConnectionKey{
		ClientID:             connectionKey.ClientID,
		ConnectionID:         connectionKey.ConnectionID,
		CounterpartyClientID: connectionKey.CounterpartyClientID,
		CounterpartyConnID:   "",
	}
}

// ChannelStateCache maintains channel open state for multiple channels.
type ChannelStateCache map[ChannelKey]bool

// Merge merges another ChannelStateCache into this one, appending messages and updating the Open state.
func (c ChannelStateCache) Merge(other ChannelStateCache) {
	for channelKey, newState := range other {
		c[channelKey] = newState
	}
}

// Clone makes a copy of the ChannelStateCache so it can be used by other threads.
func (c ChannelStateCache) Clone() ChannelStateCache {
	n := make(ChannelStateCache, len(c))
	for k, v := range c {
		n[k] = v
	}
	return n
}

// ConnectionStateCache maintains connection open state for multiple connections.
type ConnectionStateCache map[ConnectionKey]bool

// Merge merges another ChannelStateCache into this one, appending messages and updating the Open state.
func (c ConnectionStateCache) Merge(other ConnectionStateCache) {
	for channelKey, newState := range other {
		c[channelKey] = newState
	}
}

// Clone makes a copy of the ConnectionStateCache so it can be used by other threads.
func (c ConnectionStateCache) Clone() ConnectionStateCache {
	n := make(ConnectionStateCache, len(c))
	for k, v := range c {
		n[k] = v
	}
	return n
}

// ChainProcessorCacheData is the data sent from the ChainProcessors to the PathProcessors
// to keep the PathProcessors up to date with the latest info from the chains.
type ChainProcessorCacheData struct {
	IBCMessagesCache     IBCMessagesCache
	InSync               bool
	ClientState          provider.ClientState
	ConnectionStateCache ConnectionStateCache
	ChannelStateCache    ChannelStateCache
	LatestBlock          provider.LatestBlock
	LatestHeader         provider.IBCHeader
	IBCHeaderCache       IBCHeaderCache
}

// Clone creates a deep copy of a PacketMessagesCache.
func (c PacketMessagesCache) Clone() PacketMessagesCache {
	newPacketMessagesCache := make(PacketMessagesCache, len(c))
	for mk, mv := range c {
		newPacketSequenceCache := make(PacketSequenceCache, len(mv))
		for sk, sv := range mv {
			newPacketSequenceCache[sk] = sv
		}
		newPacketMessagesCache[mk] = newPacketSequenceCache
	}
	return newPacketMessagesCache
}

func (c PacketMessagesCache) DeleteMessages(toDelete ...map[string][]uint64) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, sequence := range toDeleteMessages {
				delete(c[message], sequence)
			}
		}
	}
}

// Merge merges another ChannelPacketMessagesCache into this one.
func (c ChannelPacketMessagesCache) Merge(other ChannelPacketMessagesCache) {
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
func (c ChannelPacketMessagesCache) ShouldRetainSequence(p PathProcessors, k ChannelKey, chainID string, m string, seq uint64) bool {
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
			// there can be multiple MsgRecvPacket, MsgAcknowledgement, MsgTimeout, and MsgTimeoutOnClose for the same packet
			// from different relayers.
			return false
		}
	}

	return true
}

// Retain assumes the packet is applicable to the channels for a path processor that is subscribed to this chain processor.
// It creates cache path if it doesn't exist, then caches message.
func (c ChannelPacketMessagesCache) Retain(k ChannelKey, m string, pi provider.PacketInfo) {
	if _, ok := c[k]; !ok {
		c[k] = make(PacketMessagesCache)
	}
	if _, ok := c[k][m]; !ok {
		c[k][m] = make(PacketSequenceCache)
	}
	c[k][m][pi.Sequence] = pi
}

// Merge merges another PacketMessagesCache into this one.
func (c PacketMessagesCache) Merge(other PacketMessagesCache) {
	for ibcMessage, messageCache := range other {
		_, ok := c[ibcMessage]
		if !ok {
			c[ibcMessage] = messageCache
		} else {
			c[ibcMessage].Merge(messageCache)
		}
	}
}

// Merge merges another PacketSequenceCache into this one.
func (c PacketSequenceCache) Merge(other PacketSequenceCache) {
	for k, v := range other {
		c[k] = v
	}
}

// Merge merges another ConnectionMessagesCache into this one.
func (c ConnectionMessagesCache) Merge(other ConnectionMessagesCache) {
	for ibcMessage, messageCache := range other {
		_, ok := c[ibcMessage]
		if !ok {
			c[ibcMessage] = messageCache
		} else {
			c[ibcMessage].Merge(messageCache)
		}
	}
}

// Retain assumes creates cache path if it doesn't exist, then caches message.
func (c ConnectionMessagesCache) Retain(k ConnectionKey, m string, ibcMsg provider.ConnectionInfo) {
	if _, ok := c[m]; !ok {
		c[m] = make(ConnectionMessageCache)
	}
	c[m][k] = ibcMsg
}

func (c ConnectionMessagesCache) DeleteMessages(toDelete ...map[string][]ConnectionKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, connection := range toDeleteMessages {
				delete(c[message], connection)
			}
		}
	}
}

// Merge merges another ConnectionMessageCache into this one.
func (c ConnectionMessageCache) Merge(other ConnectionMessageCache) {
	for k, v := range other {
		c[k] = v
	}
}

// Merge merges another ChannelMessagesCache into this one.
func (c ChannelMessagesCache) Merge(other ChannelMessagesCache) {
	for ibcMessage, messageCache := range other {
		_, ok := c[ibcMessage]
		if !ok {
			c[ibcMessage] = messageCache
		} else {
			c[ibcMessage].Merge(messageCache)
		}
	}
}

// Retain assumes creates cache path if it doesn't exist, then caches message.
func (c ChannelMessagesCache) Retain(k ChannelKey, m string, ibcMsg provider.ChannelInfo) {
	if _, ok := c[m]; !ok {
		c[m] = make(ChannelMessageCache)
	}
	c[m][k] = ibcMsg
}

func (c ChannelMessagesCache) DeleteMessages(toDelete ...map[string][]ChannelKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, channel := range toDeleteMessages {
				delete(c[message], channel)
			}
		}
	}
}

// Merge merges another ChannelMessageCache into this one.
func (c ChannelMessageCache) Merge(other ChannelMessageCache) {
	for k, v := range other {
		c[k] = v
	}
}

// IBCHeaderCache holds a mapping of IBCHeaders for their block height.
type IBCHeaderCache map[uint64]provider.IBCHeader

// Merge merges another IBCHeaderCache into this one.
func (c IBCHeaderCache) Merge(other IBCHeaderCache) {
	for k, v := range other {
		c[k] = v
	}
}

// Prune deletes all map entries except for the most recent (keep).
func (c IBCHeaderCache) Prune(keep int) {
	keys := make([]uint64, len(c))
	i := 0
	for k := range c {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	toRemove := len(keys) - keep - 1
	if toRemove > 0 {
		for i := 0; i <= toRemove; i++ {
			delete(c, keys[i])
		}
	}
}

// PacketInfoChannelKey returns the applicable ChannelKey for the chain based on the action.
func PacketInfoChannelKey(action string, info provider.PacketInfo) (ChannelKey, error) {
	switch action {
	case MsgRecvPacket:
		return packetInfoChannelKey(info).Counterparty(), nil
	case MsgTransfer, MsgAcknowledgement, MsgTimeout, MsgTimeoutOnClose:
		return packetInfoChannelKey(info), nil
	}
	return ChannelKey{}, fmt.Errorf("action not expected for packetIBCMessage channelKey: %s", action)
}

// ChannelInfoChannelKey returns the applicable ChannelKey for ChannelInfo.
func ChannelInfoChannelKey(info provider.ChannelInfo) ChannelKey {
	return ChannelKey{
		ChannelID:             info.ChannelID,
		CounterpartyChannelID: info.CounterpartyChannelID,
		PortID:                info.PortID,
		CounterpartyPortID:    info.CounterpartyPortID,
	}
}

// ConnectionInfoConnectionKey returns the applicable ConnectionKey for ConnectionInfo.
func ConnectionInfoConnectionKey(info provider.ConnectionInfo) ConnectionKey {
	return ConnectionKey{
		ClientID:             info.ClientID,
		CounterpartyClientID: info.CounterpartyClientID,
		ConnectionID:         info.ConnID,
		CounterpartyConnID:   info.CounterpartyConnID,
	}
}
