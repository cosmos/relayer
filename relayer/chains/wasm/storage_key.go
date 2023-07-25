package wasm

const WASM_CONTRACT_PREFIX = "03"

const (
	STORAGEKEY__ClientRegistry              = "client_registry"
	STORAGEKEY__ClientTypes                 = "client_types"
	STORAGEKEY__ClientImplementations       = "client_implementations"
	STORAGEKEY__NextSequenceSend            = "next_sequence_send"
	STORAGEKEY__NextSequenceReceive         = "next_sequence_recv"
	STORAGEKEY__NextSequenceAcknowledgement = "next_sequence_ack"
	STORAGEKEY__NextClientSequence          = "next_client_sequence"
	STORAGEKEY__NextConnectionSequence      = "next_connection_sequence"
	STORAGEKEY__NextChannelSequence         = "next_channel_sequence"
	STORAGEKEY__Connections                 = "connections"
	STORAGEKEY__ClientConnection            = "client_connections"
	STORAGEKEY__Channels                    = "channels"
	STORAGEKEY__Router                      = "router"
	STORAGEKEY__PortToModule                = "port_to_module"
	STORAGEKEY__Commitments                 = "commitments"
	STORAGEKEY__BlockTime                   = "block_time"
	STORAGEKEY__BlockHeight                 = "block_height"
	STORAGEKEY__Capabilities                = "capabilities"
	STORAGEKEY__PacketReceipts              = "packet_receipts"
	STORAGEKEY__Owner                       = "owner"
)
