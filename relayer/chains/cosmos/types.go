package cosmos

import (
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	messageType string
	messageInfo interface{}
}

// channelInfo contains useful channel information during channel state changes
type channelInfo struct {
	portID                string
	channelID             string
	counterpartyPortID    string
	counterpartyChannelID string
	connectionID          string
}

// channelKey returns the processor.ChannelKey from channelInfo
func (c channelInfo) channelKey() processor.ChannelKey {
	return processor.ChannelKey{
		ChannelID:             c.channelID,
		PortID:                c.portID,
		CounterpartyChannelID: c.counterpartyChannelID,
		CounterpartyPortID:    c.counterpartyPortID,
	}
}

type connectionInfo struct {
	connectionID             string
	clientID                 string
	counterpartyClientID     string
	counterpartyConnectionID string
}

// packetInfo contains pertinent packet information for constructing IBC messages for the counterparty.
type packetInfo struct {
	// Packet is the IBC conformant Packet.
	packet chantypes.Packet

	// if message is a MsgRecvPacket, this is needed to construct MsgAcknowledgement for counterparty.
	ack []byte

	channelOrdering string
	connectionID    string
}

// channelKey returns the processor.ChannelKey from packetInfo
func (p packetInfo) channelKey() processor.ChannelKey {
	return processor.ChannelKey{
		ChannelID:             p.packet.SourceChannel,
		PortID:                p.packet.SourcePort,
		CounterpartyChannelID: p.packet.DestinationChannel,
		CounterpartyPortID:    p.packet.DestinationPort,
	}
}

// clientInfo contains the consensus height of the counterparty chain for a client.
type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
	header          []byte
}

type latestClientState map[string]clientInfo

func (l latestClientState) UpdateLatestClientState(clientInfo clientInfo) {
	existingClientInfo, ok := l[clientInfo.clientID]
	if ok && clientInfo.consensusHeight.LT(existingClientInfo.consensusHeight) {
		// height is less than latest, so no-op
		return
	}

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientInfo
}

func (l latestClientState) Clone() latestClientState {
	newLatestClientState := make(latestClientState)
	for k, v := range l {
		newLatestClientState[k] = v
	}
	return newLatestClientState
}

type channelOpenState map[processor.ChannelKey]bool

func (c channelOpenState) Clone() channelOpenState {
	newChannelOpenState := make(channelOpenState)
	for k, v := range c {
		newChannelOpenState[k] = v
	}
	return newChannelOpenState
}
