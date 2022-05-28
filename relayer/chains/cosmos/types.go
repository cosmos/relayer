package cosmos

import (
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

// TransactionMessage is the type used for parsing IBC messages
type TransactionMessage struct {
	Action         string
	PacketInfo     *PacketInfo
	ChannelInfo    *ChannelInfo
	ClientInfo     *ClientInfo
	ConnectionInfo *ConnectionInfo
}

// ChannelInfo contains useful channel information during channel state changes
type ChannelInfo struct {
	PortID                string
	ChannelID             string
	CounterpartyPortID    string
	CounterpartyChannelID string
}

type ConnectionInfo struct {
	ConnectionID             string
	ClientID                 string
	CounterpartyClientID     string
	CounterpartyConnectionID string
}

// PacketInfo contains pertinent packet information for constructing IBC messages for the counterparty.
type PacketInfo struct {
	// Packet is the IBC conformant Packet.
	Packet chantypes.Packet

	// if message is a MsgRecvPacket, this is needed to construct MsgAcknowledgement for counterparty.
	Ack []byte
}

// ClientInfo contains the consensus height of the counterparty chain for a client.
type ClientInfo struct {
	ClientID        string
	ConsensusHeight clienttypes.Height
}
