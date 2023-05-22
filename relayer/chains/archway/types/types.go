package types

import (
	"encoding/hex"
	"encoding/json"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
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

func MsgCreateClient(clientState, consensusState CustomAny, signer HexBytes) map[string]interface{} {
	return map[string]interface{}{
		"create_client": map[string]interface{}{
			"client_state":    clientState,
			"consensus_state": consensusState,
			"signer":          signer,
		},
	}
}

type UpdateClient struct {
	UpdateClient clienttypes.MsgUpdateClient `json:"update_client"`
}

func MsgUpdateClient(clientId string, clientMsg CustomAny, signer HexBytes) map[string]interface{} {

	return map[string]interface{}{
		"update_client": map[string]interface{}{
			"client_id":      clientId,
			"client_message": clientMsg,
			"signer":         signer,
		},
	}
}

type ConnectionOpenInit struct {
	Msg conntypes.MsgConnectionOpenInit `json:"connection_open_init"`
}
type ConnectionOpenTry struct {
	Msg conntypes.MsgConnectionOpenTry `json:"connection_open_try"`
}
type ConnectionOpenAck struct {
	Msg conntypes.MsgConnectionOpenAck `json:"connection_open_ack"`
}
type ConnectionOpenConfirm struct {
	Msg conntypes.MsgConnectionOpenConfirm `json:"connection_open_confirm"`
}

type ChannelOpenInit struct {
	Msg chantypes.MsgChannelOpenInit `json:"channel_open_init"`
}

type ChannelOpenTry struct {
	Msg chantypes.MsgChannelOpenTry `json:"ChannelOpenTry"`
}

type ChannelOpenAck struct {
	Msg chantypes.MsgChannelOpenAck `json:"ChannelOpenAck"`
}
type ChannelOpenConfirm struct {
	Msg chantypes.MsgChannelOpenConfirm `json:"ChannelOpenConfirm"`
}
type ChannelCloseInit struct {
	Msg chantypes.MsgChannelCloseInit `json:"ChannelCloseInit"`
}

type ChannelCloseConfirm struct {
	Msg chantypes.MsgChannelCloseConfirm `json:"ChannelCloseConfirm"`
}

type ReceivePacket struct {
	Msg chantypes.MsgRecvPacket `json:"ReceivePacket"`
}

type AcknowledgementPacket struct {
	Msg chantypes.MsgAcknowledgement `json:"AcknowledgementPacket"`
}

type TimeoutPacket struct {
	Msg chantypes.MsgTimeout `json:"TimeoutPacket"`
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

const (
	MethodGetNextClientSequence     = "get_next_client_sequence"
	MethodGetNextChannelSequence    = "get_next_channel_sequence"
	MethodGetNextConnectionSequence = "get_next_connection_sequence"
)

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

type CustomAny struct {
	TypeUrl string   `json:"type_url"`
	Value   HexBytes `json:"value"`
}

func NewCustomAny(a *codectypes.Any) CustomAny {
	return CustomAny{
		TypeUrl: a.TypeUrl,
		Value:   NewHexBytes(a.Value),
	}
}
