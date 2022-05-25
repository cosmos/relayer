package ibc

// ChannelKey is the key used between ChainProcessor and PathProcessor
type ChannelKey struct {
	ChannelID             string
	PortID                string
	CounterpartyChannelID string
	CounterpartyPortID    string
}

// These are IBC message types used as shared message keys between ChainProcessor and PathProcessor
const (
	MsgTransfer        = "/ibc.applications.transfer.v1.MsgTransfer"
	MsgRecvPacket      = "/ibc.core.channel.v1.MsgRecvPacket"
	MsgAcknowledgement = "/ibc.core.channel.v1.MsgAcknowledgement"
)
