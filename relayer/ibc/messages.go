package ibc

const (
	MsgTransfer        = "/ibc.applications.transfer.v1.MsgTransfer"
	MsgRecvPacket      = "/ibc.core.channel.v1.MsgRecvPacket"
	MsgAcknowledgement = "/ibc.core.channel.v1.MsgAcknowledgement"
	MsgTimeout         = "/ibc.core.channel.v1.MsgTimeout"
	MsgTimeoutOnClose  = "/ibc.core.channel.v1.MsgTimeoutOnClose"

	MsgChannelCloseConfirm = "/ibc.core.channel.v1.MsgChannelCloseConfirm"
	MsgChannelCloseInit    = "/ibc.core.channel.v1.MsgChannelCloseInit"
	MsgChannelOpenAck      = "/ibc.core.channel.v1.MsgChannelOpenAck"
	MsgChannelOpenConfirm  = "/ibc.core.channel.v1.MsgChannelOpenConfirm"
	MsgChannelOpenInit     = "/ibc.core.channel.v1.MsgChannelOpenInit"
	MsgChannelOpenTry      = "/ibc.core.channel.v1.MsgChannelOpenTry"

	MsgCreateClient       = "/ibc.core.client.v1.MsgCreateClient"
	MsgUpdateClient       = "/ibc.core.client.v1.MsgUpdateClient"
	MsgUpgradeClient      = "/ibc.core.client.v1.MsgUpgradeClient"
	MsgSubmitMisbehaviour = "/ibc.core.client.v1.MsgSubmitMisbehaviour"
)
