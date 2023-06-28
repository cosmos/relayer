package icon

const (
	MethodRegisterClient = "registerClient"
	MethodCreateClient   = "createClient"
	MethodUpdateClient   = "updateClient"

	MethodConnectionOpenInit    = "connectionOpenInit"
	MethodConnectionOpenTry     = "connectionOpenTry"
	MethodConnectionOpenAck     = "connectionOpenAck"
	MethodConnectionOpenConfirm = "connectionOpenConfirm"

	MethodChannelOpenInit     = "channelOpenInit"
	MethodChannelOpenTry      = "channelOpenTry"
	MethodChannelOpenAck      = "channelOpenAck"
	MethodChannelOpenConfirm  = "channelOpenConfirm"
	MethodChannelCloseInit    = "channelCloseInit"
	MethodChannelCloseConfirm = "channelCloseConfirm"

	MethodRecvPacket = "recvPacket"
	MethodAckPacket  = "acknowledgePacket"
	MethodWriteAck   = "writeAcknowledgement"

	MethodGetPacketCommitment                = "getPacketCommitment"
	MethodGetPacketAcknowledgementCommitment = "getPacketAcknowledgementCommitment"
	MethodHasPacketReceipt                   = "hasPacketReceipt"
	MethodGetPacketReceipt                   = "getPacketReceipt"
	MethodGetNextSequenceReceive             = "getNextSequenceReceive"
	MethodGetNextSequenceSend                = "getNextSequenceSend"
	MethodGetNextSequenceAcknowledgement     = "getNextSequenceAcknowledgement"

	MethodGetChannel              = "getChannel"
	MethodGetConnection           = "getConnection"
	MethodGetClientState          = "getClientState"
	MethodGetClientConsensusState = "getClientConsensusState"
	MethodGetConsensusState       = "getConsensusState"

	MethodGetNextClientSequence     = "getNextClientSequence"
	MethodGetNextChannelSequence    = "getNextChannelSequence"
	MethodGetNextConnectionSequence = "getNextConnectionSequence"

	MethodRequestTimeout = "requestTimeout"
	MethodTimeoutPacket  = "timeoutPacket"

	MethodGetAllPorts = "getAllPorts"
)
