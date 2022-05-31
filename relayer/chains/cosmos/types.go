package cosmos

import (
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
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
}

// clientInfo contains the consensus height of the counterparty chain for a client.
type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
}
