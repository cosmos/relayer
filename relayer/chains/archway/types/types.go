package types

import (
	"encoding/hex"
	"encoding/json"

	types "github.com/cosmos/cosmos-sdk/codec/types"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"

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
type CreateClient struct {
	CreateClient clienttypes.MsgCreateClient `json:"CreateClient"`
}

func MsgCreateClient(c1, c2 *types.Any, signer string) *CreateClient {
	return &CreateClient{
		CreateClient: clienttypes.MsgCreateClient{
			ClientState:    c1,
			ConsensusState: c2,
			Signer:         signer,
		},
	}
}

type UpdateClient struct {
	UpdateClient clienttypes.MsgUpdateClient `json:"UpdateClient"`
}

func MsgUpdateClient(clientId string, clientMsg *types.Any, signer string) *UpdateClient {
	return &UpdateClient{
		UpdateClient: clienttypes.MsgUpdateClient{
			ClientId:      clientId,
			ClientMessage: clientMsg,
			Signer:        signer,
		},
	}
}

type ConnectionOpenInit struct {
	Msg conntypes.MsgConnectionOpenInit `json:"ConnectionOpenInit"`
}
type ConnectionOpenTry struct {
	Msg conntypes.MsgConnectionOpenTry `json:"ConnectionOpenTry"`
}
type ConnectionOpenAck struct {
	Msg conntypes.MsgConnectionOpenAck `json:"ConnectionOpenAck"`
}
type ConnectionOpenConfirm struct {
	Msg conntypes.MsgConnectionOpenConfirm `json:"ConnectionOpenConfirm"`
}

type ChannelOpenInit struct {
	Msg chantypes.MsgChannelOpenInit `json:"ChannelOpenInit"`
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
	} `json:"GetClientState"`
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
	} `json:"GetConsensusState"`
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
	} `json:"GetConnection"`
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
	} `json:"GetChannel"`
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
	} `json:"GetPacketCommitment"`
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
	} `json:"GetPacketAcknowledgementCommitment"`
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
	} `json:"GetNextSequenceSend"`
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
	} `json:"GetNextSequenceReceive"`
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
	} `json:"GetNextSequenceAcknowledgement"`
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
	} `json:"GetPacketReceipt"`
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
	MethodGetNextClientSequence     = "getNextClientSequence"
	MethodGetNextChannelSequence    = "getNextChannelSequence"
	MethodGetNextConnectionSequence = "getNextConnectionSequence"
)

type GetNextClientSequence struct {
	Sequence struct{} `json:"GetNextClientSequence"`
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
	Sequence struct{} `json:"GetNextConnectionSequence"`
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
	Sequence struct{} `json:"GetNextChannelSequence"`
}

func (x *GetNextChannelSequence) Bytes() ([]byte, error) {
	return json.Marshal(x)
}

func NewNextChannelSequence() *GetNextChannelSequence {
	return &GetNextChannelSequence{
		Sequence: struct{}{},
	}
}
