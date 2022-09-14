package substrate

type SubstrateEventType = string

const (
	CreateClient          SubstrateEventType = "CreateClient"
	UpdateClient          SubstrateEventType = "UpdateClient"
	UpgradeClient         SubstrateEventType = "UpgradeClient"
	ClientMisbehaviour    SubstrateEventType = "ClientMisbehaviour"
	SendPacket            SubstrateEventType = "SendPacket"
	ReceivePacket         SubstrateEventType = "ReceivePacket"
	WriteAcknowledgement  SubstrateEventType = "WriteAcknowledgement"
	AcknowledgePacket     SubstrateEventType = "AcknowledgePacket"
	TimeoutPacket         SubstrateEventType = "TimeoutPacket"
	TimeoutOnClosePacket  SubstrateEventType = "TimeoutOnClosePacket"
	OpenInitChannel       SubstrateEventType = "OpenInitChannel"
	OpenTryChannel        SubstrateEventType = "OpenTryChannel"
	OpenAckChannel        SubstrateEventType = "OpenAckChannel"
	OpenConfirmChannel    SubstrateEventType = "OpenConfirmChannel"
	CloseInitChannel      SubstrateEventType = "CloseInitChannel"
	CloseConfirmChannel   SubstrateEventType = "CloseConfirmChannel"
	OpenInitConnection    SubstrateEventType = "OpenInitConnection"
	OpenTryConnection     SubstrateEventType = "OpenTryConnection"
	OpenAckConnection     SubstrateEventType = "OpenAckConnection"
	OpenConfirmConnection SubstrateEventType = "OpenConfirmConnection"
)
