package wasm

const (
	// External methods
	MethodCreateClient          = "create_client"
	MethodUpdateClient          = "update_client"
	MethodConnectionOpenInit    = "connection_open_init"
	MethodConnectionOpenTry     = "connection_open_try"
	MethodConnectionOpenAck     = "connection_open_ack"
	MethodConnectionOpenConfirm = "connection_open_confirm"
	MethodChannelOpenInit       = "channel_open_init"
	MethodChannelOpenTry        = "channel_open_try"
	MethodChannelOpenAck        = "channel_open_ack"
	MethodChannelOpenConfirm    = "channel_open_confirm"
	MethodChannelCloseInit      = "channel_close_init"
	MethodChannelCloseConfirm   = "channel_close_confirm"
	MethodSendPacket            = "send_packet"
	MethodRecvPacket            = "receive_packet"
	MethodWriteAcknowledgement  = "write_acknowledgement"
	MethodAcknowledgePacket     = "acknowledgement_packet"
	MethodTimeoutPacket         = "timeout_packet"

	MethodGetNextClientSequence     = "get_next_client_sequence"
	MethodGetNextChannelSequence    = "get_next_channel_sequence"
	MethodGetNextConnectionSequence = "get_next_connection_sequence"
)

const (
	ClientPrefix     = "iconclient"
	ConnectionPrefix = "connection"
	ChannelPrefix    = "channel"
)

const (
	ContractAddressSizeMinusPrefix = 59
)
