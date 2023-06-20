package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type HexBytes string

func (hs HexBytes) Value() ([]byte, error) {
	if hs == "" {
		return nil, nil
	}
	return hex.DecodeString(string(hs[2:]))
}

func NewHexBytes(b []byte) HexBytes {
	return HexBytes("0x" + hex.EncodeToString(b))
}

// / IBC Handler Contract Methods and Parameters

// / EXTERNAL METHODS

type ContractCall struct {
	Msg HexBytes `json:"msg"`
}

type CreateClientMsg struct {
	CreateClient ContractCall `json:"create_client"`
}

func (c *CreateClientMsg) Bytes() ([]byte, error) {
	return json.Marshal(c)
}

func GenerateTxnParams(methodName string, value HexBytes) ([]byte, error) {
	if len(methodName) <= 0 {
		return nil, fmt.Errorf("Empty Method Name")
	}
	if len(value) <= 0 {
		return nil, fmt.Errorf("Empty value for %s", methodName)
	}
	m := map[string]interface{}{
		methodName: map[string]HexBytes{
			"msg": value,
		},
	}
	return json.Marshal(m)
}

// / READONLY METHODS
type GetClientState struct {
	ClientState struct {
		ClientId string `json:"client_id"`
	} `json:"get_client_state"`
}

func (x *GetClientState) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewClientState(clientId string) *GetClientState {
	return &GetClientState{
		struct {
			ClientId string `json:"client_id"`
		}{
			ClientId: clientId,
		},
	}
}

type GetConsensusState struct {
	ConsensusState struct {
		ClientId string "json:\"client_id\""
		Height   uint64 "json:\"height\""
	} `json:"get_consensus_state"`
}

func (x *GetConsensusState) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewConsensusState(clientId string, height uint64) *GetConsensusState {
	return &GetConsensusState{
		ConsensusState: struct {
			ClientId string "json:\"client_id\""
			Height   uint64 "json:\"height\""
		}{
			ClientId: clientId,
			Height:   height,
		},
	}
}

type GetConnection struct {
	Connection struct {
		ConnectionId string `json:"connection_id"`
	} `json:"get_connection"`
}

func (x *GetConnection) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewConnection(connId string) *GetConnection {
	return &GetConnection{
		Connection: struct {
			ConnectionId string "json:\"connection_id\""
		}{
			ConnectionId: connId,
		},
	}
}

type GetChannel struct {
	Channel struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
	} `json:"get_channel"`
}

func (x *GetChannel) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewChannel(portId, channelId string) *GetChannel {
	return &GetChannel{
		Channel: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
		}{
			PortId:    portId,
			ChannelId: channelId,
		},
	}
}

type GetPacketCommitment struct {
	PacketCommitment struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
		Sequence  uint64 `json:"sequence"`
	} `json:"get_packet_commitment"`
}

func (x *GetPacketCommitment) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewPacketCommitment(portId, channelId string, sequence uint64) *GetPacketCommitment {
	return &GetPacketCommitment{
		PacketCommitment: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
			Sequence  uint64 "json:\"sequence\""
		}{
			PortId:    portId,
			ChannelId: channelId,
			Sequence:  sequence,
		},
	}
}

type GetPacketAcknowledgementCommitment struct {
	PacketCommitment struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
		Sequence  uint64 `json:"sequence"`
	} `json:"get_packet_acknowledgement_commitment"`
}

func (x *GetPacketAcknowledgementCommitment) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewPacketAcknowledgementCommitment(portId, channelId string, sequence uint64) *GetPacketAcknowledgementCommitment {
	return &GetPacketAcknowledgementCommitment{
		PacketCommitment: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
			Sequence  uint64 "json:\"sequence\""
		}{
			PortId:    portId,
			ChannelId: channelId,
			Sequence:  sequence,
		},
	}
}

type GetNextSequenceSend struct {
	NextSequenceSend struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
	} `json:"get_next_sequence_send"`
}

func (x *GetNextSequenceSend) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextSequenceSend(portId, channelId string) *GetNextSequenceSend {
	return &GetNextSequenceSend{
		NextSequenceSend: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
		}{
			PortId:    portId,
			ChannelId: channelId,
		},
	}
}

type GetNextSequenceReceive struct {
	NextSequenceReceive struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
	} `json:"get_next_sequence_receive"`
}

func (x *GetNextSequenceReceive) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextSequenceReceive(portId, channelId string) *GetNextSequenceReceive {
	return &GetNextSequenceReceive{
		NextSequenceReceive: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
		}{
			PortId:    portId,
			ChannelId: channelId,
		},
	}
}

type GetNextSequenceAcknowledgement struct {
	NextSequenceAck struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
	} `json:"get_next_sequence_acknowledgement"`
}

func (x *GetNextSequenceAcknowledgement) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextSequenceAcknowledgement(portId, channelId string) *GetNextSequenceAcknowledgement {
	return &GetNextSequenceAcknowledgement{
		NextSequenceAck: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
		}{
			PortId:    portId,
			ChannelId: channelId,
		},
	}
}

type GetPacketReceipt struct {
	PacketReceipt struct {
		PortId    string `json:"port_id"`
		ChannelId string `json:"channel_id"`
		Sequence  uint64 `json:"sequence"`
	} `json:"get_packet_receipt"`
}

func (x *GetPacketReceipt) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewPacketReceipt(portId, channelId string, sequence uint64) *GetPacketReceipt {
	return &GetPacketReceipt{
		PacketReceipt: struct {
			PortId    string "json:\"port_id\""
			ChannelId string "json:\"channel_id\""
			Sequence  uint64 "json:\"sequence\""
		}{
			PortId:    portId,
			ChannelId: channelId,
			Sequence:  sequence,
		},
	}
}

type GetNextClientSequence struct {
	Sequence struct{} `json:"get_next_client_sequence"`
}

func (x *GetNextClientSequence) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextClientSequence() *GetNextClientSequence {
	return &GetNextClientSequence{
		Sequence: struct{}{},
	}
}

type GetNextConnectionSequence struct {
	Sequence struct{} `json:"get_next_connection_sequence"`
}

func (x *GetNextConnectionSequence) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextConnectionSequence() *GetNextConnectionSequence {
	return &GetNextConnectionSequence{
		Sequence: struct{}{},
	}
}

type GetNextChannelSequence struct {
	Sequence struct{} `json:"get_next_channel_sequence"`
}

func (x *GetNextChannelSequence) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextChannelSequence() *GetNextChannelSequence {
	return &GetNextChannelSequence{
		Sequence: struct{}{},
	}
}

type GetAllPorts struct {
	AllPorts struct{} `json:"get_all_ports"`
}

func (x *GetAllPorts) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewGetAllPorts() *GetAllPorts {
	return &GetAllPorts{
		AllPorts: struct{}{},
	}
}

type GetCommitmentPrefix struct {
	GetCommitment struct{} `json:"get_commitment_prefix"`
}

func (x *GetCommitmentPrefix) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewCommitmentPrefix() *GetCommitmentPrefix {
	return &GetCommitmentPrefix{
		GetCommitment: struct{}{},
	}
}
