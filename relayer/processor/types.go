package processor

import (
	"fmt"
	"sort"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap/zapcore"
)

// MessageLifecycle is used to send an initial IBC message to a chain
// once the chains are in sync for the PathProcessor.
// It also allows setting a stop condition for the PathProcessor.
// PathProcessor will stop if it observes a message that matches
// the MessageLifecycle's Termination message.
type MessageLifecycle interface {
	messageLifecycler() //noop
}

// Flush lifecycle informs the PathProcessor to terminate once
// all pending messages have been flushed.
type FlushLifecycle struct{}

func (t *FlushLifecycle) messageLifecycler() {}

type PacketMessage struct {
	ChainID   string
	EventType string
	Info      provider.PacketInfo
}

// PacketMessageLifecycle is used as a stop condition for the PathProcessor.
// It will send the Initial packet message (if non-nil), then stop once it
// observes the Termination packet message (if non-nil).
type PacketMessageLifecycle struct {
	Initial     *PacketMessage
	Termination *PacketMessage
}

func (t *PacketMessageLifecycle) messageLifecycler() {}

type ConnectionMessage struct {
	ChainID   string
	EventType string
	Info      provider.ConnectionInfo
}

// ConnectionMessageLifecycle is used as a stop condition for the PathProcessor.
// It will send the Initial connection message (if non-nil), then stop once it
// observes the termination connection message (if non-nil).
type ConnectionMessageLifecycle struct {
	Initial     *ConnectionMessage
	Termination *ConnectionMessage
}

func (t *ConnectionMessageLifecycle) messageLifecycler() {}

type ChannelMessage struct {
	ChainID   string
	EventType string
	Info      provider.ChannelInfo
}

// ChannelMessageLifecycle is used as a stop condition for the PathProcessor.
// It will send the Initial channel message (if non-nil), then stop once it observes
// the termination channel message (if non-nil).
type ChannelMessageLifecycle struct {
	Initial     *ChannelMessage
	Termination *ChannelMessage
}

func (t *ChannelMessageLifecycle) messageLifecycler() {}

// ChannelCloseLifecycle is used as a stop condition for the PathProcessor.
// It will attempt to finish closing the channel and terminate once the channel is closed.
type ChannelCloseLifecycle struct {
	SrcChainID   string
	SrcChannelID string
	SrcPortID    string
	SrcConnID    string
	DstConnID    string
}

func (t *ChannelCloseLifecycle) messageLifecycler() {}

// IBCMessagesCache holds cached messages for packet flows, connection handshakes,
// and channel handshakes. The PathProcessors use this for message correlation to determine
// when messages should be sent and are pruned when flows/handshakes are complete.
// ChainProcessors construct this for new IBC messages and pass it to the PathProcessors
// which will retain relevant messages for each PathProcessor.
type IBCMessagesCache struct {
	PacketFlow          ChannelPacketMessagesCache
	ConnectionHandshake ConnectionMessagesCache
	ChannelHandshake    ChannelMessagesCache
	ClientICQ           ClientICQMessagesCache
}

// Clone makes a deep copy of an IBCMessagesCache.
func (c IBCMessagesCache) Clone() IBCMessagesCache {
	x := IBCMessagesCache{
		PacketFlow:          make(ChannelPacketMessagesCache, len(c.PacketFlow)),
		ConnectionHandshake: make(ConnectionMessagesCache, len(c.ConnectionHandshake)),
		ChannelHandshake:    make(ChannelMessagesCache, len(c.ChannelHandshake)),
		ClientICQ:           make(ClientICQMessagesCache, len(c.ClientICQ)),
	}
	x.PacketFlow.Merge(c.PacketFlow)
	x.ConnectionHandshake.Merge(c.ConnectionHandshake)
	x.ChannelHandshake.Merge(c.ChannelHandshake)
	x.ClientICQ.Merge(c.ClientICQ)
	return x
}

// NewIBCMessagesCache returns an empty IBCMessagesCache.
func NewIBCMessagesCache() IBCMessagesCache {
	return IBCMessagesCache{
		PacketFlow:          make(ChannelPacketMessagesCache),
		ConnectionHandshake: make(ConnectionMessagesCache),
		ChannelHandshake:    make(ChannelMessagesCache),
		ClientICQ:           make(ClientICQMessagesCache),
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

// ClientICQType string wrapper for query/response type.
type ClientICQType string

// ClientICQMessagesCache is used for caching a ClientICQMessageCache for a given type (query/response).
type ClientICQMessagesCache map[ClientICQType]ClientICQMessageCache

// ClientICQMessageCache is used for caching a client ICQ message for a given query ID.
type ClientICQMessageCache map[provider.ClientICQQueryID]provider.ClientICQInfo

// ChannelKey is the key used for identifying channels between ChainProcessor and PathProcessor.
type ChannelKey struct {
	ChannelID             string
	PortID                string
	CounterpartyChannelID string
	CounterpartyPortID    string
}

// ChannelState is used for caching channel open state and a lookup for the channel order.
type ChannelState struct {
	Order chantypes.Order
	Open  bool
}

// Counterparty flips a ChannelKey for the perspective of the counterparty chain
func (k ChannelKey) Counterparty() ChannelKey {
	return ChannelKey{
		ChannelID:             k.CounterpartyChannelID,
		PortID:                k.CounterpartyPortID,
		CounterpartyChannelID: k.ChannelID,
		CounterpartyPortID:    k.PortID,
	}
}

// MsgInitKey is used for comparing MsgChannelOpenInit keys with other connection
// handshake messages. MsgChannelOpenInit does not have CounterpartyChannelID.
func (k ChannelKey) MsgInitKey() ChannelKey {
	return ChannelKey{
		ChannelID:             k.ChannelID,
		PortID:                k.PortID,
		CounterpartyChannelID: "",
		CounterpartyPortID:    k.CounterpartyPortID,
	}
}

// PreInitKey is used for comparing pre-init keys with other connection
// handshake messages. Before the channel handshake,
// do not have ChannelID or CounterpartyChannelID.
func (k ChannelKey) PreInitKey() ChannelKey {
	return ChannelKey{
		ChannelID:             "",
		PortID:                k.PortID,
		CounterpartyChannelID: "",
		CounterpartyPortID:    k.CounterpartyPortID,
	}
}

func (k ChannelKey) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", k.ChannelID)
	enc.AddString("port_id", k.PortID)
	enc.AddString("counterparty_channel_id", k.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", k.CounterpartyPortID)
	return nil
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

// MsgInitKey is used for comparing MsgConnectionOpenInit keys with other connection
// handshake messages. MsgConnectionOpenInit does not have CounterpartyConnectionID.
func (connectionKey ConnectionKey) MsgInitKey() ConnectionKey {
	return ConnectionKey{
		ClientID:             connectionKey.ClientID,
		ConnectionID:         connectionKey.ConnectionID,
		CounterpartyClientID: connectionKey.CounterpartyClientID,
		CounterpartyConnID:   "",
	}
}

// PreInitKey is used for comparing  pre-init keys with other connection
// handshake messages. Before starting a connection handshake,
// do not have ConnectionID or CounterpartyConnectionID.
func (connectionKey ConnectionKey) PreInitKey() ConnectionKey {
	return ConnectionKey{
		ClientID:             connectionKey.ClientID,
		ConnectionID:         "",
		CounterpartyClientID: connectionKey.CounterpartyClientID,
		CounterpartyConnID:   "",
	}
}

func (k ConnectionKey) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", k.ConnectionID)
	enc.AddString("client_id", k.ClientID)
	enc.AddString("counterparty_connection_id", k.CounterpartyConnID)
	enc.AddString("counterparty_client_id", k.CounterpartyClientID)
	return nil
}

// ChannelStateCache maintains channel open state for multiple channels.
type ChannelStateCache map[ChannelKey]ChannelState

// SetOpen sets the open state for a channel, and also the order if it is not NONE.
func (c ChannelStateCache) SetOpen(k ChannelKey, open bool, order chantypes.Order) {
	if s, ok := c[k]; ok {
		s.Open = open
		if order != chantypes.NONE {
			s.Order = order
		}
		c[k] = s
		return
	}
	c[k] = ChannelState{
		Open:  open,
		Order: order,
	}
}

// FilterForClient returns a filtered copy of channels on top of an underlying clientID so it can be used by other goroutines.
func (c ChannelStateCache) FilterForClient(clientID string, channelConnections map[string]string, connectionClients map[string]string) ChannelStateCache {
	n := make(ChannelStateCache)
	for k, v := range c {
		connection, ok := channelConnections[k.ChannelID]
		if !ok {
			continue
		}
		client, ok := connectionClients[connection]
		if !ok {
			continue
		}
		if clientID == client {
			n[k] = v
		}
	}
	return n
}

// ConnectionStateCache maintains connection open state for multiple connections.
type ConnectionStateCache map[ConnectionKey]bool

// FilterForClient makes a filtered copy of the ConnectionStateCache
// for a single client ID so it can be used by other goroutines.
func (c ConnectionStateCache) FilterForClient(clientID string) ConnectionStateCache {
	n := make(ConnectionStateCache)
	for k, v := range c {
		if k.ClientID == clientID {
			n[k] = v
		}
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

// IsCached returns true if a sequence for a channel key and event type is already cached.
func (c ChannelPacketMessagesCache) IsCached(eventType string, k ChannelKey, sequence uint64) bool {
	if _, ok := c[k]; !ok {
		return false
	}
	if _, ok := c[k][eventType]; !ok {
		return false
	}
	if _, ok := c[k][eventType][sequence]; !ok {
		return false
	}
	return true
}

// Cache stores packet info safely, generating intermediate maps along the way if necessary.
func (c ChannelPacketMessagesCache) Cache(
	eventType string,
	k ChannelKey,
	sequence uint64,
	packetInfo provider.PacketInfo,
) {
	if _, ok := c[k]; !ok {
		c[k] = make(PacketMessagesCache)
	}
	if _, ok := c[k][eventType]; !ok {
		c[k][eventType] = make(PacketSequenceCache)
	}
	c[k][eventType][sequence] = packetInfo
}

// Merge merges another ChannelPacketMessagesCache into this one.
func (c ChannelPacketMessagesCache) Merge(other ChannelPacketMessagesCache) {
	for channelKey, messageCache := range other {
		if _, ok := c[channelKey]; !ok {
			c[channelKey] = make(PacketMessagesCache)
		}
		c[channelKey].Merge(messageCache)
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
		if _, ok := c[ibcMessage]; !ok {
			c[ibcMessage] = make(PacketSequenceCache)
		}
		c[ibcMessage].Merge(messageCache)
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
		if _, ok := c[ibcMessage]; !ok {
			c[ibcMessage] = make(ConnectionMessageCache)
		}
		c[ibcMessage].Merge(messageCache)
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
		if _, ok := c[ibcMessage]; !ok {
			c[ibcMessage] = make(ChannelMessageCache)
		}
		c[ibcMessage].Merge(messageCache)
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

// Retain creates cache path if it doesn't exist, then caches message.
func (c ClientICQMessagesCache) Retain(icqType ClientICQType, ci provider.ClientICQInfo) {
	queryID := ci.QueryID
	if _, ok := c[icqType]; !ok {
		c[icqType] = make(ClientICQMessageCache)
	}
	c[icqType][queryID] = ci
}

// Merge merges another ClientICQMessagesCache into this one.
func (c ClientICQMessagesCache) Merge(other ClientICQMessagesCache) {
	for k, v := range other {
		_, ok := c[k]
		if !ok {
			c[k] = v
		} else {
			c[k].Merge(v)
		}
	}
}

// Merge merges another ClientICQMessageCache into this one.
func (c ClientICQMessageCache) Merge(other ClientICQMessageCache) {
	for k, v := range other {
		c[k] = v
	}
}

// DeleteMessages deletes cached messages for the provided query ID.
func (c ClientICQMessagesCache) DeleteMessages(queryID provider.ClientICQQueryID) {
	for _, cm := range c {
		delete(cm, queryID)
	}
}

// IBCHeaderCache holds a mapping of IBCHeaders for their block height.
type IBCHeaderCache map[uint64]provider.IBCHeader

// Clone makes a deep copy of an IBCHeaderCache.
func (c IBCHeaderCache) Clone() IBCHeaderCache {
	x := make(IBCHeaderCache, len(c))
	x.Merge(c)
	return x
}

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

// PacketInfoChannelKey returns the applicable ChannelKey for the chain based on the eventType.
func PacketInfoChannelKey(eventType string, info provider.PacketInfo) (ChannelKey, error) {
	switch eventType {
	case chantypes.EventTypeRecvPacket, chantypes.EventTypeWriteAck:
		return packetInfoChannelKey(info).Counterparty(), nil
	case chantypes.EventTypeSendPacket, chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket, chantypes.EventTypeTimeoutPacketOnClose:
		return packetInfoChannelKey(info), nil
	}
	return ChannelKey{}, fmt.Errorf("eventType not expected for packetIBCMessage channelKey: %s", eventType)
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

// StuckPacket is used for narrowing block queries on packets that are stuck on a channel for a specific chain.
type StuckPacket struct {
	ChainID     string
	StartHeight uint64
	EndHeight   uint64
}
