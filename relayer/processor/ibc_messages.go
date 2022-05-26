package processor

// These are IBC message types used as shared message keys between ChainProcessors and PathProcessors.
const (
	MsgTransfer        = "/ibc.applications.transfer.v1.MsgTransfer"
	MsgRecvPacket      = "/ibc.core.channel.v1.MsgRecvPacket"
	MsgAcknowledgement = "/ibc.core.channel.v1.MsgAcknowledgement"
)
